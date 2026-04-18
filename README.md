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

## Installation

Download the latest release from the [Releases](https://github.com/attson/netcatcher/releases) page:

| Platform | File |
|----------|------|
| macOS (Apple Silicon) | `NetCatcher-arm64.dmg` |
| macOS (Intel) | `NetCatcher-amd64.dmg` |
| Windows | `NetCatcher-amd64.exe` |

**macOS:** Open the `.dmg` and drag NetCatcher to Applications. On first route operation the app prompts for your admin password once — no need for `sudo`.

**Windows:** Run `NetCatcher-amd64.exe` as Administrator.

## Building from Source

### Prerequisites

- Go 1.25+
- Node.js 20+

```bash
# Clone the repository
git clone https://github.com/attson/netcatcher.git
cd netcatcher

# Build frontend
cd frontend && npm ci && npx vite build && cd ..

# Build Go binary
go build -o build/bin/netcatcher-app .

# Run
./build/bin/netcatcher-app
```

## Usage

### Dashboard

The main screen shows status overview and inline route configuration. System network interfaces are listed in a dropdown with VPN service names (e.g. `ppp0 (My VPN)`) for easy identification.

- Select an interface from the dropdown and click **Add Interface**
- Expand the interface card to add routes — domain names, IPs, or CIDR blocks
- Click **Save & Apply** (appears only when changes are pending)
- Use **Ping** to test route connectivity
- Use **Start** / **Stop** to control monitoring
- Domain routes show their resolved IP addresses

![Dashboard](doc/screenshots/dashboard.png)

### Logs

Real-time log viewer showing interface up/down events, route add/remove operations, and errors. Filter by log level or search by keyword. Scrolling up pauses auto-scroll; scroll back to the bottom to resume.

![Logs](doc/screenshots/logs.png)

### Settings

- **Launch at startup** — register/unregister the app for auto-start on login.
- **Notifications** — toggle system notifications when interfaces connect or disconnect.
- **Language** — switch between English and Chinese. The preference persists across restarts.

![Settings](doc/screenshots/settings.png)

### System Tray

The app runs in the system tray. Closing the window hides it to the tray — the app keeps running and monitoring in the background. Right-click the tray icon to show the window or quit.

## Configuration

Config is stored at a platform-specific path and managed through the GUI:

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

The app prompts for admin credentials once on the first route operation. A sudoers rule is created to allow passwordless `route` calls afterwards. Routes are automatically cleaned up when the app exits.

### Windows

By default, a VPN connection routes all traffic through the tunnel. To use per-route configuration:

1. Open the VPN adapter properties (Network Settings -> VPN connection -> right-click -> Properties -> Networking).
2. Select Internet Protocol Version 4 (TCP/IPv4) -> Properties -> Advanced.
3. Uncheck "Use default gateway on remote network". Repeat for IPv6 if needed.

This stops all traffic from being sent through the VPN, allowing NetCatcher's static routes to control which destinations use the tunnel.

![vpn-info.png](doc/vpn-info.png)
![vpn-net.png](doc/vpn-net.png)
![modify- default.png](doc/modify-%20default.png)

## Notes

- If a local DNS proxy or global proxy is active, domain names may resolve to incorrect IP addresses. Disable the proxy before starting NetCatcher if you rely on hostname-based routes.
- Routes are re-resolved fresh on each connect event, so DNS changes are picked up automatically when the interface reconnects.
