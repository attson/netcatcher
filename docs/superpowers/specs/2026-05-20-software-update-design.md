# Software Update — Design

Status: Draft (brainstorming output)
Author: brainstorming session, 2026-05-20
Repo: attson/netcatcher
Reference implementation: attson/atterm (Wails v2, ~1900-line `desktop/updater.go`)

## 1. Goal

Give NetCatcher an in-app one-click update path on macOS and Windows: detect a new GitHub release, download the platform asset, verify it (SHA256 + Ed25519), swap in the new app, and relaunch — without sending the user back to the browser.

## 2. Scope

In scope:
- macOS (`.app.tar.gz`) and Windows (`.exe`) in-app upgrade.
- GitHub Releases API as the manifest source; no self-hosted update server.
- Ed25519 signature on a `SHA256SUMS` manifest; per-asset SHA256 verification.
- Settings panel section + top-of-app banner; en + zh-CN i18n.
- Skip-this-version, release-notes view, download progress.
- Auto-check default on (start + every 24h); user can disable.

Out of scope:
- Linux builds (NetCatcher does not ship Linux today).
- Delta / patch updates — always full asset download.
- Background download (download only after user clicks).
- Code-signing notarization (macOS Developer ID, Windows Authenticode). Existing ad-hoc signing stays unchanged; updater integrity rests on Ed25519 manifest.
- Multi-channel (stable / beta) — single channel keyed to `releases/latest`.

## 3. Architecture overview

```
┌─────────────── Frontend (Vue 3) ───────────────┐
│  App.vue → UpdateBanner.vue                     │
│  Settings.vue → SettingsUpdate.vue              │
│                  │                              │
│  stores/updater.js (Pinia)                      │
│   - state snapshot                              │
│   - Events.On('update:state-changed', ...)      │
│   - Call.ByName('main.App.<method>', ...)       │
└──────────────────┬──────────────────────────────┘
                   │ Wails Call / Event
                   ▼
┌──────────────── Go backend ─────────────────────┐
│  main.go: AppVersion, UpdateVerifyPublicKey      │
│           (ldflags-injected)                    │
│  app.go: 6 binding methods → updater.Updater    │
│                                                 │
│  updater/                                       │
│  ├─ updater.go  orchestrator + facade           │
│  ├─ state.go    State + transition lock         │
│  ├─ github.go   Release API + ETag cache        │
│  ├─ verify.go   Ed25519 + SHA256SUMS            │
│  ├─ installer_darwin.go  .tar.gz → /Applications│
│  ├─ installer_windows.go .exe self-replace      │
│  └─ scripts/  install-darwin.sh, install-windows.ps1
│              (//go:embed)                       │
│                                                 │
│  config/config.go: UpdaterConfig                │
└──────────────────┬──────────────────────────────┘
                   │ HTTPS
                   ▼
              GitHub Releases API
```

### Invariants

- `updater/` package depends only on `config/` (for `UpdaterConfig` shape) and stdlib + `golang.org/x/mod/semver`. It does **not** import `netcatcher/`, `route/`, or any business package.
- Public key is injected at compile time. Empty key → development build → Ed25519 verification is skipped but SHA256 verification still runs; the 24h loop is also disabled when `AppVersion == "dev"`.
- The main Go process never holds the new asset's installation lock — replacement happens inside an embedded shell/PowerShell helper that runs after the main process quits.
- Persisted state lives in the existing config file (`~/Library/Application Support/NetCatcher/config.json` on macOS, `%APPDATA%\NetCatcher\config.json` on Windows). Runtime fields (last-check timestamp, download progress) stay in memory.

## 4. Go interfaces

### 4.1 State (`updater/state.go`)

```go
type Status string

const (
    StatusIdle        Status = "idle"
    StatusChecking    Status = "checking"
    StatusAvailable   Status = "available"
    StatusDownloading Status = "downloading"
    StatusReady       Status = "ready"   // downloaded, awaiting install
    StatusError       Status = "error"
)

type State struct {
    Status         Status    `json:"status"`
    CurrentVersion string    `json:"currentVersion"`
    LatestVersion  string    `json:"latestVersion,omitempty"`
    ReleaseNotes   string    `json:"releaseNotes,omitempty"`
    ReleaseURL     string    `json:"releaseUrl,omitempty"`
    DownloadPct    int       `json:"downloadPct"`
    AssetSize      int64     `json:"assetSize,omitempty"`
    Error          string    `json:"error,omitempty"`
    LastCheckedAt  time.Time `json:"lastCheckedAt,omitempty"`
    SkippedVersion string    `json:"skippedVersion,omitempty"`
}
```

