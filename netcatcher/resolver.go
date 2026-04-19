package netcatcher

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// lookupIPViaInterface resolves host using DNS queries whose UDP/TCP sockets
// are bound to the given interface. This bypasses system-wide TUN proxies
// that would otherwise hijack DNS and return fake IPs.
//
// dnsServers is the priority list; if empty or all fail, the gateway itself
// is tried as a last resort (many VPN peers answer DNS on :53). Each entry
// may be either "host" or "host:port" (default port 53).
func lookupIPViaInterface(iface *net.Interface, gateway string, dnsServers []string, host string) ([]net.IP, error) {
	servers := make([]string, 0, len(dnsServers)+1)
	for _, s := range dnsServers {
		servers = append(servers, withDefaultPort(s, "53"))
	}
	if gateway != "" {
		servers = append(servers, net.JoinHostPort(gateway, "53"))
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("no DNS servers available for interface %s", iface.Name)
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 3 * time.Second,
				Control: bindControl(iface.Index),
			}
			var lastErr error
			for _, srv := range servers {
				conn, err := d.DialContext(ctx, network, srv)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return resolver.LookupIP(ctx, "ip", host)
}

func withDefaultPort(addr, port string) string {
	if strings.Contains(addr, ":") && !strings.HasPrefix(addr, "[") {
		// Bare IPv6 without brackets — wrap it.
		if strings.Count(addr, ":") > 1 {
			return "[" + addr + "]:" + port
		}
		// host:port already.
		return addr
	}
	return net.JoinHostPort(addr, port)
}
