//go:build windows

package netcatcher

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// Winsock option numbers from ws2ipdef.h. Not exposed by x/sys/windows.
const (
	ipUnicastIf   = 31
	ipv6UnicastIf = 31
)

func bindControl(ifIndex int) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var sockErr error
		err := c.Control(func(fd uintptr) {
			handle := windows.Handle(fd)
			switch network {
			case "udp6", "tcp6":
				sockErr = windows.SetsockoptInt(handle, windows.IPPROTO_IPV6, ipv6UnicastIf, ifIndex)
			default:
				// IP_UNICAST_IF expects the interface index in network byte order for IPv4.
				nbo := int(((uint32(ifIndex) & 0x000000FF) << 24) |
					((uint32(ifIndex) & 0x0000FF00) << 8) |
					((uint32(ifIndex) & 0x00FF0000) >> 8) |
					((uint32(ifIndex) & 0xFF000000) >> 24))
				sockErr = windows.SetsockoptInt(handle, windows.IPPROTO_IP, ipUnicastIf, nbo)
			}
		})
		if err != nil {
			return err
		}
		return sockErr
	}
}