A `*stateStore` wraps `State` with a `sync.RWMutex` and a `transition(func(*State))` helper. Every transition calls the registered `Emit` to push `update:state-changed`.

### 4.2 Orchestrator (`updater/updater.go`)

```go
type Options struct {
    CurrentVersion string                  // ldflags
    PublicKey      string                  // base64, "" = dev build
    Repo           string                  // "attson/netcatcher"
    ConfigDir      string                  // os-specific
    HTTPClient     *http.Client            // injectable for tests
    Emit           func(event string, data any)
    Logger         *slog.Logger
    Now            func() time.Time        // injectable for tests; drives 24h ticker
}

type Updater struct { /* private */ }

func New(opts Options) (*Updater, error)
func (u *Updater) Start(ctx context.Context)               // 5s delay + 24h ticker
func (u *Updater) State() State                            // snapshot copy
func (u *Updater) Check(ctx context.Context, force bool) error
func (u *Updater) Download(ctx context.Context) error
func (u *Updater) InstallAndQuit(ctx context.Context) error // launches helper, returns; caller calls wailsApp.Quit()
func (u *Updater) Skip(version string) error
func (u *Updater) SetAutoCheck(enabled bool) error
```

Concurrency rules:
- `Check`, `Download`, `InstallAndQuit` are mutually exclusive. A call while another is in flight returns the current state without erroring.
- `Check` uses ETag caching with a 1h soft TTL; `force=true` bypasses the cache.
- Semver comparison uses `golang.org/x/mod/semver` (`semver.Compare("v"+a, "v"+b)`).
- Download path: `<ConfigDir>/updates/<version>/<asset>.partial`; atomic rename to drop `.partial` after verification.

### 4.3 GitHub client (`updater/github.go`)

- `GET https://api.github.com/repos/{Repo}/releases/latest` with `Accept: application/vnd.github+json`, `If-None-Match: <etag>`.
- Parses `tag_name`, `body` (release notes), and `assets[]`. Selects the asset matching `osArch()`:
  - `darwin/arm64` → `NetCatcher-arm64.app.tar.gz`
  - `darwin/amd64` → `NetCatcher-amd64.app.tar.gz`
  - `windows/amd64` → `NetCatcher-amd64.exe`
- Also locates `SHA256SUMS` and `SHA256SUMS.sig` assets in the same release.
- 403 with `X-RateLimit-Remaining: 0` → returns a `RateLimitError`; the updater records it in `State.Error` but does **not** fall back to a redirect-based discovery (atterm has this; we skip the second path to keep the surface small).
- If no asset matches the current OS/arch, returns `ErrNoAssetForPlatform` carrying the expected filename so the UI can render an actionable error.

### 4.4 Verification (`updater/verify.go`)

```go
// VerifyChecksums returns nil iff:
//  - publicKey == "" OR ed25519.Verify(publicKey, sumsBytes, sigBytes) == true
//  - assetSHA256 matches the line for assetName in sumsBytes
func VerifyChecksums(publicKey string, sumsBytes, sigBytes []byte, assetName string, assetSHA256 []byte) error
```

Two failure modes are surfaced distinctly:
- SHA mismatch → delete the download; safe to retry later.
- Ed25519 failure → keep the download on disk (forensics), mark `State.Error`, refuse to proceed. This is a serious signal (compromised release or misconfigured CI secrets).

### 4.5 Installer

Both installers share a contract: `install(ctx, assetPath, logger) error`. They extract/stage the new build, write a helper script into a temp dir, launch it detached with the current PID as the first arg, and return immediately. The caller (`app.go`) then calls `wailsApp.Quit()`.

