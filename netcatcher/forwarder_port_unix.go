//go:build !windows

package netcatcher

// ForwarderPort is the UDP port the local DNS forwarder listens on. On Unix
// platforms macOS `/etc/resolver/<domain>` files point here via
// `nameserver 127.0.0.1` + `port 15353`, so we can use a non-privileged port
// and avoid running the NetCatcher process as root.
const ForwarderPort = 15353
