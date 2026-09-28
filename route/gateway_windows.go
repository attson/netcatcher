//go:build windows

package route

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

func DiscoverGateway(iface *net.Interface, ipv6 bool) (string, error) {
	property := "IPv4DefaultGateway"
	if ipv6 {
		property = "IPv6DefaultGateway"
	}
	script := fmt.Sprintf(
		`$g = (Get-NetIPConfiguration -InterfaceIndex %d -ErrorAction Stop).%s.NextHop; if ($g) { $g }`,
		iface.Index, property,
	)
	out, commandErr := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if commandErr == nil {
		if gateway := parseWindowsGateway(out, ipv6); gateway != "" {
			return gateway, nil
		}
		commandErr = fmt.Errorf("PowerShell returned no %s gateway", familyName(ipv6))
	}

	if iface.Flags&net.FlagPointToPoint != 0 {
		if gateway := firstInterfaceAddress(iface, ipv6); gateway != "" {
			return gateway, nil
		}
	}
	return "", fmt.Errorf("no %s gateway found for %s: %w", familyName(ipv6), iface.Name, commandErr)
}

func parseWindowsGateway(output []byte, ipv6 bool) string {
	for _, field := range strings.Fields(string(output)) {
		if gatewayMatchesFamily(field, ipv6) {
			return field
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
		if err == nil && (ip.To4() == nil) == ipv6 {
			return ip.String()
		}
	}
	return ""
}

func familyName(ipv6 bool) string {
	if ipv6 {
		return "IPv6"
	}
	return "IPv4"
}