```go
// updater/installer_darwin.go
func install(ctx context.Context, assetPath string, logger *slog.Logger) error
//   1. Resolve current .app path: exe, _ := os.Executable();
//      appPath := filepath.Dir(filepath.Dir(filepath.Dir(exe)))  // .app/Contents/MacOS/NetCatcher -> .app
//      (error out if !strings.HasSuffix(appPath, ".app"))
//   2. tar -xzf assetPath -C <staging-temp> → produces NetCatcher.app inside <staging-temp>.
//   3. Write embedded install-darwin.sh to <staging-temp>/install.sh, chmod +x.
//   4. exec.Command("/bin/bash", scriptPath, parentPID, stagedAppPath, currentAppPath).Start()
//   5. Return nil.

// updater/installer_windows.go
func install(ctx context.Context, assetPath string, logger *slog.Logger) error
//   1. Resolve current .exe path (os.Executable()).
//   2. Stage new exe at <temp>/NetCatcher-new.exe (asset is already the raw exe).
//   3. Write embedded install-windows.ps1 to <temp>\install.ps1.
//   4. exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass",
//         "-File", scriptPath, "-ParentPid", pid, "-StagedExe", staged,
//         "-TargetExe", current).Start()
//   5. Return nil.
```

### 4.6 Bindings (`app.go`, added next to existing methods)

```go
func (a *App) GetUpdateState() updater.State
func (a *App) CheckUpdate(force bool) error
func (a *App) StartDownload() error
func (a *App) InstallAndQuit() error          // calls updater.InstallAndQuit then a.app.Quit()
func (a *App) SkipVersion(version string) error
func (a *App) SetAutoCheck(enabled bool) error
```

`main.go` wires the updater after the existing manager initialisation:

```go
upd, err := updater.New(updater.Options{
    CurrentVersion: AppVersion,
    PublicKey:      UpdateVerifyPublicKey,
    Repo:           "attson/netcatcher",
    ConfigDir:      configDir,
    Emit:           wailsApp.Event.Emit,
    Logger:         logger,
})
if err != nil { log.Fatal(err) }
app.updater = upd
go upd.Start(ctx)
```

## 5. Frontend

### 5.1 Pinia store — `frontend/src/stores/updater.js`

- Holds the latest `State` snapshot plus a session-only `dismissed` flag (resets on page reload).
- On mount: one `GetUpdateState` call to seed; subscribes to `update:state-changed`.
- Exposes `check`, `download`, `installAndQuit`, `skip`, `setAutoCheck`, `dismiss`.
- Derived `showBanner` returns `true` when status ∈ {`available`, `downloading`, `ready`}, the latest version is not skipped, and `dismissed` is false.

### 5.2 `UpdateBanner.vue`

Mounted in `App.vue` directly under the navbar. Three layouts keyed by `state.status`:

- `available`: `发现新版本 vX.Y.Z   [查看说明 ▾] [跳过此版本] [立即下载] [稍后]`
- `downloading`: `正在下载 vX.Y.Z …  ██████░░ 67%   [取消]`
- `ready`: `vX.Y.Z 已就绪，重启以应用更新   [立即重启] [稍后]`

32px tall, accent-soft background, full-width. Errors do **not** appear here — they show in Settings only. "查看说明" expands a `pre-wrap` block with `max-height: 200px; overflow-y: auto` showing the release notes.

### 5.3 `SettingsUpdate.vue`

A `<section>` injected into `Settings.vue` above the existing "About" section. Layout:

```
软件更新 / Software Update
─────────────────────────────
当前版本：1.3.2
最新版本：1.4.0  (已就绪 / 已是最新 / 未检查 / 检查失败：…)
最后检查：2026-05-20 14:32
─────────────────────────────
[立即检查更新]  [立即下载]  [立即安装并重启]   ← enabled per-status
[ ] 启动时自动检查更新（每 24 小时）
─────────────────────────────
发布说明（可折叠）
```

When `SkippedVersion` is set, the latest-version row gets a "已跳过 vX.Y.Z [取消跳过]" suffix.

For a dev build (`CurrentVersion == "dev"`), the section shows a disabled placeholder: "Development build — updates disabled."

### 5.4 i18n keys

Added under `settings.update.*` and `banner.update.*` in `frontend/src/i18n/en.json` and `zh-CN.json`. Keys include `title`, `currentVersion`, `latestVersion`, `lastChecked`, `autoCheck`, `checkNow`, `download`, `installRestart`, `notes`, `skipped`, `status.{idle,checking,available,downloading,ready,error,upToDate}`, plus the banner equivalents (`available`, `downloading`, `ready`, `viewNotes`, `skip`, `download`, `later`, `installRestart`, `cancel`).

## 6. Configuration

