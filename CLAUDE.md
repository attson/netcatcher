# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

### Prerequisites

- Go 1.25+
- Node.js 20+

### Build

```bash
# Build frontend first
cd frontend && npm ci && npx vite build

# Build Go binary
go build -o build/bin/netcatcher-app .
```

### Run

```bash
./build/bin/netcatcher-app
```

On macOS the app prompts for admin credentials on first route operation (creates a sudoers rule for `/sbin/route`). No need for `sudo`.

### CI / Release

GitHub Actions workflow at `.github/workflows/release.yaml` triggers on tag push (`v*`). Builds:
- macOS arm64 + amd64: packaged as `.dmg` containing a `.app` bundle
- Windows amd64: `.exe`

The CI does NOT use `wails3 build` (which requires a Taskfile). It runs `npx vite build` + `go build` directly.

### Versioning

The version shown in Settings > About and in the macOS `Info.plist` is injected at build time:

- CI (tag push): derived from the git tag, e.g. `v1.2.3` → `1.2.3`.
- Local build: `vite.config.js` runs `git rev-parse --short HEAD` and uses `dev-<sha>`; falls back to `dev` if not in a git repo.
- Override: set `APP_VERSION` env var before `npx vite build` (and before the macOS packaging step) to force a specific version.

Vite exposes the resolved value as the global `__APP_VERSION__`, read by `frontend/src/views/Settings.vue`. The i18n strings use a `{version}` placeholder.

### Tests

```bash
go test ./config/ ./logbuffer/ ./netcatcher/ -v
go test -run TestManager -v
```

## Architecture

NetCatcher is a Wails v3 desktop application. The frontend is Vue 3; the backend is Go. The app monitors network interfaces and automatically adds static routes when they connect.

**Data flow:**

1. `main.go` — initialises the Wails application, creates an `App` struct, and registers it as a binding.
2. `App` struct (bindings) — exposes methods callable from the frontend. Delegates interface monitoring to `Manager`.
3. `Manager` — owns the set of running `NetCatcher` goroutines, one per configured interface. Handles start/stop lifecycle and config reload.
4. `NetCatcher` instances (`netcatcher/netcatcher.go`) — poll `net.InterfaceByName()` every 1 second. On connect: extract gateway IP, resolve configured routes (DNS / CIDR / plain IP), call `route.AddRoute()`. Routes are re-resolved fresh on each connect event. Domain lookups use a UDP socket bound to the monitored interface via `IP_BOUND_IF` (macOS) / `IP_UNICAST_IF` (Windows) — see `netcatcher/bind_*.go` and `netcatcher/resolver.go` — so TUN-mode proxies cannot hijack lookups with fake IPs. Falls back to `net.LookupIP` if the bound lookup fails.
5. `route/route_darwin.go` / `route/route_windows.go` — thin OS wrappers that exec the native `route` (macOS) or `route.exe` (Windows) command. macOS uses a one-time `osascript` admin prompt to install a privileged helper at `/usr/local/sbin/netcatcher-resolver-helper` and write `/etc/sudoers.d/netcatcher` allowing passwordless `sudo /sbin/route` and `sudo -n netcatcher-resolver-helper`. Subsequent calls — including `/etc/resolver/` management — never prompt again. Windows wraps output in a GBK→UTF-8 converter (`golang.org/x/text`).
6. `vpnname_darwin.go` — uses `scutil --nc list` to map network interface names to VPN service names (e.g. `ppp0` → `玩心不止`).

**TUN-mode compatibility (opt-in, `tunMode: true`):**

When a host runs a TUN-mode proxy (Clash / Mihomo / Surge) that hijacks all DNS traffic at the utun layer, even interface-bound lookups are compromised and applications get fake IPs. In this mode the `Manager` additionally:

- Starts a UDP DNS forwarder on `127.0.0.1:15353` (`netcatcher/dnsforwarder.go`). Incoming queries are dispatched by QNAME suffix to the right monitored interface, then forwarded with `IP_BOUND_IF` set so they bypass the TUN interception.
- Creates `/etc/resolver/<domain>` files for each domain route, each containing `nameserver 127.0.0.1` + `port 15353`. macOS's `mDNSResponder` then routes matching lookups to the forwarder while all other queries continue to hit the system default resolver (which typically returns fake IPs for TUN to handle).
- Teardown on monitor stop removes `/etc/resolver/` entries via the helper.

The privileged helper `route/resolver_darwin.go` only accepts `install <port> <domain>...` and `remove <domain>...` with strict argument validation, so the broad sudoers rule has a narrow attack surface.

**Frontend:**

- Vue 3 + Pinia stores for state management.
- vue-i18n for internationalization (English + Chinese). Locale files at `frontend/src/i18n/en.json` and `frontend/src/i18n/zh-CN.json`.
- `@wailsio/runtime` for calling Go methods (`Call.ByName('main.App.Method', ...args)`) and events (`Events.On('event', cb)`).
- Wails Events for real-time updates pushed from the backend (route changes, log lines, interface status).
- Views: dashboard (includes per-interface edit mode for routes + DNS), real-time log viewer, settings (including TUN proxy compatibility toggle).
- Dashboard lists all system network interfaces with VPN service names for easy selection. Each interface card has its own **Edit** / **Apply** / **Cancel** — there is no global save button. Routes dim + strike-through when the monitor is stopped or the interface is disconnected. DNS servers are edited inline next to the gateway as removable chips.

**i18n:**

- Supported languages: English (`en`), Chinese (`zh-CN`).
- Language preference saved to `localStorage('locale')`. Auto-detects browser language on first run.
- All user-facing strings are in locale JSON files. Use `$t('key')` in templates, `t('key')` in script setup via `useI18n()`.

**System tray:**

The app runs in the system tray. Closing the main window hides it rather than quitting; use the tray menu to quit.

**Config storage:**

Config is stored at a platform-specific path:
- macOS: `~/Library/Application Support/NetCatcher/config.json`
- Windows: `%APPDATA%\NetCatcher\config.json`

**Config format** (see `config-example.json`):
```json
{
  "tunMode": false,
  "interfaces": [
    {
      "name": "ppp0",
      "dns": ["114.114.114.114"],
      "routes": ["github.com", "192.168.188.11", "192.168.188.0/24"]
    }
  ]
}
```

- `tunMode` (optional, default `false`) — when `true`, `Manager.Start` also spins up the DNS forwarder and writes `/etc/resolver/` entries. Leave off in plain setups.
- `interfaces[].dns` (optional) — DNS servers queried via the monitored interface. Used both by domain lookups during route resolution AND by the DNS forwarder when `tunMode` is on. Falls back to the interface gateway and then the system resolver when empty.

## Platform Notes

- macOS: the app prompts for admin password once on the first operation that requires it. The osascript sets up `/etc/sudoers.d/netcatcher` granting passwordless `/sbin/route` + `/usr/local/sbin/netcatcher-resolver-helper`, and installs the helper script (root-owned, 0755). Every subsequent route / `/etc/resolver` operation goes through `sudo -n` silently. `authUpToDate()` in `route/route_darwin.go` skips the prompt on startup if both files already exist.
- Windows: disable "Use default gateway on remote network" in the VPN adapter settings so routes don't conflict. Build with `go build -ldflags="-H=windowsgui" ...` so the `.exe` runs without popping a console window (CI already does this). TUN mode (`/etc/resolver` equivalent via NRPT) is not implemented on Windows yet — the helpers are stubs.
- Both platforms require administrator/root privileges to modify the routing table.
