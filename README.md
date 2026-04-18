# NetCatcher

[中文文档](README.zh-CN.md)

A desktop application that monitors network interfaces and automatically adds static routes when they connect. Useful when running multiple network interfaces or VPN connections and you need specific traffic to flow through a particular interface.

## Features

- Dashboard showing active interfaces and their route status
- Route config editor — manage interfaces and routes through the GUI
- Real-time log viewer — see route add/remove events as they happen
- Route connectivity testing (ping)
- System notifications on interface connect/disconnect
- Multi-language support (English / Chinese)
- Settings panel for app preferences (auto-start, notifications, language)
- System tray integration — the app hides to the tray when the window is closed; use the tray menu to quit

## Prerequisites

- Go 1.25+
- Node.js 20+
- Wails v3 CLI

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

## Building from Source

```bash
# Clone the repository
git clone https://github.com/attson/netcatcher.git
cd netcatcher

# Build frontend
cd frontend && npm ci && npx vite build && cd ..

# Build Go binary
go build -o build/bin/netcatcher-app .

# Or use the Wails CLI (handles both steps)
wails3 build
```

## Running

The app requires administrator/root privileges to modify the routing table.

```bash
sudo ./build/bin/netcatcher-app
```

On Windows, run the executable as Administrator.

## Configuration

Config is stored at a platform-specific path and managed through the GUI settings:

- macOS: `~/Library/Application Support/NetCatcher/config.json`
- Windows: `%APPDATA%\NetCatcher\config.json`

The config format is JSON. Entries support hostnames (resolved via DNS at connect time), individual IP addresses, and CIDR ranges:

```json
{
  "interfaces": [
    {
      "name": "ppp0",
      "routes": [
        "github.com",
        "192.168.188.11",
        "192.168.188.0/24"
      ]
    }
  ]
}
```

The `name` field must match the OS network interface name exactly (e.g. the VPN adapter name).

## Platform Notes

### macOS

The app requires root to call the `route` command. Launch with `sudo`, or grant the binary the appropriate entitlements for a production build.

### Windows

By default, a VPN connection routes all traffic through the tunnel. To use per-route configuration:

1. Open the VPN adapter properties (Network Settings -> VPN connection -> right-click -> Properties -> Networking).
2. Select Internet Protocol Version 4 (TCP/IPv4) -> Properties -> Advanced.
3. Uncheck "Use default gateway on remote network". Repeat for IPv6 if needed.

This stops all traffic from being sent through the VPN, allowing NetCatcher's static routes to control which destinations use the tunnel.

## Notes

- If a local DNS proxy or global proxy is active, domain names may resolve to incorrect IP addresses. Disable the proxy before starting NetCatcher if you rely on hostname-based routes.
- Routes are re-resolved fresh on each connect event, so DNS changes are picked up automatically when the interface reconnects.