```go
// config/config.go
type Config struct {
    TUNMode    bool              `json:"tunMode"`
    Interfaces []InterfaceConfig `json:"interfaces"`
    Updater    UpdaterConfig     `json:"updater"`
}

type UpdaterConfig struct {
    AutoCheck      bool   `json:"autoCheck"`
    SkippedVersion string `json:"skippedVersion,omitempty"`
}
```

Migration: old config files have no `updater` key → JSON decodes to zero value. `Load()` defaults `AutoCheck = true` when the field is the zero value AND the config file was missing the key entirely (use the existing same pattern that `TUNMode` follows). Existing test files in `config/` get one new case for the migration.

## 7. Signing & CI

### 7.1 Key generation (one-off, manual)

A small helper command at `cmd/updater-keygen/main.go`:

- Generates an Ed25519 keypair.
- Prints base64 of the 32-byte public key and the 64-byte private seed/key.
- The maintainer copies them into GitHub Secrets:
  - `NETCATCHER_UPDATE_VERIFY_PUBLIC_KEY` — public key (base64, 32 bytes after decode).
  - `NETCATCHER_UPDATE_SIGNING_PRIVATE_KEY` — private key (base64, 64 bytes after decode).

The key material is never checked into the repo.

### 7.2 `main.go` ldflags variables

```go
var (
    AppVersion            = "dev"
    UpdateVerifyPublicKey = ""
)
```

Existing `vite.config.js` keeps doing its job for the frontend version display; the Go side now also has a canonical version string.

### 7.3 `.github/workflows/release.yaml` changes

1. Set `APP_VERSION` and `UPDATE_VERIFY_PUBLIC_KEY` in the job env.
2. The existing `go build` step adds `-X main.AppVersion=${APP_VERSION#v}` and `-X main.UpdateVerifyPublicKey=${UPDATE_VERIFY_PUBLIC_KEY}` to its ldflags.
3. After the macOS `.dmg` step, add a tar step:
   ```
   tar -czf build/bin/NetCatcher-${ARCH}.app.tar.gz -C build NetCatcher.app
   ```
4. New job `sign-checksums`:
   - `needs: [build-macos, build-windows]`
   - Downloads all build artifacts to `artifacts/`.
   - Computes `SHA256SUMS` listing `.dmg`, `.app.tar.gz`, and `.exe` files (paths normalised to bare filenames).
   - Runs `go run ./.github/scripts/sign-checksums.go SHA256SUMS SHA256SUMS.sig` using `NETCATCHER_UPDATE_SIGNING_PRIVATE_KEY`.
   - Uploads `SHA256SUMS` + `SHA256SUMS.sig` as artifacts.
5. The existing `softprops/action-gh-release` step adds `SHA256SUMS` and `SHA256SUMS.sig` to its `files:` list.

### 7.4 `.github/scripts/sign-checksums.go`

Roughly 30 lines: reads the env-provided base64 private key, reads the input file bytes, calls `ed25519.Sign`, writes base64 of the signature to the output path.

## 8. Platform helper scripts (embedded via `//go:embed`)

### 8.1 `updater/scripts/install-darwin.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail
PARENT_PID="$1"; STAGED="$2"; APP_PATH="$3"

for _ in $(seq 1 60); do
    kill -0 "$PARENT_PID" 2>/dev/null || break
    sleep 0.5
done
kill -0 "$PARENT_PID" 2>/dev/null && exit 11

rm -rf "$APP_PATH"
mv "$STAGED" "$APP_PATH"
xattr -dr com.apple.quarantine "$APP_PATH" 2>/dev/null || true

open -a "$APP_PATH"
```

No `sudo`. If `mv` fails because the user runs NetCatcher from an unwritable location, the script exits non-zero and the *next* startup of the old app will detect the unfinished install (see §9 below).

### 8.2 `updater/scripts/install-windows.ps1`

```powershell
param([int]$ParentPid, [string]$StagedExe, [string]$TargetExe)

for ($i = 0; $i -lt 60; $i++) {
    if (-not (Get-Process -Id $ParentPid -ErrorAction SilentlyContinue)) { break }
    Start-Sleep -Milliseconds 500
}

$old = "$TargetExe.old"
if (Test-Path $old) { Remove-Item $old -Force -ErrorAction SilentlyContinue }

