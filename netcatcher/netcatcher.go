package netcatcher

import (
	"context"
	"fmt"
	"net"
	"netcatcher/config"
	"netcatcher/llog"
	"netcatcher/route"
	"strconv"
	"strings"
	"time"
)

type InterfaceStatus struct {
	InterfaceName string        `json:"interfaceName"`
	Connected     bool          `json:"connected"`
	Gateway       string        `json:"gateway"`
	IPv4Gateway   string        `json:"ipv4Gateway"`
	IPv6Gateway   string        `json:"ipv6Gateway"`
	Routes        []RouteStatus `json:"routes"`
}

type RouteStatus struct {
	For     string `json:"for"`
	Ip      string `json:"ip"`
	Gateway string `json:"gateway"`
	Active  bool   `json:"active"`
}

type StatusCallback func(status InterfaceStatus)

type status int

const (
	_ status = iota
	connected
	disconnected
)

type routeEntry struct {
	forAddr string
	ip      string
	gateway string
	mask    net.IPMask
}

func (r routeEntry) String() string {
	return fmt.Sprintf("%s -> %s @ %s", r.forAddr, r.ip, r.gateway)
}

type changeEvent struct {
	status status
	iface  *net.Interface
}

type NetCatcher struct {
	config         config.Interface
	onChange       chan changeEvent
	current        status
	routes         []routeEntry
	onStatus       StatusCallback
	ipv4Gateway    string
	ipv6Gateway    string
	interfaceIndex int
}

func NewNetCatcher(cfg config.Interface, onStatus StatusCallback) *NetCatcher {
	return &NetCatcher{
		config:   cfg,
		onChange: make(chan changeEvent),
		onStatus: onStatus,
	}
}

func (n *NetCatcher) Name() string {
	return n.config.Name
}

func (n *NetCatcher) RefreshRoute(forAddr string) error {
	if _, _, err := net.ParseCIDR(forAddr); err == nil {
		return nil
	}
	if net.ParseIP(forAddr) != nil {
		return nil
	}

	isConnected := n.current == connected

	var newIPs []net.IP
	if isConnected && (n.ipv4Gateway != "" || n.ipv6Gateway != "") {
		iface, ifaceErr := net.InterfaceByName(n.config.Name)
		if ifaceErr == nil && iface != nil {
			ips, err := lookupIPViaInterface(iface, n.gatewayList(), n.config.DNS, forAddr)
			if err != nil || len(ips) == 0 {
				llog.Warnf(n.tag(), "refresh %s via %s failed: %v; falling back to system resolver", forAddr, iface.Name, err)
			} else {
				newIPs = ips
			}
		}
	}
	if len(newIPs) == 0 {
		ips, err := net.LookupIP(forAddr)
		if err != nil {
			return fmt.Errorf("lookup %s: %w", forAddr, err)
		}
		newIPs = ips
	}

	oldByIP := map[string]routeEntry{}
	kept := make([]routeEntry, 0, len(n.routes))
	for _, r := range n.routes {
		if r.forAddr == forAddr {
			oldByIP[r.ip] = r
		} else {
			kept = append(kept, r)
		}
	}

	newEntries := make([]routeEntry, 0, len(newIPs))
	newByIP := map[string]routeEntry{}
	for _, ip := range newIPs {
		ipStr := ip.String()
		if _, dup := newByIP[ipStr]; dup {
			continue
		}
		gateway := n.gatewayForIP(ip)
		if isConnected && gateway == "" {
			llog.Warnf(n.tag(), "skip refreshed %s address %s: no matching gateway", forAddr, ipStr)
			continue
		}
		entry := routeEntry{forAddr: forAddr, ip: ipStr, gateway: gateway}
		newByIP[ipStr] = entry
		newEntries = append(newEntries, entry)
	}

	if isConnected {
		var toDelete, toAdd []route.RouteSpec
		for ip, r := range oldByIP {
			if _, ok := newByIP[ip]; !ok {
				toDelete = append(toDelete, n.routeSpec(r))
			}
		}
		for ip, r := range newByIP {
			if _, ok := oldByIP[ip]; !ok {
				toAdd = append(toAdd, n.routeSpec(r))
			}
		}

		if len(toDelete) > 0 {
			if err := route.DeleteRoutes(toDelete); err != nil {
				llog.Warnf(n.tag(), "delete stale routes for %s: %v", forAddr, err)
			}
		}
		if len(toAdd) > 0 {
			if err := route.AddRoutes(toAdd); err != nil {
				llog.Warnf(n.tag(), "add refreshed routes for %s: %v", forAddr, err)
			}
		}
	}

	n.routes = append(kept, newEntries...)
	n.refreshSystemDNSCache()
	n.emitStatus()
	return nil
}

