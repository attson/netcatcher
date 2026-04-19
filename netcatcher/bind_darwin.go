//go:build darwin

package netcatcher

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func bindControl(ifIndex int) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var sockErr error
		err := c.Control(func(fd uintptr) {
			switch network {
			case "udp6", "tcp6":
				sockErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF, ifIndex)
			default:
				sockErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, ifIndex)
			}
		})
		if err != nil {
			return err
		}
		return sockErr
	}
}
