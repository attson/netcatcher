package niar

import (
	"fmt"
	"log"
	"net"
	"niar/config"
	"niar/route"
	"time"
)

type Status int

const (
	_ Status = iota
	Connected
	DisConnected
)

func (s Status) String() string {
	switch s {
	case Connected:
		return "connected"
	case DisConnected:
		return "disconnected"
	}

	return "unknown"
}

type Route struct {
	For     string
	Ip      string
	Gateway string
	Mask    bool
}

func (r Route) String() string {
	return fmt.Sprintf("%s -> %s @ %s", r.For, r.Ip, r.Gateway)
}

type ChangeEvent struct {
	Status Status
	Addr   net.Addr
}

type Niar struct {
	config config.Interface

	onChange chan ChangeEvent
	status   Status

	routes []Route
}

func NewNiac(config config.Interface) *Niar {
	return &Niar{config: config, onChange: make(chan ChangeEvent)}
}

func (n *Niar) resolveRoutes(gateway string) {
	n.routes = []Route{}
	for _, addr := range n.config.Routes {
		_, _, err := net.ParseCIDR(addr)
		if err == nil {
			n.routes = append(n.routes, Route{
				For:     addr,
				Ip:      addr,
				Mask:    true,
				Gateway: gateway,
			})
			continue
		}

		if net.ParseIP(addr) != nil {
			n.routes = append(n.routes, Route{
				For:     addr,
				Ip:      addr,
				Mask:    false,
				Gateway: gateway,
			})
			continue
		}

		ips, err := net.LookupIP(addr)
		if err != nil {
			log.Printf("warn lookup %s fail %v\n", addr, err)
		}
		for _, ip := range ips {
			n.routes = append(n.routes, Route{
				For:     addr,
				Ip:      ip.String(),
				Gateway: gateway,
			})
		}

		continue
	}
}

func (n *Niar) addRoutersTo(addr net.Addr) {
	ip, _, err := net.ParseCIDR(addr.String())
	if err != nil {
		log.Printf("error parse %s CIDR fail %v", addr.String(), err)
		return
	}

	n.resolveRoutes(ip.String())
	for _, r := range n.routes {
		err := route.AddRoute(r.Ip, r.Gateway, r.Mask)
		if err != nil {
			log.Printf("warn: add route fail %s %v", r, err)
		} else {
			log.Printf("debug: add route %s", r)
		}

	}
}

func (n *Niar) clearRouters() {
	for _, r := range n.routes {
		err := route.DeleteRoute(r.Ip, r.Gateway, r.Mask)
		if err != nil {
			log.Printf("warn: delete route fail %s %v", r, err)
		} else {
			log.Printf("debug: delete route %s", r)
		}
	}
}

func (n *Niar) Watch() {
	go func() {
		for {
			i, err := net.InterfaceByName(n.config.Name)
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok {
					if opErr.Unwrap().Error() == "no such network interface" {
						n.onChange <- ChangeEvent{
							Status: DisConnected,
						}
						continue
					}
				}

				log.Printf("warn get interface fail %v\n", err)
			} else {
				addrs, err := i.Addrs()
				if err != nil || len(addrs) == 0 {
					log.Printf("warn get interface addr fail %v\n", err)
				} else {
					n.onChange <- ChangeEvent{
						Status: Connected,
						Addr:   addrs[0],
					}
				}
			}

			time.Sleep(time.Second)
		}
	}()

	for {
		select {
		case event := <-n.onChange:
			if n.status == event.Status {
				break
			}

			log.Printf("interface %s status is %s\n", n.config.Name, event.Status.String())

			n.status = event.Status
			if event.Status == Connected {
				n.addRoutersTo(event.Addr)
			} else {
				// when interface disconnect. the system will clean up routes
				// n.clearRouters()
			}
		}
	}
}

func (n *Niar) Stop() {
	n.clearRouters()
}