func (n *NetCatcher) GetStatus() InterfaceStatus {
	s := InterfaceStatus{
		InterfaceName: n.config.Name,
		Connected:     n.current == connected,
		IPv4Gateway:   n.ipv4Gateway,
		IPv6Gateway:   n.ipv6Gateway,
		Routes:        make([]RouteStatus, len(n.routes)),
	}
	if n.ipv4Gateway != "" {
		s.Gateway = n.ipv4Gateway
	} else {
		s.Gateway = n.ipv6Gateway
	}
	for i, r := range n.routes {
		s.Routes[i] = RouteStatus{
			For:     r.forAddr,
			Ip:      r.ip,
			Gateway: r.gateway,
			Active:  n.current == connected,
		}
	}
	return s
}

func (n *NetCatcher) emitStatus() {
	if n.onStatus != nil {
		n.onStatus(n.GetStatus())
	}
}

func (n *NetCatcher) tag() string { return "netcatcher/" + n.config.Name }

func (n *NetCatcher) hasDomainRoutes() bool {
	for _, addr := range n.config.Routes {
		if _, _, err := net.ParseCIDR(addr); err == nil {
			continue
		}
		if net.ParseIP(addr) == nil {
			return true
		}
	}
	return false
}

func (n *NetCatcher) refreshSystemDNSCache() {
	if err := route.FlushDNSCache(); err != nil {
		llog.Warnf(n.tag(), "refresh system DNS cache failed: %v", err)
	} else {
		llog.Infof(n.tag(), "system DNS cache refreshed")
	}
}

func (n *NetCatcher) resolveRoutes(iface *net.Interface) {
	n.routes = []routeEntry{}
	for _, addr := range n.config.Routes {
		ip, ipnet, err := net.ParseCIDR(addr)
		if err == nil {
			gateway := n.gatewayForIP(ip)
			if gateway == "" {
				llog.Warnf(n.tag(), "skip route %s: no matching gateway", addr)
				continue
			}
			n.routes = append(n.routes, routeEntry{
				forAddr: addr, ip: ipnet.IP.String(), mask: ipnet.Mask, gateway: gateway,
			})
			continue
		}
		if ip := net.ParseIP(addr); ip != nil {
			gateway := n.gatewayForIP(ip)
			if gateway == "" {
				llog.Warnf(n.tag(), "skip route %s: no matching gateway", addr)
				continue
			}
			n.routes = append(n.routes, routeEntry{
				forAddr: addr, ip: ip.String(), mask: nil, gateway: gateway,
			})
			continue
		}
		var ips []net.IP
		if iface != nil {
			ips, err = lookupIPViaInterface(iface, n.gatewayList(), n.config.DNS, addr)
			if err != nil || len(ips) == 0 {
				llog.Warnf(n.tag(), "lookup %s via %s failed: %v; falling back to system resolver", addr, iface.Name, err)
				ips = nil
			}
		}
		if len(ips) == 0 {
			ips, err = net.LookupIP(addr)
			if err != nil {
				llog.Warnf(n.tag(), "lookup %s failed: %v", addr, err)
			}
		}
		for _, ip := range ips {
			gateway := n.gatewayForIP(ip)
			if gateway == "" {
				llog.Warnf(n.tag(), "skip %s address %s: no matching gateway", addr, ip)
				continue
			}
			n.routes = append(n.routes, routeEntry{
				forAddr: addr, ip: ip.String(), gateway: gateway,
			})
		}
	}
}

func (n *NetCatcher) addRoutesForInterface(iface *net.Interface) {
	n.interfaceIndex = iface.Index
	n.ipv4Gateway = n.resolveGateway(iface, false)
	n.ipv6Gateway = n.resolveGateway(iface, true)
	if n.ipv4Gateway == "" && n.ipv6Gateway == "" {
		llog.Errorf(n.tag(), "no IPv4 or IPv6 gateway available")
		n.routes = nil
		return
	}
	n.resolveRoutes(iface)
	specs := make([]route.RouteSpec, len(n.routes))
	for i, r := range n.routes {
		specs[i] = n.routeSpec(r)
		llog.Debugf(n.tag(), "add route %s", r)
	}
	if err := route.AddRoutes(specs); err != nil {
		llog.Warnf(n.tag(), "add routes failed: %v", err)
	}
}

