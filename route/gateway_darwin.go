//go:build darwin

package route

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// DiscoverGateway returns the active gateway for one address family, scoped
// to iface. Physical interfaces use the routing table; point-to-point
// interfaces fall back to their peer address when they have no default route.
func DiscoverGateway(iface *net.Interface, ipv6 bool) (string, error) {
	family := "-inet"
	if ipv6 {
		family = "-inet6"
	}
	out, routeErr := exec.Command("/sbin/route", "-n", "get", family, "-ifscope", iface.Name, "default").CombinedOutput()
	if routeErr == nil {
		gateway, err := parseDarwinGateway(out, iface.Name, ipv6)
		if err == nil {
			return gateway, nil
		}
		routeErr = err
	}
	if !ipv6 {
		out, err := exec.Command("/usr/sbin/ipconfig", "getoption", iface.Name, "router").CombinedOutput()
		if err == nil {
			for _, field := range strings.Fields(string(out)) {
				if gatewayMatchesFamily(field, false) {
					return field, nil
				}
			}
		}
	}

	if iface.Flags&net.FlagPointToPoint != 0 {
		ifconfigOut, err := exec.Command("/sbin/ifconfig", iface.Name).CombinedOutput()
		if err == nil {
			if gateway := parseDarwinPeer(ifconfigOut, ipv6); gateway != "" {
				return gateway, nil
			}
		}
		if gateway := firstInterfaceAddress(iface, ipv6); gateway != "" {
			return gateway, nil
		}
	}

	return "", fmt.Errorf("no %s gateway found for %s: %w", familyName(ipv6), iface.Name, routeErr)
}

func parseDarwinGateway(output []byte, interfaceName string, ipv6 bool) (string, error) {
	var gateway, foundInterface string
	for _, line := range strings.Split(string(output), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "gateway":
			gateway = strings.TrimSpace(value)
		case "interface":
			foundInterface = strings.TrimSpace(value)
		}
	}
	if foundInterface != interfaceName {
		return "", fmt.Errorf("route belongs to interface %q", foundInterface)
	}
	if !gatewayMatchesFamily(gateway, ipv6) {
		return "", fmt.Errorf("route returned invalid %s gateway %q", familyName(ipv6), gateway)
	}
	if ipv6 && strings.HasPrefix(gateway, "fe80:") && !strings.Contains(gateway, "%") {
		gateway += "%" + interfaceName
	}
	return gateway, nil
}

func parseDarwinPeer(output []byte, ipv6 bool) string {
	want := "inet"
	if ipv6 {
		want = "inet6"
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] != want {
			continue
		}
		for i, field := range fields {
			if field == "-->" && i+1 < len(fields) && gatewayMatchesFamily(fields[i+1], ipv6) {
				return fields[i+1]
			}
		}
	}
	return ""
}

func gatewayMatchesFamily(gateway string, ipv6 bool) bool {
	address := strings.SplitN(gateway, "%", 2)[0]
	ip := net.ParseIP(address)
	return ip != nil && (ip.To4() == nil) == ipv6
}

func firstInterfaceAddress(iface *net.Interface, ipv6 bool) string {
	addresses, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, address := range addresses {
		ip, _, err := net.ParseCIDR(address.String())
		if err != nil || (ip.To4() == nil) != ipv6 {
			continue
		}
		value := ip.String()
		if ipv6 && ip.IsLinkLocalUnicast() {
			value += "%" + iface.Name
		}
		return value
	}
	return ""
}

func familyName(ipv6 bool) string {
	if ipv6 {
		return "IPv6"
	}
	return "IPv4"
}
