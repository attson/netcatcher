package netcatcher

import (
	"context"
	"fmt"
	"log"
	"net"
	"netcatcher/config"
	"netcatcher/route"
	"time"
)

type InterfaceStatus struct {
	InterfaceName string        `json:"interfaceName"`
	Connected     bool          `json:"connected"`
	Gateway       string        `json:"gateway"`
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
	addr   net.Addr
}

type NetCatcher struct {
	config   config.Interface
	onChange chan changeEvent
	current  status
	routes   []routeEntry
	onStatus StatusCallback
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

	var gateway string
	for _, r := range n.routes {
		if r.gateway != "" {
			gateway = r.gateway
			break
		}
	}

	var newIPs []net.IP
	if isConnected && gateway != "" {
		iface, ifaceErr := net.InterfaceByName(n.config.Name)
		if ifaceErr == nil && iface != nil {
			ips, err := lookupIPViaInterface(iface, gateway, n.config.DNS, forAddr)
			if err != nil || len(ips) == 0 {
				log.Printf("%s: [warn] refresh %s via %s fail %v; falling back to system resolver", n.config.Name, forAddr, iface.Name, err)
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

	entryGateway := gateway
	if !isConnected {
		entryGateway = ""
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
		entry := routeEntry{forAddr: forAddr, ip: ipStr, gateway: entryGateway}
		newByIP[ipStr] = entry
		newEntries = append(newEntries, entry)
	}

	if isConnected && gateway != "" {
		var toDelete, toAdd []route.RouteSpec
		for ip, r := range oldByIP {
			if _, ok := newByIP[ip]; !ok {
				toDelete = append(toDelete, route.RouteSpec{Ip: r.ip, Gateway: r.gateway, Mask: r.mask})
			}
		}
		for ip, r := range newByIP {
			if _, ok := oldByIP[ip]; !ok {
				toAdd = append(toAdd, route.RouteSpec{Ip: r.ip, Gateway: r.gateway, Mask: r.mask})
			}
		}

		if len(toDelete) > 0 {
			if err := route.DeleteRoutes(toDelete); err != nil {
				log.Printf("%s: [warn] delete stale routes for %s: %v", n.config.Name, forAddr, err)
			}
		}
		if len(toAdd) > 0 {
			if err := route.AddRoutes(toAdd); err != nil {
				log.Printf("%s: [warn] add refreshed routes for %s: %v", n.config.Name, forAddr, err)
			}
		}
	}

	n.routes = append(kept, newEntries...)
	n.emitStatus()
	return nil
}

func (n *NetCatcher) GetStatus() InterfaceStatus {
	s := InterfaceStatus{
		InterfaceName: n.config.Name,
		Connected:     n.current == connected,
		Routes:        make([]RouteStatus, len(n.routes)),
	}
	for i, r := range n.routes {
		s.Routes[i] = RouteStatus{
			For:     r.forAddr,
			Ip:      r.ip,
			Gateway: r.gateway,
			Active:  n.current == connected,
		}
	}
	if len(n.routes) > 0 {
		s.Gateway = n.routes[0].gateway
	}
	return s
}

func (n *NetCatcher) emitStatus() {
	if n.onStatus != nil {
		n.onStatus(n.GetStatus())
	}
}

func (n *NetCatcher) resolveRoutes(gateway string) {
	n.routes = []routeEntry{}
	iface, ifaceErr := net.InterfaceByName(n.config.Name)
	if ifaceErr != nil {
		log.Printf("%s: [warn] lookup interface for DNS binding: %v", n.config.Name, ifaceErr)
	}
	for _, addr := range n.config.Routes {
		_, ipnet, err := net.ParseCIDR(addr)
		if err == nil {
			n.routes = append(n.routes, routeEntry{
				forAddr: addr, ip: addr, mask: ipnet.Mask, gateway: gateway,
			})
			continue
		}
		if net.ParseIP(addr) != nil {
			n.routes = append(n.routes, routeEntry{
				forAddr: addr, ip: addr, mask: nil, gateway: gateway,
			})
			continue
		}
		var ips []net.IP
		if iface != nil {
			ips, err = lookupIPViaInterface(iface, gateway, n.config.DNS, addr)
			if err != nil || len(ips) == 0 {
				log.Printf("%s: [warn] lookup %s via %s fail %v; falling back to system resolver\n", n.config.Name, addr, iface.Name, err)
				ips = nil
			}
		}
		if len(ips) == 0 {
			ips, err = net.LookupIP(addr)
			if err != nil {
				log.Printf("%s: [warn] lookup %s fail %v\n", n.config.Name, addr, err)
			}
		}
		for _, ip := range ips {
			n.routes = append(n.routes, routeEntry{
				forAddr: addr, ip: ip.String(), gateway: gateway,
			})
		}
	}
}

func (n *NetCatcher) addRoutesTo(addr net.Addr) {
	ip, _, err := net.ParseCIDR(addr.String())
	if err != nil {
		log.Printf("%s: [error] parse %s CIDR fail %v", n.config.Name, addr.String(), err)
		return
	}
	n.resolveRoutes(ip.String())
	specs := make([]route.RouteSpec, len(n.routes))
	for i, r := range n.routes {
		specs[i] = route.RouteSpec{Ip: r.ip, Gateway: r.gateway, Mask: r.mask}
		log.Printf("%s: [debug] add route %s", n.config.Name, r)
	}
	if err := route.AddRoutes(specs); err != nil {
		log.Printf("%s: [warn] add routes failed: %v", n.config.Name, err)
	}
}

func (n *NetCatcher) clearRoutes() {
	if len(n.routes) == 0 || n.current != connected {
		return
	}
	specs := make([]route.RouteSpec, len(n.routes))
	for i, r := range n.routes {
		specs[i] = route.RouteSpec{Ip: r.ip, Gateway: r.gateway, Mask: r.mask}
		log.Printf("%s: [debug] delete route %s", n.config.Name, r)
	}
	if err := route.DeleteRoutes(specs); err != nil {
		log.Printf("%s: [warn] delete routes failed: %v", n.config.Name, err)
	}
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
			log.Printf("%s: [info] interface status changed to %v\n", n.config.Name, event.status == connected)
			n.current = event.status
			if event.status == connected {
				n.addRoutesTo(event.addr)
			}
			n.emitStatus()
		}
	}
}

func (n *NetCatcher) poll() *changeEvent {
	i, err := net.InterfaceByName(n.config.Name)
	if err != nil {
		if opErr, ok := err.(*net.OpError); ok {
			if opErr.Unwrap().Error() == "no such network interface" {
				return &changeEvent{status: disconnected}
			}
		}
		log.Printf("%s: [warn] get interface fail %v\n", n.config.Name, err)
		return nil
	}
	addrs, err := i.Addrs()
	if err != nil || len(addrs) == 0 {
		log.Printf("%s: [warn] get interface addr fail %v\n", n.config.Name, err)
		return nil
	}
	return &changeEvent{status: connected, addr: addrs[0]}
}

func (n *NetCatcher) Stop() {
	n.clearRoutes()
}
