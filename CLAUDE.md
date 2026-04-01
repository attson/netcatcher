# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build
go build -o netcatcher

# Run (requires root/admin privileges)
sudo ./netcatcher -c config.json
sudo ./netcatcher -c config.json -l /tmp/netcatcher.log

# Print available network interfaces (no config needed)
sudo ./netcatcher -c config.json
```

Releases are built via GoReleaser (see `.goreleaser.yaml`) for macOS (amd64/arm64) and Windows (amd64/arm64/386). CI triggers on tag push via `.github/workflows/release.yaml`.

There are no tests and no linter configured.

## Architecture

NetCatcher monitors network interfaces and automatically adds static routes when they connect.

**Data flow:**

1. `main.go` — reads JSON config, spawns one `NetCatcher` goroutine per interface, waits for SIGINT/SIGTERM to trigger cleanup.
2. `netcatcher/netcatcher.go` — polls `net.InterfaceByName()` every 1 second. On connect: extracts gateway IP from the interface address, resolves configured routes (DNS lookup / parse CIDR / parse IP), calls `route.AddRoute()`. Routes are re-resolved fresh on each connect event.
3. `route/route_darwin.go` / `route/route_windows.go` — thin OS wrappers that exec the native `route` (macOS) or `route.exe` (Windows) command. Windows variant wraps output in a GBK→UTF-8 converter (`golang.org/x/text`).

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
`config.json` is git-ignored; use `config-example.json` as a template.

## Platform Notes

- macOS: `install/darwin.sh` installs the binary and registers a launchctl daemon for auto-start.
- Windows: disable "Use default gateway on remote network" in the VPN adapter settings so routes don't conflict.
- Both platforms require administrator/root privileges to modify the routing table.
