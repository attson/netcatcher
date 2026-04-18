# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

### Prerequisites

- Go 1.25+
- Node.js 20+
- Wails v3 CLI (`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`)

### Build

```bash
# Build frontend first
cd frontend && npm ci && npx vite build

# Build Go binary
go build -o build/bin/netcatcher-app .

# Or use the Wails CLI (handles both steps)
wails3 build
```

### Run

```bash
# Requires admin/root privileges for route management
sudo ./build/bin/netcatcher-app
```

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
4. `NetCatcher` instances (`netcatcher/netcatcher.go`) — poll `net.InterfaceByName()` every 1 second. On connect: extract gateway IP, resolve configured routes (DNS / CIDR / plain IP), call `route.AddRoute()`. Routes are re-resolved fresh on each connect event.
5. `route/route_darwin.go` / `route/route_windows.go` — thin OS wrappers that exec the native `route` (macOS) or `route.exe` (Windows) command. Windows wraps output in a GBK→UTF-8 converter (`golang.org/x/text`).

**Frontend:**

- Vue 3 + Pinia stores for state management.
- vue-i18n for internationalization (English + Chinese). Locale files at `frontend/src/i18n/en.json` and `frontend/src/i18n/zh-CN.json`.
- `@wailsio/runtime` for calling Go methods (`Call.ByName('main.App.Method', ...args)`) and events (`Events.On('event', cb)`).
- Wails Events for real-time updates pushed from the backend (route changes, log lines, interface status).
- Views: dashboard, route config editor, real-time log viewer, settings.

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
  "interfaces": [
    {
      "name": "ppp0",
      "routes": ["github.com", "192.168.188.11", "192.168.188.0/24"]
    }
  ]
}
```

## Platform Notes

- macOS: requires root to modify the routing table. Run with `sudo` or grant the binary the necessary entitlements.
- Windows: disable "Use default gateway on remote network" in the VPN adapter settings so routes don't conflict.
- Both platforms require administrator/root privileges to modify the routing table.
