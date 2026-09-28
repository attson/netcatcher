package route

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

type RouteSpec struct {
	IP             string
	Gateway        string
	Mask           net.IPMask
	InterfaceName  string
	InterfaceIndex int
}

func AddRoutes(routes []RouteSpec) error {
	var errs []error
	for _, route := range routes {
		if err := AddRoute(route); err != nil {
			errs = append(errs, fmt.Errorf("add %s: %w", route.IP, err))
		}
	}
	return errors.Join(errs...)
}

func DeleteRoutes(routes []RouteSpec) error {
	var errs []error
	for _, route := range routes {
		if err := DeleteRoute(route); err != nil {
			errs = append(errs, fmt.Errorf("delete %s: %w", route.IP, err))
		}
	}
	return errors.Join(errs...)
}

func validateRouteSpec(spec RouteSpec) (net.IP, bool, error) {
	ip := net.ParseIP(spec.IP)
	if ip == nil {
		return nil, false, fmt.Errorf("invalid destination IP %q", spec.IP)
	}
	gatewayIP := net.ParseIP(strings.SplitN(spec.Gateway, "%", 2)[0])
	if gatewayIP == nil {
		return nil, false, fmt.Errorf("invalid gateway IP %q", spec.Gateway)
	}
	ipv6 := ip.To4() == nil
	if ipv6 != (gatewayIP.To4() == nil) {
		return nil, false, fmt.Errorf("destination %s and gateway %s use different address families", spec.IP, spec.Gateway)
	}
	if spec.Mask != nil {
		_, bits := spec.Mask.Size()
		if (!ipv6 && bits != 32) || (ipv6 && bits != 128) {
			return nil, false, fmt.Errorf("invalid mask for destination %s", spec.IP)
		}
	}
	return ip, ipv6, nil
}