func (n *NetCatcher) clearRoutes() {
	if len(n.routes) == 0 || n.current != connected {
		return
	}
	specs := make([]route.RouteSpec, len(n.routes))
	for i, r := range n.routes {
		specs[i] = n.routeSpec(r)
		llog.Debugf(n.tag(), "delete route %s", r)
	}
	if err := route.DeleteRoutes(specs); err != nil {
		llog.Warnf(n.tag(), "delete routes failed: %v", err)
	}
	n.routes = nil
}

func (n *NetCatcher) Watch(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			n.clearRoutes()
			return
		case <-ticker.C:
			event := n.poll()
			if event == nil || n.current == event.status {
				continue
			}
			if event.status == connected {
				llog.Infof(n.tag(), "interface connected")
			} else {
				llog.Infof(n.tag(), "interface disconnected")
				n.clearRoutes()
			}
			n.current = event.status
			if event.status == connected {
				n.addRoutesForInterface(event.iface)
				if n.hasDomainRoutes() {
					n.refreshSystemDNSCache()
				}
			}
			if event.status == disconnected {
				n.ipv4Gateway = ""
				n.ipv6Gateway = ""
				n.interfaceIndex = 0
			}
			n.emitStatus()
		}
	}
}

func (n *NetCatcher) poll() *changeEvent {
	i, err := net.InterfaceByName(n.config.Name)
	if err != nil {
		if opErr, ok := err.(*net.OpError); ok {
			if cause := opErr.Unwrap(); cause != nil && cause.Error() == "no such network interface" {
				return &changeEvent{status: disconnected}
			}
		}
		llog.Warnf(n.tag(), "get interface failed: %v", err)
		return nil
	}
	addrs, err := i.Addrs()
	if err != nil {
		llog.Warnf(n.tag(), "get interface addr failed: %v", err)
		return nil
	}
	if len(addrs) == 0 {
		return &changeEvent{status: disconnected}
	}
	return &changeEvent{status: connected, iface: i}
}

func (n *NetCatcher) Stop() {
	n.clearRoutes()
}

func (n *NetCatcher) resolveGateway(iface *net.Interface, ipv6 bool) string {
	configured := strings.TrimSpace(n.config.IPv4Gateway)
	if ipv6 {
		configured = strings.TrimSpace(n.config.IPv6Gateway)
	}
	if configured != "" {
		parts := strings.SplitN(configured, "%", 2)
		address := parts[0]
		ip := net.ParseIP(address)
		if ip == nil || (ip.To4() == nil) != ipv6 {
			llog.Errorf(n.tag(), "configured %s gateway is invalid: %s", familyName(ipv6), configured)
			return ""
		}
		if len(parts) == 2 && parts[1] != iface.Name && parts[1] != strconv.Itoa(iface.Index) {
			llog.Errorf(n.tag(), "configured IPv6 gateway scope %q does not match interface %s", parts[1], iface.Name)
			return ""
		}
		llog.Infof(n.tag(), "using configured %s gateway %s", familyName(ipv6), configured)
		return configured
	}
	gateway, err := route.DiscoverGateway(iface, ipv6)
	if err != nil {
		llog.Debugf(n.tag(), "auto-detect %s gateway: %v", familyName(ipv6), err)
		return ""
	}
	llog.Infof(n.tag(), "auto-detected %s gateway %s", familyName(ipv6), gateway)
	return gateway
}

func (n *NetCatcher) gatewayForIP(ip net.IP) string {
	if ip.To4() != nil {
		return n.ipv4Gateway
	}
	return n.ipv6Gateway
}

func (n *NetCatcher) gatewayList() []string {
	gateways := make([]string, 0, 2)
	if n.ipv4Gateway != "" {
		gateways = append(gateways, n.ipv4Gateway)
	}
	if n.ipv6Gateway != "" {
		gateways = append(gateways, n.ipv6Gateway)
	}
	return gateways
}

func (n *NetCatcher) routeSpec(entry routeEntry) route.RouteSpec {
	return route.RouteSpec{
		IP:             entry.ip,
		Gateway:        entry.gateway,
		Mask:           entry.mask,
		InterfaceName:  n.config.Name,
		InterfaceIndex: n.interfaceIndex,
	}
}

func familyName(ipv6 bool) string {
	if ipv6 {
		return "IPv6"
	}
	return "IPv4"
}
