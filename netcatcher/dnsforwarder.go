package netcatcher

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"netcatcher/llog"
	"strings"
	"sync"
	"time"
)

// ForwarderPort is the UDP port the local DNS forwarder listens on (loopback).
// Chosen to be non-privileged so the NetCatcher process does not need root to bind.
// macOS `/etc/resolver/<domain>` files point here via `nameserver 127.0.0.1` + `port`.
const ForwarderPort = 15353

type forwarderRoute struct {
	iface      *net.Interface
	ifaceName  string
	dnsServers []string // host:port entries, tried in order
}

type DNSForwarder struct {
	mu     sync.RWMutex
	routes map[string]forwarderRoute // lowercase domain suffix → route
	conn   *net.UDPConn
	done   chan struct{}
}

func NewDNSForwarder() *DNSForwarder {
	return &DNSForwarder{routes: make(map[string]forwarderRoute)}
}

// Start begins listening on 127.0.0.1:ForwarderPort. Safe to call while
// routes are being added; calls are idempotent.
func (f *DNSForwarder) Start() error {
	f.mu.Lock()
	if f.conn != nil {
		f.mu.Unlock()
		return nil
	}
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: ForwarderPort}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		f.mu.Unlock()
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	f.conn = conn
	f.done = make(chan struct{})
	f.mu.Unlock()

	go f.loop()
	llog.Infof("dns", "forwarder listening on %s", addr)
	return nil
}

// Stop closes the listener and waits for the read loop to exit.
func (f *DNSForwarder) Stop() {
	f.mu.Lock()
	conn := f.conn
	done := f.done
	f.conn = nil
	f.done = nil
	f.mu.Unlock()
	if conn == nil {
		return
	}
	conn.Close()
	if done != nil {
		<-done
	}
	llog.Infof("dns", "forwarder stopped")
}

// SetRoute registers a domain → interface binding. Incoming queries whose
// QNAME equals the domain or a subdomain are forwarded with their UDP socket
// bound to the interface.
func (f *DNSForwarder) SetRoute(domain string, iface *net.Interface, dnsServers []string) {
	if domain == "" || iface == nil {
		return
	}
	servers := normalizeDNSServers(dnsServers)
	f.mu.Lock()
	f.routes[normalizeDomain(domain)] = forwarderRoute{
		iface: iface, ifaceName: iface.Name, dnsServers: servers,
	}
	f.mu.Unlock()
}

// ClearInterface removes every route whose interface name matches.
func (f *DNSForwarder) ClearInterface(ifaceName string) {
	f.mu.Lock()
	for d, r := range f.routes {
		if r.ifaceName == ifaceName {
			delete(f.routes, d)
		}
	}
	f.mu.Unlock()
}

// Domains returns the current set of registered domains (useful for the
// privileged /etc/resolver helper).
func (f *DNSForwarder) Domains() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]string, 0, len(f.routes))
	for d := range f.routes {
		out = append(out, d)
	}
	return out
}

func (f *DNSForwarder) loop() {
	defer close(f.done)
	buf := make([]byte, 1500)
	for {
		n, client, err := f.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		go f.handle(pkt, client)
	}
}

func (f *DNSForwarder) handle(pkt []byte, client *net.UDPAddr) {
	qname, err := extractQName(pkt)
	if err != nil {
		f.reply(pkt, client, rcodeFormErr)
		return
	}
	route, ok := f.matchRoute(qname)
	if !ok {
		llog.Debugf("dns", "no route for %s, refusing", qname)
		f.reply(pkt, client, rcodeRefused)
		return
	}
	resp, err := f.forward(pkt, route)
	if err != nil {
		llog.Warnf("dns", "forward %s via %s failed: %v", qname, route.ifaceName, err)
		f.reply(pkt, client, rcodeServFail)
		return
	}
	if _, err := f.conn.WriteToUDP(resp, client); err != nil {
		llog.Warnf("dns", "reply to %s failed: %v", client, err)
	}
}

func (f *DNSForwarder) matchRoute(qname string) (forwarderRoute, bool) {
	q := normalizeDomain(qname)
	f.mu.RLock()
	defer f.mu.RUnlock()
	for {
		if r, ok := f.routes[q]; ok {
			return r, true
		}
		idx := strings.Index(q, ".")
		if idx < 0 {
			return forwarderRoute{}, false
		}
		q = q[idx+1:]
	}
}

func (f *DNSForwarder) forward(pkt []byte, r forwarderRoute) ([]byte, error) {
	if len(r.dnsServers) == 0 {
		return nil, fmt.Errorf("no DNS servers configured for %s", r.ifaceName)
	}
	dialer := net.Dialer{
		Timeout: 3 * time.Second,
		Control: bindControl(r.iface.Index),
	}
	var lastErr error
	for _, srv := range r.dnsServers {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		conn, err := dialer.DialContext(ctx, "udp", srv)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		conn.SetDeadline(time.Now().Add(3 * time.Second))
		if _, err := conn.Write(pkt); err != nil {
			conn.Close()
			lastErr = err
			continue
		}
		buf := make([]byte, 1500)
		n, err := conn.Read(buf)
		conn.Close()
		if err != nil {
			lastErr = err
			continue
		}
		return buf[:n], nil
	}
	return nil, lastErr
}

func (f *DNSForwarder) reply(pkt []byte, client *net.UDPAddr, rcode byte) {
	if f.conn == nil || len(pkt) < 12 {
		return
	}
	resp := make([]byte, len(pkt))
	copy(resp, pkt)
	resp[2] |= 0x80 // QR=1
	resp[3] = (resp[3] & 0xf0) | (rcode & 0x0f)
	f.conn.WriteToUDP(resp, client)
}

const (
	rcodeFormErr  byte = 1
	rcodeServFail byte = 2
	rcodeRefused  byte = 5
)

func extractQName(pkt []byte) (string, error) {
	if len(pkt) < 12 {
		return "", fmt.Errorf("packet too short")
	}
	qd := binary.BigEndian.Uint16(pkt[4:6])
	if qd == 0 {
		return "", fmt.Errorf("no question section")
	}
	pos := 12
	var labels []string
	for pos < len(pkt) {
		l := int(pkt[pos])
		if l == 0 {
			break
		}
		if l&0xc0 != 0 {
			return "", fmt.Errorf("unexpected compression pointer in question")
		}
		pos++
		if pos+l > len(pkt) {
			return "", fmt.Errorf("truncated label")
		}
		labels = append(labels, string(pkt[pos:pos+l]))
		pos += l
	}
	if len(labels) == 0 {
		return "", fmt.Errorf("empty qname")
	}
	return strings.Join(labels, "."), nil
}

func normalizeDomain(s string) string {
	return strings.ToLower(strings.Trim(s, "."))
}

func normalizeDNSServers(servers []string) []string {
	out := make([]string, 0, len(servers))
	for _, s := range servers {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.Contains(s, ":") {
			s = s + ":53"
		}
		out = append(out, s)
	}
	return out
}
