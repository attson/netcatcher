//go:build windows

package netcatcher

// ForwarderPort is the UDP port the local DNS forwarder listens on. On
// Windows the system-wide DNS redirect mechanism is NRPT (Name Resolution
// Policy Table), which only accepts a DNS server IP — no port. The only way
// the forwarder can receive traffic from it is to bind the standard DNS
// port. NetCatcher already requires Administrator on Windows for route
// management, so binding 53 is not an additional ask.
const ForwarderPort = 53
