# NetCatcher

[中文文档](README.zh-CN.md)

A desktop application that monitors network interfaces and automatically adds static routes when they connect. Useful when running multiple network interfaces or VPN connections and you need specific traffic to flow through a particular interface.

## Features

- Dashboard showing active interfaces and their route status; each interface card has its own Edit / Apply / Cancel flow
- Per-interface route and DNS config editor — manage interfaces, routes, and DNS servers through the GUI
- Routes visually dim + strike-through when the monitor is stopped or the interface is disconnected, so state is obvious at a glance
- Real-time log viewer — see route add/remove events as they happen
- Route connectivity testing (ping)
- **TUN proxy compatibility** (opt-in) — runs a local DNS forwarder + `/etc/resolver/` entries so domain routes resolve to real IPs when the host uses a TUN-mode proxy (Clash / Mihomo / Surge)
- DNS queries for route resolution are bound to the monitored interface (`IP_BOUND_IF` / `IP_UNICAST_IF`), bypassing TUN DNS hijacking
- System notifications on interface connect/disconnect
- Multi-language support (English / Chinese)
- Settings panel for app preferences (auto-start, notifications, language, TUN compatibility)
- System tray integration — the app hides to the tray when the window is closed; use the tray menu to quit

## Installation

Download the latest release from the [Releases](https://github.com/attson/netcatcher/releases) page:

| Platform | File |
|----------|------|
| macOS (Apple Silicon) | `NetCatcher-arm64.dmg` |
| macOS (Intel) | `NetCatcher-amd64.dmg` |
| Windows | `NetCatcher-amd64.exe` |

**macOS:** Open the `.dmg` and drag NetCatcher to Applications. On first route operation the app prompts for your admin password once — no need for `sudo`.

> **"NetCatcher is damaged and can't be opened"?** The app is ad-hoc signed, not notarized with an Apple Developer ID, so Gatekeeper rejects it once macOS adds the quarantine attribute during DMG download. Remove the attribute once:
>
> ```bash
> sudo xattr -cr /Applications/NetCatcher.app
> ```
>
> Then open normally. You only need to do this after each update.

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

The main screen shows status overview and per-interface route configuration. System network interfaces are listed in a dropdown with VPN service names (e.g. `ppp0 (My VPN)`) for easy identification.

- Select an interface from the dropdown and click **Add Interface** — the new card opens in edit mode
- Click **Edit** on any existing card to enter edit mode; add or remove routes (domain names, IPs, CIDR blocks) and DNS servers (inline chips next to the gateway)
- Click **Apply** to save and apply, or **Cancel** to discard changes to that card
- Use **Ping** to test route connectivity; the latency shows next to the route
- Use **Start** / **Stop** to control monitoring — inactive routes dim + strike-through so you can see what is live
- Domain routes show their resolved IP addresses alongside the name

![Dashboard](doc/screenshots/dashboard.png)

### Logs

Real-time log viewer showing interface up/down events, route add/remove operations, and errors. Filter by log level or search by keyword. Scrolling up pauses auto-scroll; scroll back to the bottom to resume.

![Logs](doc/screenshots/logs.png)

### Settings

- **Launch at startup** — register/unregister the app for auto-start on login.
- **Notifications** — toggle system notifications when interfaces connect or disconnect.
- **Language** — switch between English and Chinese. The preference persists across restarts.
- **TUN proxy compatibility** — when you have a TUN-mode proxy running (Clash / Mihomo / Surge), enable this so NetCatcher runs a local DNS forwarder and writes `/etc/resolver/<domain>` entries for each configured domain route. macOS then routes those lookups to the forwarder (bound to the VPN interface) and returns real IPs instead of the proxy's fake IPs. Leave off in plain setups.

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
  "tunMode": false,
  "interfaces": [
    {
      "name": "ppp0",
      "dns": ["114.114.114.114"],
      "routes": [
        "github.com",
        "192.168.188.11",
        "192.168.188.0/24"
      ]
    }
  ]
}
```

- `name` must match the OS network interface name exactly (e.g. the VPN adapter name).
- `dns` (optional) — DNS servers to query via the monitored interface when resolving domain routes. When `tunMode` is on, the local DNS forwarder also uses this list for matching queries.
- `tunMode` (optional, default `false`) — enable the TUN proxy compatibility flow. Equivalent to the Settings toggle.

## Platform Notes

### macOS

The app prompts for admin credentials once on the first operation that needs them. The osascript:

1. Installs a small privileged helper at `/usr/local/sbin/netcatcher-resolver-helper` (root-owned, 0755). The helper only accepts `install <port> <domain>...` and `remove <domain>...` with strict input validation.
2. Writes `/etc/sudoers.d/netcatcher` granting passwordless `sudo /sbin/route` and `sudo -n netcatcher-resolver-helper`.

Every subsequent route change and `/etc/resolver/` update goes through `sudo -n` silently — no further prompts. Routes and `/etc/resolver/` entries are cleaned up when the monitor stops.

### Windows

By default, a VPN connection routes all traffic through the tunnel. To use per-route configuration:

1. Open the VPN adapter properties (Network Settings -> VPN connection -> right-click -> Properties -> Networking).
2. Select Internet Protocol Version 4 (TCP/IPv4) -> Properties -> Advanced.
3. Uncheck "Use default gateway on remote network". Repeat for IPv6 if needed.

This stops all traffic from being sent through the VPN, allowing NetCatcher's static routes to control which destinations use the tunnel.

![vpn-info.png](doc/vpn-info.png)
![vpn-net.png](doc/vpn-net.png)
![modify- default.png](doc/modify-%20default.png)

## Auto-update

NetCatcher checks GitHub Releases for new versions on launch (after a 5s
delay) and every 24h while running. When a new release ships:

1. A banner appears under the navbar.
2. Click **Download** — the asset (`.app.tar.gz` on macOS, `.exe` on
   Windows) is downloaded into `~/Library/Application Support/NetCatcher/updates/`
   (macOS) or `%APPDATA%\NetCatcher\updates\` (Windows).
3. Downloads are verified against `SHA256SUMS`; release artifacts also
   include `SHA256SUMS.sig` signed with the project's Ed25519 key.
4. Click **Install & restart** to swap the bundle and relaunch.

You can disable auto-checks in Settings → Software Update. Skip a
specific version to silence the banner until a newer release lands.

### Setting up signing (maintainers only)

Run `go run ./cmd/updater-keygen` once, then store:
- `NETCATCHER_UPDATE_VERIFY_PUBLIC_KEY` — base64 public key — as a GitHub Secret.
- `NETCATCHER_UPDATE_SIGNING_PRIVATE_KEY` — base64 private key — also as a GitHub Secret.

The release workflow injects the public key into the binary at build
time and signs `SHA256SUMS` with the private key.

## Notes

- Route-resolution DNS queries are always bound to the monitored interface (`IP_BOUND_IF` / `IP_UNICAST_IF`), so a TUN-mode proxy cannot hijack them with fake IPs. If the bound lookup fails (unreachable DNS, no interface DNS configured), NetCatcher falls back to the system resolver.
- If the host runs a TUN-mode proxy (Clash / Mihomo / Surge) AND the applications themselves also need real IPs (not just NetCatcher's routing), turn on **TUN proxy compatibility** in Settings. Without it, your browser/terminal will still get fake IPs for the configured domains because the system DNS path is hijacked at the utun layer.
- NetCatcher automatically refreshes the system DNS cache after a VPN interface connects, a domain route is refreshed manually, or TUN resolver rules change. Browser-private DNS and connection pools are separate; if a page still reuses an old connection, close that connection in the browser or TUN proxy and retry.
- Routes and DNS lookups are re-run fresh on each connect event, so DNS changes are picked up automatically when the interface reconnects.