# Retry up to 3 times for AV/lock contention.
for ($i = 0; $i -lt 3; $i++) {
    try {
        Rename-Item -Path $TargetExe -NewName ([System.IO.Path]::GetFileName($old)) -ErrorAction Stop
        Copy-Item -Path $StagedExe -Destination $TargetExe -Force -ErrorAction Stop
        break
    } catch {
        Start-Sleep -Seconds 1
        if ($i -eq 2) { throw }
    }
}

Start-Process -FilePath $TargetExe
```

Because the existing `feat/windows-admin-manifest` work bakes `requestedExecutionLevel=requireAdministrator` into the exe, the relaunch goes through UAC the same way a manual start does. The user already accepts that prompt as part of NetCatcher's normal flow.

## 9. Error handling

| Scenario | Behaviour |
|---|---|
| GitHub API rate limited (403, `X-RateLimit-Remaining: 0`) | `State.Error = "API rate limit, retrying later"`; auto-loop keeps ticking; banner not shown. |
| Network error during check or download | Drop `.partial`; `Status = error`; UI offers retry; no automatic retry. |
| SHA256 mismatch | Delete downloaded file; `Error = "checksum mismatch"`; do not retry automatically. |
| Ed25519 signature failure | Keep file on disk for forensics; `Error = "signature verification failed"`; refuse to proceed. |
| Asset for the running OS/arch missing in the release | `Status = error`, message names the missing asset filename. |
| Unzip fails / target path unwritable (macOS) | `Status = error`; banner provides a "在 Finder 中显示" link to the staged `.app`. |
| Windows rename fails 3x (AV lock) | `Status = error`; user-visible message says to disable AV or retry. |
| Helper script exits non-zero / parent never exits | Detected on next launch: `main.go` startup runs a small `updater.CleanupStale(configDir)` that removes leftover `<exe>.old` (Windows) and `<ConfigDir>/updates/*/staging` (macOS/Windows); emits a log line, no user-facing error. |
| User closes window mid-download | Window hides to tray; download continues; tray notification when ready. |
| User defers a `ready` state | `Status = ready` persists; on next launch the banner re-appears immediately. |
| Dev build (`AppVersion == "dev"`) | `Start()` short-circuits; all binding methods return `errors.New("updates disabled in development build")`; Settings shows the disabled placeholder. |
| `SkippedVersion == LatestVersion` | Banner hidden; Settings shows skipped state with "取消跳过" button. |

## 10. Testing

### Unit (in `updater/`)

- `state_test.go` — legal transitions (`idle→checking→available→downloading→ready`); illegal transitions return error; concurrent calls serialise.
- `github_test.go` — `httptest.Server` mock covering: normal 200, 304 (ETag hit), 403 rate-limit, network error, asset-missing.
- `verify_test.go` — generates a fresh Ed25519 keypair in-test, builds a `SHA256SUMS`, signs it; cases: happy path, tampered sums, bad signature, empty public key (Ed25519 skipped but SHA256 enforced), wrong asset name.
- `updater_test.go` — orchestrator end-to-end: mocks the GitHub client, writes a fake asset + sums to `t.TempDir()`, runs `Check → Download → Verify`, asserts the resulting `State`.

### Manual verification (recorded in the spec's checklist for the implementer)

- macOS: build an old `1.0.0` `.app` with a test public key, install in `/Applications`. Build a `1.0.1` `.app.tar.gz` + sums signed with the matching test private key. Serve from `python3 -m http.server` and point `updater.Options.Repo` at a local GitHub API mock (`httptest` style binary). Run check → download → install; confirm the new `/Applications/NetCatcher.app` is on `1.0.1` and the app relaunched.
- Windows: same flow with `.exe`; confirm `<exe>.old` cleanup happens on the next start.
- Dev build: confirm `Start()` is a no-op, settings UI shows the disabled placeholder, all bindings return the development-build error.

### Not in scope

- No CI runner exercises real end-to-end upgrades (GitHub runners can't validate a relaunched GUI app on the host).
- No Vue component tests — the project currently has no frontend test setup; we do not add one for this feature.

## 11. Open questions / future work

- macOS Developer ID signing + notarization would let us drop the ad-hoc signing workaround in the helper (`xattr -dr com.apple.quarantine`); tracked as a follow-up.
- Background download (start as soon as an update is detected) — explicitly deferred. Current model is "show banner, user clicks download".
- Multi-channel (stable / beta) — single channel for now; revisit if a beta program is created.
