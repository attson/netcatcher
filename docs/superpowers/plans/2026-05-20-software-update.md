# Software Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an in-app software updater to NetCatcher: detect new GitHub releases, verify signed checksums, download, and swap-in the new build on macOS (`.app.tar.gz`) and Windows (`.exe`).

**Architecture:** A new `updater/` Go package (state machine + GitHub Releases client + Ed25519/SHA256 verifier + per-platform installer with embedded helper scripts) wired into the existing `App` binding layer. Vue 3 frontend gains a Pinia store, an `UpdateBanner` component mounted in `App.vue`, and a `SettingsUpdate` section inside the existing Settings view. CI gains `.app.tar.gz` packaging and a sign-checksums job that produces `SHA256SUMS` + `SHA256SUMS.sig` attached to each release.

**Tech Stack:** Go 1.25, Wails v3, `golang.org/x/mod/semver`, Ed25519 (stdlib `crypto/ed25519`), Vue 3 + Pinia + vue-i18n, GitHub Actions, embedded bash + PowerShell helper scripts.

**Spec:** `docs/superpowers/specs/2026-05-20-software-update-design.md`

---

## File Structure

**New files:**
- `updater/state.go` — `Status` enum, `State`, `stateStore` with `sync.RWMutex` and `transition()`.
- `updater/state_test.go` — transition legality, emit fires.
- `updater/github.go` — GitHub Releases API client, ETag cache, asset selection.
- `updater/github_test.go` — `httptest.Server`-driven coverage.
- `updater/verify.go` — Ed25519 signature + SHA256SUMS line lookup.
- `updater/verify_test.go` — happy/tampered/bad-sig/empty-key paths.
- `updater/installer_common.go` — `stagingDir()`, `osArch()`, shared paths.
- `updater/installer_darwin.go` — macOS install flow (build tag `darwin`).
- `updater/installer_windows.go` — Windows install flow (build tag `windows`).
- `updater/installer_other.go` — stub for unsupported platforms (build tag `!darwin && !windows`).
- `updater/scripts/install-darwin.sh` — embedded helper.
- `updater/scripts/install-windows.ps1` — embedded helper.
- `updater/scripts/scripts.go` — `//go:embed` declarations.
- `updater/cleanup.go` — `CleanupStale(configDir)` for leftover `<exe>.old` and staging dirs.
- `updater/updater.go` — `Updater`, `Options`, `New`, `Start`, `Check`, `Download`, `InstallAndQuit`, `Skip`, `SetAutoCheck`.
- `updater/updater_test.go` — orchestrator end-to-end with mocks.
- `cmd/updater-keygen/main.go` — Ed25519 keypair generator (one-off CLI).
- `.github/scripts/sign-checksums.go` — CI signing helper.
- `frontend/src/stores/updater.js` — Pinia store.
- `frontend/src/components/UpdateBanner.vue` — banner under navbar.
- `frontend/src/components/SettingsUpdate.vue` — settings section.

**Modified files:**
- `config/config.go` — add `UpdaterConfig` and migration in `Load()`.
- `config/config_test.go` — test for the new default.
- `main.go` — add `AppVersion` + `UpdateVerifyPublicKey` ldflags variables, wire `updater.Updater`, register cleanup, call `updater.CleanupStale` early.
- `app.go` — add 6 binding methods + an `updater` field on `App`.
- `frontend/src/App.vue` — mount `<UpdateBanner />`.
- `frontend/src/views/Settings.vue` — embed `<SettingsUpdate />`.
- `frontend/src/i18n/en.json` — new keys.
- `frontend/src/i18n/zh-CN.json` — new keys.
- `.github/workflows/release.yaml` — inject `UPDATE_VERIFY_PUBLIC_KEY`, build `.app.tar.gz`, add `sign-checksums` job.
- `go.mod` / `go.sum` — `golang.org/x/mod` added.

**Style/pattern reminders for the implementer:**
- Existing Go style: lowercase package names matching dir, table-driven tests, no testify (stdlib `testing` + `t.Errorf`/`t.Fatalf`).
- i18n keys in this project are flat strings like `"settings.general"`, **not** nested objects. Add new keys at the bottom of each JSON file with a blank line above the new group.
- Pinia stores use the composition-API style with `defineStore('name', () => { … })` (see `frontend/src/stores/monitor.js`).
- Wails v3 frontend calls: `await Call.ByName('main.App.MethodName', ...args)`; events: `Events.On('event:name', e => { /* e.data is the payload */ })`.

---

## Task 1: Extend config with `UpdaterConfig`

**Files:**
- Modify: `config/config.go`
- Modify: `config/config_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `config/config_test.go`:

```go
func TestLoadDefaultsAutoCheckTrue(t *testing.T) {
	// Existing file has no "updater" key (legacy users).
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"interfaces":[]}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Updater.AutoCheck {
		t.Fatalf("expected Updater.AutoCheck default true, got false")
	}
}

func TestLoadRespectsAutoCheckFalse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"interfaces":[],"updater":{"autoCheck":false}}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Updater.AutoCheck {
		t.Fatalf("expected AutoCheck=false to be preserved")
	}
}

func TestLoadPersistsSkippedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Config{Updater: UpdaterConfig{AutoCheck: true, SkippedVersion: "1.4.0"}}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Updater.SkippedVersion != "1.4.0" {
		t.Fatalf("expected SkippedVersion=1.4.0, got %q", loaded.Updater.SkippedVersion)
	}
}
```

Add `"os"` to the test file's imports if not present.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/attson/code/github.com.attson/netcatcher
go test ./config/ -run 'TestLoadDefaultsAutoCheckTrue|TestLoadRespectsAutoCheckFalse|TestLoadPersistsSkippedVersion' -v
```

Expected: FAIL (`UpdaterConfig` undefined).

- [ ] **Step 3: Add `UpdaterConfig` and migration**

Modify `config/config.go`. Add the new type below `Interface`:

```go
type UpdaterConfig struct {
	// AutoCheck controls whether the updater polls GitHub on startup and
	// every 24h. Defaults to true on legacy configs that lack the field.
	AutoCheck bool `json:"autoCheck"`
	// SkippedVersion is the latest version the user explicitly dismissed.
	// Banner stays hidden as long as the latest release equals this string.
	SkippedVersion string `json:"skippedVersion,omitempty"`
}
```

Add `Updater UpdaterConfig \`json:"updater"\`` to `Config`:

```go
type Config struct {
	Interfaces []Interface `json:"interfaces"`
	// TunMode enables a local DNS forwarder + /etc/resolver entries so that
	// domain routes resolve correctly when the host uses a TUN-mode proxy
	// (Clash / Mihomo / Surge). Leave off in plain setups.
	TunMode bool          `json:"tunMode,omitempty"`
	Updater UpdaterConfig `json:"updater"`
}
```

Update `Load()` — detect missing `updater` key in the raw JSON and default `AutoCheck` to `true`:

```go
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{Interfaces: []Interface{}, Updater: UpdaterConfig{AutoCheck: true}}, nil
		}
		return Config{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Interfaces == nil {
		cfg.Interfaces = []Interface{}
	}
	if _, hasUpdater := raw["updater"]; !hasUpdater {
		cfg.Updater.AutoCheck = true
	}
	return cfg, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./config/ -v
```

Expected: PASS for all six tests (three existing + three new).

- [ ] **Step 5: Commit**

```bash
git add config/config.go config/config_test.go
git commit -m "config: add UpdaterConfig with AutoCheck default true"
```

---

## Task 2: Add ldflags variables to `main.go`

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Add the two package-level variables**

Insert near the top of `main.go`, after the `var appIcon []byte` declaration:

```go
// Populated at build time via -ldflags="-X main.AppVersion=… -X main.UpdateVerifyPublicKey=…".
// Empty UpdateVerifyPublicKey means a development build: the updater skips
// Ed25519 verification (SHA256 is still enforced) and disables the 24h loop.
var (
	AppVersion            = "dev"
	UpdateVerifyPublicKey = ""
)
```

- [ ] **Step 2: Verify the build still compiles**

```bash
go build ./...
```

Expected: PASS, no output.

- [ ] **Step 3: Verify variables can be overridden by ldflags**

```bash
go build -ldflags="-X main.AppVersion=1.2.3 -X main.UpdateVerifyPublicKey=YWJj" -o /tmp/nc-test .
strings /tmp/nc-test | grep -F "1.2.3" | head -3
rm /tmp/nc-test
```

Expected: at least one line containing `1.2.3`.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "main: add AppVersion and UpdateVerifyPublicKey ldflags variables"
```

---

## Task 3: `updater/state.go` — State machine

**Files:**
- Create: `updater/state.go`
- Create: `updater/state_test.go`

- [ ] **Step 1: Write the failing tests**

Create `updater/state_test.go`:

```go
package updater

import (
	"sync"
	"testing"
	"time"
)

func TestStateStoreInitialSnapshot(t *testing.T) {
	st := newStateStore(State{CurrentVersion: "1.0.0"}, nil)
	snap := st.Snapshot()
	if snap.Status != StatusIdle {
		t.Fatalf("expected initial status idle, got %q", snap.Status)
	}
	if snap.CurrentVersion != "1.0.0" {
		t.Fatalf("expected current 1.0.0, got %q", snap.CurrentVersion)
	}
}

func TestStateStoreTransitionEmits(t *testing.T) {
	var (
		mu     sync.Mutex
		events []State
	)
	emit := func(event string, data any) {
		if event != "update:state-changed" {
			t.Errorf("unexpected event %q", event)
		}
		mu.Lock()
		events = append(events, data.(State))
		mu.Unlock()
	}
	st := newStateStore(State{CurrentVersion: "1.0.0"}, emit)

	st.transition(func(s *State) {
		s.Status = StatusChecking
		s.LastCheckedAt = time.Unix(0, 0)
	})

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 1 {
		t.Fatalf("expected 1 emit, got %d", len(events))
	}
	if events[0].Status != StatusChecking {
		t.Fatalf("expected emitted Status=checking, got %q", events[0].Status)
	}
}

func TestStateStoreConcurrentReadsAreSafe(t *testing.T) {
	st := newStateStore(State{CurrentVersion: "1.0.0"}, nil)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _ = st.Snapshot() }()
		go func() {
			defer wg.Done()
			st.transition(func(s *State) { s.DownloadPct++ })
		}()
	}
	wg.Wait()
	if st.Snapshot().DownloadPct != 100 {
		t.Fatalf("expected DownloadPct=100, got %d", st.Snapshot().DownloadPct)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./updater/ -v
```

Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Implement `state.go`**

Create `updater/state.go`:

```go
package updater

import (
	"sync"
	"time"
)

// Status is the high-level state of the updater state machine.
type Status string

const (
	StatusIdle        Status = "idle"
	StatusChecking    Status = "checking"
	StatusAvailable   Status = "available"
	StatusDownloading Status = "downloading"
	StatusReady       Status = "ready"
	StatusError       Status = "error"
)

// State is the snapshot the frontend renders. JSON tags match the
// frontend Pinia store field names.
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

// EmitFunc is the Wails event emit signature, kept minimal so tests can
// inject a recorder.
type EmitFunc func(event string, data any)

type stateStore struct {
	mu    sync.RWMutex
	state State
	emit  EmitFunc
}

func newStateStore(initial State, emit EmitFunc) *stateStore {
	if initial.Status == "" {
		initial.Status = StatusIdle
	}
	return &stateStore{state: initial, emit: emit}
}

// Snapshot returns a copy of the current state. Safe to call from any goroutine.
func (s *stateStore) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// transition mutates state under the write lock and emits the new snapshot.
// Always call via this method so the emit fires.
func (s *stateStore) transition(fn func(*State)) {
	s.mu.Lock()
	fn(&s.state)
	snap := s.state
	emit := s.emit
	s.mu.Unlock()
	if emit != nil {
		emit("update:state-changed", snap)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./updater/ -v
```

Expected: PASS for all three tests.

- [ ] **Step 5: Commit**

```bash
git add updater/state.go updater/state_test.go
git commit -m "updater: state machine with mutex-guarded transitions"
```

---

## Task 4: `updater/verify.go` — Signature + checksum verification

**Files:**
- Create: `updater/verify.go`
- Create: `updater/verify_test.go`

- [ ] **Step 1: Write the failing tests**

Create `updater/verify_test.go`:

```go
package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"
)

func makeKey(t *testing.T) (pubB64, privB64 string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	return base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv)
}

func sign(t *testing.T, privB64 string, data []byte) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil {
		t.Fatalf("decode priv: %v", err)
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(raw), data))
}

func sha(b []byte) []byte { h := sha256.Sum256(b); return h[:] }
func hexs(b []byte) string { return hex.EncodeToString(b) }

func TestVerifyChecksumsHappy(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", hexs(sha(asset))))
	pub, priv := makeKey(t)
	sig := sign(t, priv, sums)

	if err := VerifyChecksums(pub, sums, mustB64(sig), "NetCatcher-arm64.app.tar.gz", sha(asset)); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
}

func TestVerifyChecksumsTamperedSums(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", hexs(sha(asset))))
	pub, priv := makeKey(t)
	sig := sign(t, priv, sums)
	// Tamper.
	sums[0] ^= 0x01

	err := VerifyChecksums(pub, sums, mustB64(sig), "NetCatcher-arm64.app.tar.gz", sha(asset))
	if err == nil {
		t.Fatalf("expected signature failure on tampered sums")
	}
}

func TestVerifyChecksumsBadSig(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", hexs(sha(asset))))
	pub, _ := makeKey(t)
	_, otherPriv := makeKey(t)
	sig := sign(t, otherPriv, sums)

	if err := VerifyChecksums(pub, sums, mustB64(sig), "NetCatcher-arm64.app.tar.gz", sha(asset)); err == nil {
		t.Fatalf("expected signature failure with mismatched key")
	}
}

func TestVerifyChecksumsAssetMismatch(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", hexs(sha(asset))))
	pub, priv := makeKey(t)
	sig := sign(t, priv, sums)

	if err := VerifyChecksums(pub, sums, mustB64(sig), "NetCatcher-arm64.app.tar.gz", sha([]byte("different"))); err == nil {
		t.Fatalf("expected SHA mismatch error")
	}
}

func TestVerifyChecksumsAssetMissing(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  Other-Asset.zip\n", hexs(sha(asset))))
	pub, priv := makeKey(t)
	sig := sign(t, priv, sums)

	if err := VerifyChecksums(pub, sums, mustB64(sig), "NetCatcher-arm64.app.tar.gz", sha(asset)); err == nil {
		t.Fatalf("expected missing-asset error")
	}
}

func TestVerifyChecksumsEmptyPubKeySkipsEd25519(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", hexs(sha(asset))))

	// Empty pub key, any sig — must pass as long as SHA matches.
	if err := VerifyChecksums("", sums, nil, "NetCatcher-arm64.app.tar.gz", sha(asset)); err != nil {
		t.Fatalf("expected pass with empty pub key, got %v", err)
	}
}

func TestVerifyChecksumsEmptyPubKeyStillEnforcesSHA(t *testing.T) {
	asset := []byte("hello world")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", hexs(sha(asset))))

	if err := VerifyChecksums("", sums, nil, "NetCatcher-arm64.app.tar.gz", sha([]byte("nope"))); err == nil {
		t.Fatalf("expected SHA mismatch even with empty pub key")
	}
}

func mustB64(s string) []byte {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return raw
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./updater/ -run TestVerify -v
```

Expected: FAIL — `VerifyChecksums` undefined.

- [ ] **Step 3: Implement `verify.go`**

Create `updater/verify.go`:

```go
package updater

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrSignatureFailed is returned when Ed25519 verification of SHA256SUMS
// does not match. The download must be kept on disk for forensics.
var ErrSignatureFailed = errors.New("signature verification failed")

// ErrChecksumMismatch is returned when the asset's SHA256 does not match
// the line in SHA256SUMS. The download is safe to delete and retry.
var ErrChecksumMismatch = errors.New("checksum mismatch")

// ErrAssetNotInSums is returned when SHA256SUMS contains no line for the
// requested asset name.
var ErrAssetNotInSums = errors.New("asset not present in SHA256SUMS")

// VerifyChecksums returns nil iff:
//   - publicKeyB64 == "" OR ed25519.Verify(pub, sumsBytes, sigBytes) == true
//   - the line in sumsBytes for assetName has SHA256 hex matching assetSHA256
//
// sigBytes must be the raw 64-byte signature (not base64). Callers decode it.
// publicKeyB64 is base64 of the 32-byte public key.
func VerifyChecksums(publicKeyB64 string, sumsBytes, sigBytes []byte, assetName string, assetSHA256 []byte) error {
	if publicKeyB64 != "" {
		pub, err := base64.StdEncoding.DecodeString(publicKeyB64)
		if err != nil {
			return fmt.Errorf("decode public key: %w", err)
		}
		if len(pub) != ed25519.PublicKeySize {
			return fmt.Errorf("public key size %d, want %d", len(pub), ed25519.PublicKeySize)
		}
		if !ed25519.Verify(ed25519.PublicKey(pub), sumsBytes, sigBytes) {
			return ErrSignatureFailed
		}
	}

	wantHex, ok := findChecksum(sumsBytes, assetName)
	if !ok {
		return fmt.Errorf("%w: %s", ErrAssetNotInSums, assetName)
	}
	gotHex := hex.EncodeToString(assetSHA256)
	if !strings.EqualFold(wantHex, gotHex) {
		return fmt.Errorf("%w: want %s, got %s", ErrChecksumMismatch, wantHex, gotHex)
	}
	return nil
}

// findChecksum scans a `sha256sum`-style file. Each non-empty line is
// "<hex>  <filename>" or "<hex> *<filename>" (binary marker). Path
// separators are stripped — only the basename is compared.
func findChecksum(sumsBytes []byte, assetName string) (string, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(sumsBytes))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := fields[0]
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		// Normalise to basename.
		if idx := strings.LastIndexAny(name, "/\\"); idx >= 0 {
			name = name[idx+1:]
		}
		if name == assetName {
			return hash, true
		}
	}
	return "", false
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./updater/ -run TestVerify -v
```

Expected: PASS for all seven tests.

- [ ] **Step 5: Commit**

```bash
git add updater/verify.go updater/verify_test.go
git commit -m "updater: Ed25519 + SHA256SUMS verification"
```

---

## Task 5: `updater/github.go` — Release API client

**Files:**
- Create: `updater/github.go`
- Create: `updater/github_test.go`
- Modify: `go.mod` (add `golang.org/x/mod`)

- [ ] **Step 1: Add the `golang.org/x/mod` dependency**

```bash
cd /Users/attson/code/github.com.attson/netcatcher
go get golang.org/x/mod@latest
go mod tidy
```

Verify the import resolves:

```bash
go list -m golang.org/x/mod
```

Expected: prints `golang.org/x/mod vX.Y.Z`.

- [ ] **Step 2: Write the failing tests**

Create `updater/github_test.go`:

```go
package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newGHServer(t *testing.T, body any, statusCode int, headers map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		if etag := r.Header.Get("If-None-Match"); etag != "" && etag == headers["Etag"] {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func TestGitHubLatestSelectsMatchingAsset(t *testing.T) {
	body := map[string]any{
		"tag_name":   "v1.5.0",
		"body":       "release notes",
		"html_url":   "https://github.com/attson/netcatcher/releases/tag/v1.5.0",
		"assets": []map[string]any{
			{"name": "NetCatcher-amd64.app.tar.gz", "size": 12345, "browser_download_url": "https://example.test/a"},
			{"name": "NetCatcher-arm64.app.tar.gz", "size": 67890, "browser_download_url": "https://example.test/b"},
			{"name": "SHA256SUMS", "size": 256, "browser_download_url": "https://example.test/sums"},
			{"name": "SHA256SUMS.sig", "size": 88, "browser_download_url": "https://example.test/sig"},
		},
	}
	srv := newGHServer(t, body, 200, map[string]string{"Etag": "abc123"})
	defer srv.Close()

	c := newGitHubClient("attson/netcatcher", srv.Client(), srv.URL)
	rel, _, err := c.fetchLatest(context.Background(), "")
	if err != nil {
		t.Fatalf("fetchLatest: %v", err)
	}
	if rel.TagName != "v1.5.0" {
		t.Fatalf("tag: %q", rel.TagName)
	}

	asset, err := rel.AssetForPlatform("darwin", "arm64")
	if err != nil {
		t.Fatalf("AssetForPlatform: %v", err)
	}
	if asset.Name != "NetCatcher-arm64.app.tar.gz" {
		t.Fatalf("got %q", asset.Name)
	}

	sums, sig, err := rel.ChecksumAssets()
	if err != nil {
		t.Fatalf("ChecksumAssets: %v", err)
	}
	if sums.Name != "SHA256SUMS" || sig.Name != "SHA256SUMS.sig" {
		t.Fatalf("unexpected checksum assets: %s / %s", sums.Name, sig.Name)
	}
}

func TestGitHubLatestRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := newGitHubClient("attson/netcatcher", srv.Client(), srv.URL)
	_, _, err := c.fetchLatest(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error on 403")
	}
	if !isRateLimit(err) {
		t.Fatalf("expected rate-limit error, got %v", err)
	}
}

func TestGitHubLatestETagNotModified(t *testing.T) {
	body := map[string]any{"tag_name": "v1.0.0"}
	srv := newGHServer(t, body, 200, map[string]string{"Etag": "etag-1"})
	defer srv.Close()

	c := newGitHubClient("attson/netcatcher", srv.Client(), srv.URL)
	_, etag, err := c.fetchLatest(context.Background(), "")
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if etag != "etag-1" {
		t.Fatalf("got etag %q", etag)
	}

	_, _, err = c.fetchLatest(context.Background(), "etag-1")
	if err != ErrNotModified {
		t.Fatalf("expected ErrNotModified, got %v", err)
	}
}

func TestAssetForPlatformMissingReturnsTypedError(t *testing.T) {
	rel := &Release{
		TagName: "v1.0.0",
		Assets: []Asset{{Name: "SHA256SUMS"}},
	}
	_, err := rel.AssetForPlatform("darwin", "arm64")
	if err == nil {
		t.Fatalf("expected ErrNoAssetForPlatform")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./updater/ -run TestGitHub -v
```

Expected: FAIL — types undefined.

- [ ] **Step 4: Implement `github.go`**

Create `updater/github.go`:

```go
package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ErrNotModified is returned by fetchLatest when the server replied 304.
var ErrNotModified = errors.New("not modified")

// ErrNoAssetForPlatform is returned when the release has no asset matching
// the running OS/arch. The error message names the expected filename so
// the UI can show an actionable message.
type ErrNoAssetForPlatform struct {
	Want string
}

func (e *ErrNoAssetForPlatform) Error() string {
	return fmt.Sprintf("release has no asset for this platform; expected %s", e.Want)
}

// ErrRateLimited is returned when GitHub responds 403 with
// X-RateLimit-Remaining: 0.
var errRateLimited = errors.New("github api rate limited")

func isRateLimit(err error) bool { return errors.Is(err, errRateLimited) }

// Asset is a single file attached to a release.
type Asset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"browser_download_url"`
}

// Release is the subset of the GitHub Release payload the updater needs.
type Release struct {
	TagName string  `json:"tag_name"`
	Body    string  `json:"body"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// AssetForPlatform returns the asset whose name matches the convention
// NetCatcher-<arch>.app.tar.gz on darwin and NetCatcher-<arch>.exe on windows.
func (r *Release) AssetForPlatform(goos, goarch string) (Asset, error) {
	want := PlatformAssetName(goos, goarch)
	for _, a := range r.Assets {
		if a.Name == want {
			return a, nil
		}
	}
	return Asset{}, &ErrNoAssetForPlatform{Want: want}
}

// ChecksumAssets returns the SHA256SUMS and SHA256SUMS.sig assets, or an
// error if either is missing.
func (r *Release) ChecksumAssets() (sums Asset, sig Asset, err error) {
	var foundSums, foundSig bool
	for _, a := range r.Assets {
		switch a.Name {
		case "SHA256SUMS":
			sums = a
			foundSums = true
		case "SHA256SUMS.sig":
			sig = a
			foundSig = true
		}
	}
	if !foundSums || !foundSig {
		return Asset{}, Asset{}, errors.New("release missing SHA256SUMS or SHA256SUMS.sig")
	}
	return sums, sig, nil
}

// PlatformAssetName returns the asset filename convention used by the
// release workflow. Keep in sync with `.github/workflows/release.yaml`.
func PlatformAssetName(goos, goarch string) string {
	if goos == "windows" {
		return fmt.Sprintf("NetCatcher-%s.exe", goarch)
	}
	return fmt.Sprintf("NetCatcher-%s.app.tar.gz", goarch)
}

type githubClient struct {
	repo    string
	http    *http.Client
	baseURL string // override for tests; defaults to https://api.github.com
}

func newGitHubClient(repo string, httpc *http.Client, baseURL string) *githubClient {
	if httpc == nil {
		httpc = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &githubClient{repo: repo, http: httpc, baseURL: baseURL}
}

// fetchLatest hits /repos/<repo>/releases/latest. If etag is non-empty and
// the server replies 304, ErrNotModified is returned. On success the new
// ETag header value is returned alongside the parsed release.
func (c *githubClient) fetchLatest(ctx context.Context, etag string) (*Release, string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.baseURL, c.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil, etag, ErrNotModified
	case http.StatusForbidden:
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return nil, "", errRateLimited
		}
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("github 403: %s", string(body))
	case http.StatusOK:
		// fallthrough
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("github %d: %s", resp.StatusCode, string(body))
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, "", fmt.Errorf("decode release: %w", err)
	}
	return &rel, resp.Header.Get("Etag"), nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./updater/ -v
```

Expected: PASS for all tests (state + verify + github).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum updater/github.go updater/github_test.go
git commit -m "updater: GitHub Releases API client with ETag and rate-limit handling"
```

---

## Task 6: `updater/installer_common.go` — Shared installer helpers

**Files:**
- Create: `updater/installer_common.go`

- [ ] **Step 1: Implement the shared helpers**

Create `updater/installer_common.go`:

```go
package updater

import (
	"path/filepath"
	"runtime"
)

// stagingDir returns the directory where downloads and staged installs
// live. configDir is the same path Config uses, so the updater stays in
// the user's app-support directory rather than scattering temp files.
func stagingDir(configDir string) string {
	return filepath.Join(configDir, "updates")
}

// downloadPath builds the on-disk path for a release asset. The .partial
// suffix is added during download and stripped after verification.
func downloadPath(configDir, version, assetName string) string {
	return filepath.Join(stagingDir(configDir), version, assetName)
}

// CurrentPlatform returns goos/goarch for use in tests and runtime calls.
// Exposed so the orchestrator can pass the same values to GitHub asset
// selection and installer dispatch.
func CurrentPlatform() (goos, goarch string) {
	return runtime.GOOS, runtime.GOARCH
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./updater/
```

Expected: PASS, no output.

- [ ] **Step 3: Commit**

```bash
git add updater/installer_common.go
git commit -m "updater: shared installer path helpers"
```

---

## Task 7: `updater/scripts/` — Embed helper scripts

**Files:**
- Create: `updater/scripts/install-darwin.sh`
- Create: `updater/scripts/install-windows.ps1`
- Create: `updater/scripts/scripts.go`

- [ ] **Step 1: Create the macOS helper**

Create `updater/scripts/install-darwin.sh`:

```bash
#!/usr/bin/env bash
#
# Args: <parent-pid> <staged-app-path> <current-app-path>
# Replaces <current-app-path> with <staged-app-path> after <parent-pid> exits,
# then relaunches the .app via `open`.
#
set -euo pipefail

if [ "$#" -ne 3 ]; then
    echo "usage: $0 <parent-pid> <staged-app> <current-app>" >&2
    exit 2
fi

PARENT_PID="$1"
STAGED="$2"
APP_PATH="$3"

# Wait up to 30s for the parent (NetCatcher) to exit so we can replace its bundle.
for _ in $(seq 1 60); do
    if ! kill -0 "$PARENT_PID" 2>/dev/null; then
        break
    fi
    sleep 0.5
done

if kill -0 "$PARENT_PID" 2>/dev/null; then
    echo "parent $PARENT_PID still running after 30s" >&2
    exit 11
fi

# Swap the bundle.
if [ ! -d "$STAGED" ]; then
    echo "staged bundle missing: $STAGED" >&2
    exit 12
fi
rm -rf "$APP_PATH"
mv "$STAGED" "$APP_PATH"

# Clear quarantine so the relaunched bundle doesn't trip Gatekeeper.
xattr -dr com.apple.quarantine "$APP_PATH" 2>/dev/null || true

# Re-launch.
open -a "$APP_PATH"
```

Make it executable in the working tree (the embed retains mode 0644 in the binary, but the installer chmods it back to 0755 on write):

```bash
chmod +x updater/scripts/install-darwin.sh
```

- [ ] **Step 2: Create the Windows helper**

Create `updater/scripts/install-windows.ps1`:

```powershell
# Args: -ParentPid <int> -StagedExe <path> -TargetExe <path>
# Replaces TargetExe with StagedExe after ParentPid exits, then relaunches.
param(
    [Parameter(Mandatory=$true)][int]$ParentPid,
    [Parameter(Mandatory=$true)][string]$StagedExe,
    [Parameter(Mandatory=$true)][string]$TargetExe
)

$ErrorActionPreference = 'Stop'

# Wait up to 30s for the parent to exit.
for ($i = 0; $i -lt 60; $i++) {
    if (-not (Get-Process -Id $ParentPid -ErrorAction SilentlyContinue)) { break }
    Start-Sleep -Milliseconds 500
}
if (Get-Process -Id $ParentPid -ErrorAction SilentlyContinue) {
    Write-Error "parent $ParentPid still running after 30s"
    exit 11
}

$old = "$TargetExe.old"
if (Test-Path $old) {
    Remove-Item $old -Force -ErrorAction SilentlyContinue
}

# Retry up to 3x for AV / file-lock contention.
$lastErr = $null
for ($i = 0; $i -lt 3; $i++) {
    try {
        Rename-Item -Path $TargetExe -NewName ([System.IO.Path]::GetFileName($old)) -ErrorAction Stop
        Copy-Item -Path $StagedExe -Destination $TargetExe -Force -ErrorAction Stop
        $lastErr = $null
        break
    } catch {
        $lastErr = $_
        Start-Sleep -Seconds 1
    }
}
if ($lastErr) {
    throw $lastErr
}

Start-Process -FilePath $TargetExe
```

- [ ] **Step 3: Create the embed declarations**

Create `updater/scripts/scripts.go`:

```go
// Package scripts exposes the platform helper scripts as embedded bytes.
// They are written to a temp directory at install time and launched
// detached from the main process.
package scripts

import _ "embed"

//go:embed install-darwin.sh
var InstallDarwin []byte

//go:embed install-windows.ps1
var InstallWindows []byte
```

- [ ] **Step 4: Verify build**

```bash
go build ./updater/scripts/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add updater/scripts/
git commit -m "updater: embedded macOS and Windows install helpers"
```

---

## Task 8: `updater/installer_darwin.go` — macOS install flow

**Files:**
- Create: `updater/installer_darwin.go`

- [ ] **Step 1: Implement the darwin installer**

Create `updater/installer_darwin.go`:

```go
//go:build darwin

package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"netcatcher/updater/scripts"
)

// installDarwin extracts the .app.tar.gz into a sibling staging dir,
// writes the install-darwin.sh helper, launches it detached, and returns.
// The caller must invoke wailsApp.Quit() afterwards so the helper can
// swap the bundle.
func installDarwin(ctx context.Context, assetPath string, logger *slog.Logger) error {
	currentApp, err := resolveCurrentAppBundle()
	if err != nil {
		return err
	}

	stagingRoot := filepath.Dir(assetPath)
	stagedApp := filepath.Join(stagingRoot, "NetCatcher.app")
	_ = os.RemoveAll(stagedApp)

	if err := extractTarGz(assetPath, stagingRoot); err != nil {
		return fmt.Errorf("extract %s: %w", assetPath, err)
	}
	if _, err := os.Stat(stagedApp); err != nil {
		return fmt.Errorf("expected NetCatcher.app inside %s: %w", assetPath, err)
	}

	scriptPath := filepath.Join(stagingRoot, "install.sh")
	if err := os.WriteFile(scriptPath, scripts.InstallDarwin, 0o755); err != nil {
		return fmt.Errorf("write helper: %w", err)
	}

	pid := strconv.Itoa(os.Getpid())
	cmd := exec.Command("/bin/bash", scriptPath, pid, stagedApp, currentApp)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	// Detach so the script outlives the parent process.
	cmd.SysProcAttr = detachAttrs()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch helper: %w", err)
	}
	// Release; we deliberately do not Wait().
	_ = cmd.Process.Release()
	logger.Info("updater: helper script launched", "pid", cmd.Process.Pid, "currentApp", currentApp)
	return nil
}

// resolveCurrentAppBundle walks up from the running binary to the .app.
// Returns an error if the binary is not inside a .app bundle (dev runs).
func resolveCurrentAppBundle() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("os.Executable: %w", err)
	}
	// .../NetCatcher.app/Contents/MacOS/NetCatcher
	app := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if !strings.HasSuffix(app, ".app") {
		return "", errors.New("not running from a .app bundle (dev build); refusing to install")
	}
	return app, nil
}

func extractTarGz(archive, destRoot string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destRoot, hdr.Name)
		if !strings.HasPrefix(target, filepath.Clean(destRoot)+string(os.PathSeparator)) {
			return fmt.Errorf("tar path escape: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
	return nil
}
```

Also create `updater/detach_unix.go` to hold the `detachAttrs` syscall flags (so the helper survives the parent's exit):

```go
//go:build darwin

package updater

import "syscall"

func detachAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
```

- [ ] **Step 2: Verify build on darwin**

```bash
GOOS=darwin go build ./updater/
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add updater/installer_darwin.go updater/detach_unix.go
git commit -m "updater: macOS installer with tar.gz extraction and detached helper"
```

---

## Task 9: `updater/installer_windows.go` — Windows install flow

**Files:**
- Create: `updater/installer_windows.go`
- Create: `updater/installer_other.go` (stub for non-supported builds)

- [ ] **Step 1: Implement the windows installer**

Create `updater/installer_windows.go`:

```go
//go:build windows

package updater

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"netcatcher/updater/scripts"
)

func installWindows(ctx context.Context, assetPath string, logger *slog.Logger) error {
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("os.Executable: %w", err)
	}

	stagingRoot := filepath.Dir(assetPath)
	stagedExe := filepath.Join(stagingRoot, "NetCatcher-new.exe")
	if err := copyFile(assetPath, stagedExe); err != nil {
		return fmt.Errorf("stage exe: %w", err)
	}

	scriptPath := filepath.Join(stagingRoot, "install.ps1")
	if err := os.WriteFile(scriptPath, scripts.InstallWindows, 0o644); err != nil {
		return fmt.Errorf("write helper: %w", err)
	}

	pid := strconv.Itoa(os.Getpid())
	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile", "-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
		"-ParentPid", pid,
		"-StagedExe", stagedExe,
		"-TargetExe", currentExe,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP — survive parent exit
		// and don't share its console (we run with -H=windowsgui so there's
		// no console to share anyway).
		CreationFlags: 0x00000008 | 0x00000200,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch helper: %w", err)
	}
	_ = cmd.Process.Release()
	logger.Info("updater: helper script launched", "pid", cmd.Process.Pid, "currentExe", currentExe)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
```

- [ ] **Step 2: Create the unsupported-platform stub**

Create `updater/installer_other.go`:

```go
//go:build !darwin && !windows

package updater

import (
	"context"
	"errors"
	"log/slog"
)

func installDarwin(_ context.Context, _ string, _ *slog.Logger) error {
	return errors.New("darwin installer not available on this build")
}

func installWindows(_ context.Context, _ string, _ *slog.Logger) error {
	return errors.New("windows installer not available on this build")
}
```

- [ ] **Step 3: Verify cross-platform build**

```bash
GOOS=windows GOARCH=amd64 go build ./updater/
GOOS=darwin GOARCH=arm64 go build ./updater/
GOOS=linux GOARCH=amd64 go build ./updater/
```

Expected: all three PASS.

- [ ] **Step 4: Commit**

```bash
git add updater/installer_windows.go updater/installer_other.go
git commit -m "updater: Windows installer with detached PowerShell helper"
```

---

## Task 10: `updater/cleanup.go` — Stale-install cleanup

**Files:**
- Create: `updater/cleanup.go`
- Create: `updater/cleanup_test.go`

- [ ] **Step 1: Write the failing tests**

Create `updater/cleanup_test.go`:

```go
package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupStaleRemovesStagingDirs(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "updates", "1.4.0")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staging, "leftover"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := CleanupStale(dir); err != nil {
		t.Fatalf("CleanupStale: %v", err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("expected staging dir gone, got %v", err)
	}
}

func TestCleanupStaleHandlesMissingDir(t *testing.T) {
	if err := CleanupStale(t.TempDir()); err != nil {
		t.Fatalf("expected no error for empty configDir, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./updater/ -run TestCleanupStale -v
```

Expected: FAIL — `CleanupStale` undefined.

- [ ] **Step 3: Implement `cleanup.go`**

Create `updater/cleanup.go`:

```go
package updater

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CleanupStale removes leftover staging dirs from a previous unfinished
// install. Safe to call on every startup. Does not touch <exe>.old —
// the Windows installer overwrites it on the next install, and removing
// it here would race with a still-running helper.
func CleanupStale(configDir string) error {
	root := stagingDir(configDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read staging dir: %w", err)
	}
	var firstErr error
	for _, e := range entries {
		path := filepath.Join(root, e.Name())
		if err := os.RemoveAll(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./updater/ -v
```

Expected: PASS for all updater tests.

- [ ] **Step 5: Commit**

```bash
git add updater/cleanup.go updater/cleanup_test.go
git commit -m "updater: cleanup leftover staging dirs on startup"
```

---

## Task 11: `updater/updater.go` — Orchestrator

**Files:**
- Create: `updater/updater.go`
- Create: `updater/updater_test.go`

- [ ] **Step 1: Write the failing tests**

Create `updater/updater_test.go`:

```go
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// updaterTestServer mounts a GitHub-shaped /repos/<repo>/releases/latest
// endpoint plus arbitrary asset URLs. The latest release JSON uses the
// server's own base URL so the asset downloads stay self-contained.
func updaterTestServer(t *testing.T, files map[string][]byte, tag string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)

	for name, body := range files {
		body := body
		mux.HandleFunc("/asset/"+name, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
			_, _ = w.Write(body)
		})
	}

	mux.HandleFunc("/repos/attson/netcatcher/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", "etag-test")
		assets := []map[string]any{}
		for name, body := range files {
			assets = append(assets, map[string]any{
				"name":                 name,
				"size":                 len(body),
				"browser_download_url": srv.URL + "/asset/" + name,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag,
			"body":     "test notes",
			"html_url": "https://example.test/" + tag,
			"assets":   assets,
		})
	})

	return srv
}

func sha256hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestUpdaterCheckReportsAvailable(t *testing.T) {
	asset := []byte("fake app archive")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", sha256hex(asset)))
	files := map[string][]byte{
		"NetCatcher-arm64.app.tar.gz": asset,
		"SHA256SUMS":                  sums,
		"SHA256SUMS.sig":              []byte("ignored"),
	}
	srv := updaterTestServer(t, files, "v1.5.0")
	defer srv.Close()

	var (
		mu     sync.Mutex
		events []State
	)
	emit := func(_ string, data any) {
		mu.Lock(); events = append(events, data.(State)); mu.Unlock()
	}

	u, err := New(Options{
		CurrentVersion: "1.4.0",
		Repo:           "attson/netcatcher",
		ConfigDir:      t.TempDir(),
		HTTPClient:     srv.Client(),
		APIBaseURL:     srv.URL,
		Emit:           emit,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Platform:       platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil { t.Fatalf("New: %v", err) }

	if err := u.Check(context.Background(), true); err != nil {
		t.Fatalf("Check: %v", err)
	}
	snap := u.State()
	if snap.Status != StatusAvailable {
		t.Fatalf("status: %q", snap.Status)
	}
	if snap.LatestVersion != "1.5.0" {
		t.Fatalf("latest: %q", snap.LatestVersion)
	}
}

func TestUpdaterCheckReportsUpToDate(t *testing.T) {
	files := map[string][]byte{
		"NetCatcher-arm64.app.tar.gz": []byte("x"),
		"SHA256SUMS":                  []byte("0  x\n"),
		"SHA256SUMS.sig":              []byte(""),
	}
	srv := updaterTestServer(t, files, "v1.4.0")
	defer srv.Close()

	u, err := New(Options{
		CurrentVersion: "1.4.0",
		Repo:           "attson/netcatcher",
		ConfigDir:      t.TempDir(),
		HTTPClient:     srv.Client(),
		APIBaseURL:     srv.URL,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Platform:       platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil { t.Fatalf("New: %v", err) }
	if err := u.Check(context.Background(), true); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if u.State().Status != StatusIdle {
		t.Fatalf("expected idle when up-to-date, got %q", u.State().Status)
	}
}

func TestUpdaterDownloadVerifiesChecksum(t *testing.T) {
	asset := []byte("real bytes")
	good := sha256hex(asset)
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", good))
	files := map[string][]byte{
		"NetCatcher-arm64.app.tar.gz": asset,
		"SHA256SUMS":                  sums,
		"SHA256SUMS.sig":              []byte("ignored"),
	}
	srv := updaterTestServer(t, files, "v1.5.0")
	defer srv.Close()

	cfgDir := t.TempDir()
	u, err := New(Options{
		CurrentVersion: "1.4.0",
		Repo:           "attson/netcatcher",
		ConfigDir:      cfgDir,
		HTTPClient:     srv.Client(),
		APIBaseURL:     srv.URL,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Platform:       platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil { t.Fatalf("New: %v", err) }
	if err := u.Check(context.Background(), true); err != nil { t.Fatalf("Check: %v", err) }
	if err := u.Download(context.Background()); err != nil { t.Fatalf("Download: %v", err) }

	if u.State().Status != StatusReady {
		t.Fatalf("expected ready, got %q (err=%q)", u.State().Status, u.State().Error)
	}
	dl := downloadPath(cfgDir, "1.5.0", "NetCatcher-arm64.app.tar.gz")
	if _, err := os.Stat(dl); err != nil {
		t.Fatalf("expected staged asset at %s, got %v", dl, err)
	}
}

func TestUpdaterSkipHidesAvailable(t *testing.T) {
	asset := []byte("x")
	sums := []byte(fmt.Sprintf("%s  NetCatcher-arm64.app.tar.gz\n", sha256hex(asset)))
	files := map[string][]byte{
		"NetCatcher-arm64.app.tar.gz": asset,
		"SHA256SUMS":                  sums,
		"SHA256SUMS.sig":              []byte("ignored"),
	}
	srv := updaterTestServer(t, files, "v1.5.0")
	defer srv.Close()

	u, err := New(Options{
		CurrentVersion: "1.4.0",
		Repo:           "attson/netcatcher",
		ConfigDir:      t.TempDir(),
		HTTPClient:     srv.Client(),
		APIBaseURL:     srv.URL,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Platform:       platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil { t.Fatalf("New: %v", err) }
	if err := u.Check(context.Background(), true); err != nil { t.Fatalf("Check: %v", err) }
	if err := u.Skip("1.5.0"); err != nil { t.Fatalf("Skip: %v", err) }
	if u.State().SkippedVersion != "1.5.0" {
		t.Fatalf("SkippedVersion: %q", u.State().SkippedVersion)
	}
}

func TestUpdaterRejectsConcurrentChecks(t *testing.T) {
	files := map[string][]byte{
		"NetCatcher-arm64.app.tar.gz": []byte("x"),
		"SHA256SUMS":                  []byte("0  x\n"),
		"SHA256SUMS.sig":              []byte(""),
	}
	srv := updaterTestServer(t, files, "v1.4.0")
	defer srv.Close()

	u, err := New(Options{
		CurrentVersion: "1.4.0",
		Repo:           "attson/netcatcher",
		ConfigDir:      t.TempDir(),
		HTTPClient:     srv.Client(),
		APIBaseURL:     srv.URL,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Platform:       platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil { t.Fatalf("New: %v", err) }

	u.busyForTest(true)
	defer u.busyForTest(false)
	err = u.Check(context.Background(), true)
	if err == nil {
		t.Fatalf("expected concurrent-check error")
	}
}

// Helper for the concurrency test — exported via internal unexported
// receiver method to keep the public surface minimal.
type platform = struct {
	OS   string
	Arch string
}

func TestUpdaterDevBuildShortCircuits(t *testing.T) {
	u, err := New(Options{
		CurrentVersion: "dev",
		Repo:           "attson/netcatcher",
		ConfigDir:      t.TempDir(),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Platform:       platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil { t.Fatalf("New: %v", err) }

	if err := u.Check(context.Background(), true); err == nil {
		t.Fatalf("expected dev-build error")
	}
}

// Verify HTTPClient propagation via a closed transport
var _ = filepath.Join // keep import warm if test trimmed
```

- [ ] **Step 2: Implement the orchestrator**

Create `updater/updater.go`:

```go
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

// Platform identifies the OS/arch for asset selection. Defaulting from
// runtime.GOOS/GOARCH; tests override via Options.Platform.
type Platform struct {
	OS   string
	Arch string
}

// Options configures a new Updater. Required: CurrentVersion, Repo,
// ConfigDir. Everything else has sensible defaults.
type Options struct {
	CurrentVersion string
	PublicKey      string // base64; "" = dev build / unsigned local
	Repo           string // "attson/netcatcher"
	ConfigDir      string

	HTTPClient *http.Client     // injectable for tests
	APIBaseURL string           // override https://api.github.com (tests)
	Platform   Platform         // defaults to runtime.GOOS/GOARCH
	Emit       EmitFunc         // wails Event.Emit
	Logger     *slog.Logger     // never nil after New
	Now        func() time.Time // defaults to time.Now
}

// Updater is the orchestrator. Methods are safe for concurrent calls;
// Check / Download / InstallAndQuit are mutually exclusive via busy flag.
type Updater struct {
	opts   Options
	store  *stateStore
	github *githubClient
	platf  Platform
	logger *slog.Logger
	now    func() time.Time

	busyMu sync.Mutex
	busy   bool

	cacheMu sync.Mutex
	etag    string
	cached  *Release
}

func New(opts Options) (*Updater, error) {
	if opts.CurrentVersion == "" {
		return nil, errors.New("updater: CurrentVersion required")
	}
	if opts.Repo == "" {
		return nil, errors.New("updater: Repo required")
	}
	if opts.ConfigDir == "" {
		return nil, errors.New("updater: ConfigDir required")
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Platform.OS == "" {
		opts.Platform.OS = runtime.GOOS
	}
	if opts.Platform.Arch == "" {
		opts.Platform.Arch = runtime.GOARCH
	}

	store := newStateStore(State{
		Status:         StatusIdle,
		CurrentVersion: opts.CurrentVersion,
	}, opts.Emit)

	return &Updater{
		opts:   opts,
		store:  store,
		github: newGitHubClient(opts.Repo, opts.HTTPClient, opts.APIBaseURL),
		platf:  opts.Platform,
		logger: opts.Logger,
		now:    opts.Now,
	}, nil
}

// State returns a snapshot.
func (u *Updater) State() State { return u.store.Snapshot() }

// Start launches the 5s startup check + 24h ticker. No-op on dev builds.
func (u *Updater) Start(ctx context.Context) {
	if u.isDev() {
		u.logger.Info("updater: dev build, auto-check disabled")
		return
	}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
		_ = u.Check(ctx, false)
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = u.Check(ctx, false)
			}
		}
	}()
}

func (u *Updater) isDev() bool { return u.opts.CurrentVersion == "dev" }

func (u *Updater) acquireBusy() bool {
	u.busyMu.Lock()
	defer u.busyMu.Unlock()
	if u.busy {
		return false
	}
	u.busy = true
	return true
}

func (u *Updater) releaseBusy() {
	u.busyMu.Lock()
	u.busy = false
	u.busyMu.Unlock()
}

// busyForTest is used by tests to simulate a busy updater for the
// concurrent-check rejection path.
func (u *Updater) busyForTest(b bool) {
	u.busyMu.Lock()
	u.busy = b
	u.busyMu.Unlock()
}

// Check polls GitHub. force=true bypasses ETag caching. Returns the
// error encountered; the state's Error field is set in lockstep.
func (u *Updater) Check(ctx context.Context, force bool) error {
	if u.isDev() {
		return errors.New("updates disabled in development build")
	}
	if !u.acquireBusy() {
		return errors.New("update operation already in progress")
	}
	defer u.releaseBusy()

	u.store.transition(func(s *State) {
		s.Status = StatusChecking
		s.Error = ""
	})

	cacheEtag := ""
	if !force {
		u.cacheMu.Lock()
		cacheEtag = u.etag
		u.cacheMu.Unlock()
	}
	rel, etag, err := u.github.fetchLatest(ctx, cacheEtag)
	if errors.Is(err, ErrNotModified) {
		u.cacheMu.Lock()
		rel = u.cached
		u.cacheMu.Unlock()
		err = nil
	}
	if err != nil {
		u.store.transition(func(s *State) {
			s.Status = StatusError
			s.Error = err.Error()
			s.LastCheckedAt = u.now()
		})
		return err
	}
	u.cacheMu.Lock()
	u.etag = etag
	u.cached = rel
	u.cacheMu.Unlock()

	latest := strings.TrimPrefix(rel.TagName, "v")
	cmp := semver.Compare("v"+latest, "v"+u.opts.CurrentVersion)
	u.store.transition(func(s *State) {
		s.LatestVersion = latest
		s.ReleaseNotes = rel.Body
		s.ReleaseURL = rel.HTMLURL
		s.LastCheckedAt = u.now()
		if cmp > 0 {
			s.Status = StatusAvailable
		} else {
			s.Status = StatusIdle
		}
	})
	return nil
}

// Download fetches the asset for the current platform and verifies it.
// On success the state advances to StatusReady; the staged file lives
// at <ConfigDir>/updates/<version>/<asset>.
func (u *Updater) Download(ctx context.Context) error {
	if u.isDev() {
		return errors.New("updates disabled in development build")
	}
	if !u.acquireBusy() {
		return errors.New("update operation already in progress")
	}
	defer u.releaseBusy()

	u.cacheMu.Lock()
	rel := u.cached
	u.cacheMu.Unlock()
	if rel == nil {
		return errors.New("call Check before Download")
	}

	asset, err := rel.AssetForPlatform(u.platf.OS, u.platf.Arch)
	if err != nil {
		u.store.transition(func(s *State) { s.Status = StatusError; s.Error = err.Error() })
		return err
	}
	sumsAsset, sigAsset, err := rel.ChecksumAssets()
	if err != nil {
		u.store.transition(func(s *State) { s.Status = StatusError; s.Error = err.Error() })
		return err
	}

	version := strings.TrimPrefix(rel.TagName, "v")
	dst := downloadPath(u.opts.ConfigDir, version, asset.Name)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		u.store.transition(func(s *State) { s.Status = StatusError; s.Error = err.Error() })
		return err
	}

	u.store.transition(func(s *State) {
		s.Status = StatusDownloading
		s.AssetSize = asset.Size
		s.DownloadPct = 0
		s.Error = ""
	})

	tmp := dst + ".partial"
	sum, err := u.downloadWithProgress(ctx, asset.URL, tmp, asset.Size)
	if err != nil {
		_ = os.Remove(tmp)
		u.store.transition(func(s *State) { s.Status = StatusError; s.Error = err.Error() })
		return err
	}

	sumsBytes, err := u.fetchBytes(ctx, sumsAsset.URL)
	if err != nil {
		_ = os.Remove(tmp)
		u.store.transition(func(s *State) { s.Status = StatusError; s.Error = err.Error() })
		return err
	}
	sigB64, err := u.fetchBytes(ctx, sigAsset.URL)
	if err != nil {
		// Keep the .partial — could be a transient sig fetch failure.
		u.store.transition(func(s *State) { s.Status = StatusError; s.Error = err.Error() })
		return err
	}
	sigBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigB64)))
	if err != nil {
		u.store.transition(func(s *State) { s.Status = StatusError; s.Error = err.Error() })
		return err
	}

	if err := VerifyChecksums(u.opts.PublicKey, sumsBytes, sigBytes, asset.Name, sum); err != nil {
		if errors.Is(err, ErrSignatureFailed) {
			// Keep the file for forensics; do not delete.
			u.store.transition(func(s *State) { s.Status = StatusError; s.Error = err.Error() })
			return err
		}
		_ = os.Remove(tmp)
		u.store.transition(func(s *State) { s.Status = StatusError; s.Error = err.Error() })
		return err
	}

	if err := os.Rename(tmp, dst); err != nil {
		u.store.transition(func(s *State) { s.Status = StatusError; s.Error = err.Error() })
		return err
	}
	u.store.transition(func(s *State) {
		s.Status = StatusReady
		s.DownloadPct = 100
	})
	return nil
}

// InstallAndQuit launches the platform helper. The caller (App) must
// invoke wailsApp.Quit() after this returns nil.
func (u *Updater) InstallAndQuit(ctx context.Context) error {
	if u.isDev() {
		return errors.New("updates disabled in development build")
	}
	if !u.acquireBusy() {
		return errors.New("update operation already in progress")
	}
	defer u.releaseBusy()

	snap := u.store.Snapshot()
	if snap.Status != StatusReady {
		return fmt.Errorf("install requires status=ready, got %q", snap.Status)
	}
	version := snap.LatestVersion
	asset := PlatformAssetName(u.platf.OS, u.platf.Arch)
	staged := downloadPath(u.opts.ConfigDir, version, asset)

	switch u.platf.OS {
	case "darwin":
		return installDarwin(ctx, staged, u.logger)
	case "windows":
		return installWindows(ctx, staged, u.logger)
	default:
		return fmt.Errorf("unsupported platform %s", u.platf.OS)
	}
}

// Skip records the version the user dismissed.
func (u *Updater) Skip(version string) error {
	u.store.transition(func(s *State) {
		s.SkippedVersion = version
	})
	return nil
}

// SetAutoCheck is a thin pass-through; the actual persistence lives in
// the App layer which owns the config file. This exists for symmetry
// with the other binding methods and to allow future refactor.
func (u *Updater) SetAutoCheck(enabled bool) error {
	u.logger.Info("updater: auto-check toggled", "enabled", enabled)
	return nil
}

func (u *Updater) downloadWithProgress(ctx context.Context, url, dst string, expectedSize int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := u.github.http
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	hasher := sha256.New()
	pw := &progressWriter{store: u.store, total: expectedSize}
	mw := io.MultiWriter(out, hasher, pw)
	if _, err := io.Copy(mw, resp.Body); err != nil {
		return nil, err
	}
	return hasher.Sum(nil), nil
}

func (u *Updater) fetchBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.github.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

type progressWriter struct {
	store *stateStore
	read  int64
	total int64
	last  int
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.read += int64(n)
	if p.total > 0 {
		pct := int(p.read * 100 / p.total)
		if pct > 100 {
			pct = 100
		}
		if pct != p.last {
			p.last = pct
			p.store.transition(func(s *State) { s.DownloadPct = pct })
		}
	}
	return n, nil
}
```

- [ ] **Step 3: Resolve test compile mismatches**

The test file in Step 1 uses a local `platform` alias. Adjust the test file at the top to:

```go
type platform = Platform
```

(replacing the existing inline `type platform = struct { ... }` declaration).

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./updater/ -v
```

Expected: PASS for all updater tests (state, verify, github, cleanup, updater).

- [ ] **Step 5: Commit**

```bash
git add updater/updater.go updater/updater_test.go
git commit -m "updater: orchestrator wires GitHub, verify, and platform installers"
```

---

## Task 12: Add binding methods to `App`

**Files:**
- Modify: `app.go`

- [ ] **Step 1: Add the `updater` field and getter helper**

Modify `app.go`. Update the `App` struct to add an `updater` field, and import the new package:

```go
import (
	// ... existing imports ...
	"netcatcher/updater"
)

type App struct {
	ctx        context.Context
	manager    *Manager
	notifier   *Notifier
	notifSvc   *notifications.NotificationService
	configPath string
	logBuf     *logbuffer.Buffer
	app        *application.App
	updater    *updater.Updater
}
```

- [ ] **Step 2: Add the binding methods**

Append the six binding methods to `app.go`:

```go
// SetUpdater is called once from main.go after the updater is constructed.
func (a *App) SetUpdater(u *updater.Updater) { a.updater = u }

func (a *App) GetUpdateState() updater.State {
	if a.updater == nil {
		return updater.State{Status: updater.StatusIdle}
	}
	return a.updater.State()
}

func (a *App) CheckUpdate(force bool) error {
	if a.updater == nil {
		return errors.New("updater not initialised")
	}
	return a.updater.Check(a.ctx, force)
}

func (a *App) StartDownload() error {
	if a.updater == nil {
		return errors.New("updater not initialised")
	}
	return a.updater.Download(a.ctx)
}

func (a *App) InstallAndQuit() error {
	if a.updater == nil {
		return errors.New("updater not initialised")
	}
	if err := a.updater.InstallAndQuit(a.ctx); err != nil {
		return err
	}
	// Give the helper a beat to detach before Wails tears the window down.
	go func() {
		time.Sleep(250 * time.Millisecond)
		if a.app != nil {
			a.app.Quit()
		}
	}()
	return nil
}

func (a *App) SkipVersion(version string) error {
	if a.updater == nil {
		return errors.New("updater not initialised")
	}
	if err := a.updater.Skip(version); err != nil {
		return err
	}
	// Persist into config.
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return err
	}
	cfg.Updater.SkippedVersion = version
	return config.Save(a.configPath, cfg)
}

func (a *App) SetAutoCheck(enabled bool) error {
	if a.updater == nil {
		return errors.New("updater not initialised")
	}
	if err := a.updater.SetAutoCheck(enabled); err != nil {
		return err
	}
	cfg, err := config.Load(a.configPath)
	if err != nil {
		return err
	}
	cfg.Updater.AutoCheck = enabled
	return config.Save(a.configPath, cfg)
}
```

Add `"errors"` to the imports if not present.

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add app.go
git commit -m "app: wire updater bindings (Get/Check/Download/Install/Skip/AutoCheck)"
```

---

## Task 13: Wire updater into `main.go`

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Build the Updater after the App is created**

Insert below the existing `app := NewApp(...)` line in `main.go`:

```go
// Updater setup (no-op on dev builds; see updater.Start docstring).
configDir := filepath.Dir(configPath)
cfg, _ := config.Load(configPath)
logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
upd, err := updater.New(updater.Options{
	CurrentVersion: AppVersion,
	PublicKey:      UpdateVerifyPublicKey,
	Repo:           "attson/netcatcher",
	ConfigDir:      configDir,
	Emit:           wailsApp.Event.Emit,
	Logger:         logger,
})
if err != nil {
	llog.Errorf("updater", "init failed: %v", err)
} else {
	app.SetUpdater(upd)
	// Apply the persisted skipped-version into runtime state so the
	// banner stays hidden across restarts.
	if cfg.Updater.SkippedVersion != "" {
		_ = upd.Skip(cfg.Updater.SkippedVersion)
	}
	if cfg.Updater.AutoCheck {
		go upd.Start(context.Background())
	}
	// Clean up leftover staging dirs from a previous unfinished install.
	if err := updater.CleanupStale(configDir); err != nil {
		llog.Warnf("updater", "cleanup stale: %v", err)
	}
}
```

Add the matching imports:

```go
"context"
"log/slog"
"path/filepath"

"netcatcher/updater"
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

Expected: PASS.

- [ ] **Step 3: Smoke-test the dev build**

```bash
go build -o /tmp/nc-dev .
ls -la /tmp/nc-dev
# Optionally run interactively; close the window after confirming logs.
rm /tmp/nc-dev
```

Expected: build succeeds. No need to run the GUI here — the updater is dev-disabled so there's nothing user-visible to verify yet.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "main: construct and start the updater alongside the manager"
```

---

## Task 14: Add i18n keys

**Files:**
- Modify: `frontend/src/i18n/en.json`
- Modify: `frontend/src/i18n/zh-CN.json`

- [ ] **Step 1: Append the English keys**

Open `frontend/src/i18n/en.json`. Replace the closing `}` block at the end of the file by inserting the new keys immediately before it (each new key on its own line, comma after the previous last key):

```json
  "settings.update.title": "Software Update",
  "settings.update.currentVersion": "Current version",
  "settings.update.latestVersion": "Latest version",
  "settings.update.lastChecked": "Last checked",
  "settings.update.never": "never",
  "settings.update.autoCheck": "Check for updates at startup",
  "settings.update.autoCheckDesc": "Polls GitHub once at launch and every 24 hours.",
  "settings.update.checkNow": "Check now",
  "settings.update.download": "Download",
  "settings.update.installRestart": "Install and restart",
  "settings.update.notes": "Release notes",
  "settings.update.notesNone": "No release notes provided.",
  "settings.update.skipped": "Skipped version {version}",
  "settings.update.unskip": "Cancel skip",
  "settings.update.devDisabled": "Development build — updates disabled.",
  "settings.update.status.idle": "Up to date",
  "settings.update.status.checking": "Checking…",
  "settings.update.status.available": "Update available: {version}",
  "settings.update.status.downloading": "Downloading…",
  "settings.update.status.ready": "Update ready",
  "settings.update.status.error": "Check failed",

  "banner.update.available": "New version {version} available",
  "banner.update.downloading": "Downloading {version}…",
  "banner.update.ready": "{version} downloaded — restart to apply",
  "banner.update.viewNotes": "View notes",
  "banner.update.hideNotes": "Hide notes",
  "banner.update.skip": "Skip this version",
  "banner.update.download": "Download",
  "banner.update.later": "Later",
  "banner.update.installRestart": "Install & restart",
  "banner.update.cancel": "Cancel"
```

- [ ] **Step 2: Append the Chinese keys**

Open `frontend/src/i18n/zh-CN.json` and apply the same shape:

```json
  "settings.update.title": "软件更新",
  "settings.update.currentVersion": "当前版本",
  "settings.update.latestVersion": "最新版本",
  "settings.update.lastChecked": "最近检查",
  "settings.update.never": "从未",
  "settings.update.autoCheck": "启动时自动检查更新",
  "settings.update.autoCheckDesc": "启动时和此后每 24 小时查询一次 GitHub。",
  "settings.update.checkNow": "立即检查",
  "settings.update.download": "下载",
  "settings.update.installRestart": "安装并重启",
  "settings.update.notes": "发布说明",
  "settings.update.notesNone": "没有发布说明。",
  "settings.update.skipped": "已跳过版本 {version}",
  "settings.update.unskip": "取消跳过",
  "settings.update.devDisabled": "开发构建 — 更新已禁用。",
  "settings.update.status.idle": "已是最新",
  "settings.update.status.checking": "正在检查…",
  "settings.update.status.available": "发现新版本：{version}",
  "settings.update.status.downloading": "正在下载…",
  "settings.update.status.ready": "更新已就绪",
  "settings.update.status.error": "检查失败",

  "banner.update.available": "发现新版本 {version}",
  "banner.update.downloading": "正在下载 {version}…",
  "banner.update.ready": "{version} 已下载，重启以应用",
  "banner.update.viewNotes": "查看说明",
  "banner.update.hideNotes": "隐藏说明",
  "banner.update.skip": "跳过此版本",
  "banner.update.download": "下载",
  "banner.update.later": "稍后",
  "banner.update.installRestart": "安装并重启",
  "banner.update.cancel": "取消"
```

- [ ] **Step 3: Validate JSON**

```bash
python3 -c "import json; json.load(open('frontend/src/i18n/en.json')); json.load(open('frontend/src/i18n/zh-CN.json')); print('ok')"
```

Expected: prints `ok`.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/i18n/en.json frontend/src/i18n/zh-CN.json
git commit -m "i18n: add updater banner and settings keys (en, zh-CN)"
```

---

## Task 15: Pinia store `stores/updater.js`

**Files:**
- Create: `frontend/src/stores/updater.js`

- [ ] **Step 1: Implement the store**

Create `frontend/src/stores/updater.js`:

```js
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { Call, Events } from '@wailsio/runtime'

const TERMINAL_BANNER_STATES = new Set(['available', 'downloading', 'ready'])

export const useUpdaterStore = defineStore('updater', () => {
  const state = ref({
    status: 'idle',
    currentVersion: 'dev',
    latestVersion: '',
    releaseNotes: '',
    releaseUrl: '',
    downloadPct: 0,
    assetSize: 0,
    error: '',
    lastCheckedAt: '',
    skippedVersion: '',
  })
  const dismissed = ref(false)
  const initialized = ref(false)

  const showBanner = computed(() => {
    if (dismissed.value) return false
    if (!TERMINAL_BANNER_STATES.has(state.value.status)) return false
    if (state.value.skippedVersion && state.value.skippedVersion === state.value.latestVersion) return false
    return true
  })

  const isDev = computed(() => state.value.currentVersion === 'dev')

  async function init() {
    if (initialized.value) return
    initialized.value = true
    try {
      const snap = await Call.ByName('main.App.GetUpdateState')
      state.value = { ...state.value, ...snap }
    } catch (e) {
      console.error('updater: GetUpdateState failed', e)
    }
    Events.On('update:state-changed', (e) => {
      const data = e.data
      const payload = Array.isArray(data) ? data[0] : data
      if (payload) state.value = { ...state.value, ...payload }
    })
  }

  async function check(force = true) {
    try { await Call.ByName('main.App.CheckUpdate', force) }
    catch (e) { console.error('updater: CheckUpdate failed', e) }
  }
  async function download() {
    try { await Call.ByName('main.App.StartDownload') }
    catch (e) { console.error('updater: StartDownload failed', e) }
  }
  async function installAndQuit() {
    try { await Call.ByName('main.App.InstallAndQuit') }
    catch (e) { console.error('updater: InstallAndQuit failed', e) }
  }
  async function skip(version) {
    try { await Call.ByName('main.App.SkipVersion', version) }
    catch (e) { console.error('updater: SkipVersion failed', e) }
  }
  async function unskip() {
    try { await Call.ByName('main.App.SkipVersion', '') }
    catch (e) { console.error('updater: SkipVersion("") failed', e) }
  }
  async function setAutoCheck(enabled) {
    try { await Call.ByName('main.App.SetAutoCheck', enabled) }
    catch (e) { console.error('updater: SetAutoCheck failed', e) }
  }
  function dismiss() { dismissed.value = true }

  return {
    state,
    dismissed,
    showBanner,
    isDev,
    init,
    check,
    download,
    installAndQuit,
    skip,
    unskip,
    setAutoCheck,
    dismiss,
  }
})
```

- [ ] **Step 2: Verify the frontend still builds**

```bash
cd frontend && npx vite build
```

Expected: build succeeds. (`vite build` is the same command CI uses.)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/stores/updater.js
git commit -m "frontend: Pinia store for updater state and actions"
```

---

## Task 16: `UpdateBanner.vue`

**Files:**
- Create: `frontend/src/components/UpdateBanner.vue`

- [ ] **Step 1: Implement the component**

Create `frontend/src/components/UpdateBanner.vue`:

```vue
<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useUpdaterStore } from '../stores/updater'

const { t } = useI18n()
const updater = useUpdaterStore()
const showNotes = ref(false)

const version = computed(() => updater.state.latestVersion)
const pct = computed(() => Math.max(0, Math.min(100, updater.state.downloadPct || 0)))

async function onDownload() { await updater.download() }
async function onInstall() { await updater.installAndQuit() }
async function onSkip()    { await updater.skip(version.value); updater.dismiss() }
function onLater()         { updater.dismiss() }
function toggleNotes()     { showNotes.value = !showNotes.value }
</script>

<template>
  <div v-if="updater.showBanner" class="update-banner" :data-status="updater.state.status">
    <div class="update-banner-row">
      <span class="update-banner-label">
        <template v-if="updater.state.status === 'available'">
          {{ t('banner.update.available', { version }) }}
        </template>
        <template v-else-if="updater.state.status === 'downloading'">
          {{ t('banner.update.downloading', { version }) }} {{ pct }}%
        </template>
        <template v-else-if="updater.state.status === 'ready'">
          {{ t('banner.update.ready', { version }) }}
        </template>
      </span>

      <span class="update-banner-actions">
        <template v-if="updater.state.status === 'available'">
          <button class="banner-btn" @click="toggleNotes">
            {{ showNotes ? t('banner.update.hideNotes') : t('banner.update.viewNotes') }}
          </button>
          <button class="banner-btn" @click="onSkip">{{ t('banner.update.skip') }}</button>
          <button class="banner-btn banner-btn-primary" @click="onDownload">{{ t('banner.update.download') }}</button>
          <button class="banner-btn" @click="onLater">{{ t('banner.update.later') }}</button>
        </template>
        <template v-else-if="updater.state.status === 'ready'">
          <button class="banner-btn banner-btn-primary" @click="onInstall">{{ t('banner.update.installRestart') }}</button>
          <button class="banner-btn" @click="onLater">{{ t('banner.update.later') }}</button>
        </template>
      </span>
    </div>

    <div v-if="updater.state.status === 'downloading'" class="update-banner-progress">
      <div class="update-banner-progress-fill" :style="{ width: pct + '%' }"></div>
    </div>

    <pre v-if="showNotes && updater.state.releaseNotes" class="update-banner-notes">{{ updater.state.releaseNotes }}</pre>
  </div>
</template>

<style scoped>
.update-banner {
  background: var(--bg-accent-soft, #1f2a3a);
  color: var(--text-primary, #e5e7eb);
  border-bottom: 1px solid var(--border-color, #2d3748);
  font-size: 13px;
}
.update-banner-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 6px 16px;
  min-height: 32px;
}
.update-banner-label { font-weight: 500; }
.update-banner-actions { display: flex; gap: 8px; }
.banner-btn {
  font-size: 12px;
  padding: 4px 10px;
  border-radius: 4px;
  border: 1px solid var(--border-color, #2d3748);
  background: transparent;
  color: inherit;
  cursor: pointer;
}
.banner-btn:hover { background: rgba(255,255,255,0.06); }
.banner-btn-primary {
  background: var(--text-link, #3b82f6);
  border-color: var(--text-link, #3b82f6);
  color: #fff;
}
.update-banner-progress {
  height: 3px;
  background: rgba(255,255,255,0.08);
  overflow: hidden;
}
.update-banner-progress-fill {
  height: 100%;
  background: var(--text-link, #3b82f6);
  transition: width 200ms linear;
}
.update-banner-notes {
  max-height: 200px;
  overflow-y: auto;
  margin: 0;
  padding: 8px 16px;
  background: rgba(0,0,0,0.2);
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 12px;
  white-space: pre-wrap;
}
</style>
```

- [ ] **Step 2: Verify build**

```bash
cd frontend && npx vite build
```

Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/UpdateBanner.vue
git commit -m "frontend: UpdateBanner with available/downloading/ready layouts"
```

---

## Task 17: `SettingsUpdate.vue` + integrate into Settings

**Files:**
- Create: `frontend/src/components/SettingsUpdate.vue`
- Modify: `frontend/src/views/Settings.vue`

- [ ] **Step 1: Implement the section component**

Create `frontend/src/components/SettingsUpdate.vue`:

```vue
<script setup>
import { onMounted, ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Call } from '@wailsio/runtime'
import { useUpdaterStore } from '../stores/updater'

const { t, d } = useI18n()
const updater = useUpdaterStore()
const autoCheck = ref(true)
const showNotes = ref(false)

onMounted(async () => {
  await updater.init()
  try {
    const cfg = await Call.ByName('main.App.GetConfig')
    autoCheck.value = cfg?.updater?.autoCheck !== false
  } catch (e) {
    console.error('SettingsUpdate: GetConfig failed', e)
  }
})

const statusText = computed(() => {
  if (updater.isDev) return t('settings.update.devDisabled')
  const s = updater.state.status
  if (s === 'available') return t('settings.update.status.available', { version: updater.state.latestVersion })
  return t(`settings.update.status.${s}`)
})

const lastCheckedDisplay = computed(() => {
  const v = updater.state.lastCheckedAt
  if (!v || v.startsWith('0001-')) return t('settings.update.never')
  try { return d(new Date(v), 'short') } catch { return v }
})

async function toggleAutoCheck() {
  autoCheck.value = !autoCheck.value
  await updater.setAutoCheck(autoCheck.value)
}
async function onCheck()   { await updater.check(true) }
async function onDownload(){ await updater.download() }
async function onInstall() { await updater.installAndQuit() }
async function onUnskip()  { await updater.unskip() }
</script>

<template>
  <div class="card" style="margin-top: 16px;">
    <h2>{{ t('settings.update.title') }}</h2>

    <div style="padding: 8px 0; font-size: 13px;">
      <div><span style="color: var(--text-secondary);">{{ t('settings.update.currentVersion') }}:</span> {{ updater.state.currentVersion }}</div>
      <div>
        <span style="color: var(--text-secondary);">{{ t('settings.update.latestVersion') }}:</span>
        <span style="margin-left: 4px;">{{ updater.state.latestVersion || '—' }}</span>
        <span style="margin-left: 8px; color: var(--text-secondary);">({{ statusText }})</span>
      </div>
      <div><span style="color: var(--text-secondary);">{{ t('settings.update.lastChecked') }}:</span> {{ lastCheckedDisplay }}</div>
      <div v-if="updater.state.skippedVersion" style="margin-top: 4px;">
        {{ t('settings.update.skipped', { version: updater.state.skippedVersion }) }}
        <button class="banner-btn" style="margin-left: 8px;" @click="onUnskip">{{ t('settings.update.unskip') }}</button>
      </div>
      <div v-if="updater.state.error" style="color: var(--error, #ef4444); margin-top: 4px;">{{ updater.state.error }}</div>
    </div>

    <div style="display: flex; gap: 8px; padding: 8px 0; flex-wrap: wrap;">
      <button class="banner-btn" :disabled="updater.isDev || updater.state.status === 'checking'" @click="onCheck">{{ t('settings.update.checkNow') }}</button>
      <button class="banner-btn" :disabled="updater.state.status !== 'available'" @click="onDownload">{{ t('settings.update.download') }}</button>
      <button class="banner-btn banner-btn-primary" :disabled="updater.state.status !== 'ready'" @click="onInstall">{{ t('settings.update.installRestart') }}</button>
    </div>

    <div style="display: flex; justify-content: space-between; align-items: center; padding: 12px 0; border-top: 1px solid var(--border-color);">
      <div>
        <div style="font-weight: 500;">{{ t('settings.update.autoCheck') }}</div>
        <div style="color: var(--text-secondary); font-size: 13px;">{{ t('settings.update.autoCheckDesc') }}</div>
      </div>
      <div class="toggle" :class="{ active: autoCheck }" @click="toggleAutoCheck"></div>
    </div>

    <div style="padding: 8px 0;">
      <button class="banner-btn" @click="showNotes = !showNotes">
        {{ showNotes ? t('banner.update.hideNotes') : t('settings.update.notes') }}
      </button>
      <pre v-if="showNotes" style="margin-top: 8px; padding: 8px 12px; background: var(--bg-primary); border-radius: var(--radius); font-family: var(--font-mono); font-size: 12px; white-space: pre-wrap; max-height: 240px; overflow-y: auto;">{{ updater.state.releaseNotes || t('settings.update.notesNone') }}</pre>
    </div>
  </div>
</template>

<style scoped>
.banner-btn {
  font-size: 12px;
  padding: 4px 10px;
  border-radius: 4px;
  border: 1px solid var(--border-color);
  background: transparent;
  color: var(--text-primary);
  cursor: pointer;
}
.banner-btn:hover:not(:disabled) { background: rgba(255,255,255,0.06); }
.banner-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.banner-btn-primary:not(:disabled) {
  background: var(--text-link);
  border-color: var(--text-link);
  color: #fff;
}
</style>
```

- [ ] **Step 2: Embed `<SettingsUpdate />` into `Settings.vue`**

Open `frontend/src/views/Settings.vue`. Add the import at the top of `<script setup>`:

```js
import SettingsUpdate from '../components/SettingsUpdate.vue'
```

Insert the component just before the existing About `<div class="card">` (which contains `<h2>{{ $t('settings.about') }}</h2>`):

```vue
    <SettingsUpdate />

    <div class="card" style="margin-top: 16px;">
      <h2>{{ $t('settings.about') }}</h2>
      ...
```

- [ ] **Step 3: Verify build**

```bash
cd frontend && npx vite build
```

Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/SettingsUpdate.vue frontend/src/views/Settings.vue
git commit -m "frontend: Settings section for software update controls"
```

---

## Task 18: Mount `<UpdateBanner />` in `App.vue`

**Files:**
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: Mount the banner**

Open `frontend/src/App.vue`. Add the import and store init:

```vue
<script setup>
import { onMounted } from 'vue'
import { Events } from '@wailsio/runtime'
import { useMonitorStore } from './stores/monitor'
import { useLogStore } from './stores/logs'
import { useUpdaterStore } from './stores/updater'
import UpdateBanner from './components/UpdateBanner.vue'

const monitor = useMonitorStore()
const logStore = useLogStore()
const updater = useUpdaterStore()

onMounted(async () => {
  await monitor.fetchStatus()
  await logStore.fetchRecent()
  await updater.init()

  Events.On('monitor:started', () => { monitor.fetchStatus() })
  Events.On('monitor:stopped', () => { monitor.fetchStatus() })
  Events.On('interface:status-changed', (event) => { monitor.updateInterfaceStatus(event.data) })
})
</script>
```

Wrap the existing `<main class="content">` so the banner sits above it:

```vue
<template>
  <div class="layout">
    <nav class="sidebar">
      <!-- existing sidebar content unchanged -->
    </nav>
    <div class="layout-main">
      <UpdateBanner />
      <main class="content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<style scoped>
.layout-main { display: flex; flex-direction: column; flex: 1; min-width: 0; }
</style>
```

(If `.content` already provides `flex: 1`, the new `.layout-main` simply propagates the same behaviour.)

- [ ] **Step 2: Verify build**

```bash
cd frontend && npx vite build
```

Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/App.vue
git commit -m "frontend: mount UpdateBanner above the main content area"
```

---

## Task 19: `cmd/updater-keygen` — Keypair generator CLI

**Files:**
- Create: `cmd/updater-keygen/main.go`

- [ ] **Step 1: Implement the keygen tool**

Create `cmd/updater-keygen/main.go`:

```go
// updater-keygen generates an Ed25519 keypair for the software update
// signing pipeline. Run it once locally and store the printed values in
// the project's GitHub Secrets:
//
//   NETCATCHER_UPDATE_VERIFY_PUBLIC_KEY   = base64 of the 32-byte public key
//   NETCATCHER_UPDATE_SIGNING_PRIVATE_KEY = base64 of the 64-byte private key
//
// Never commit the private key.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("=== NetCatcher updater signing keypair ===")
	fmt.Println()
	fmt.Println("PUBLIC KEY (set as GitHub Secret NETCATCHER_UPDATE_VERIFY_PUBLIC_KEY):")
	fmt.Println(base64.StdEncoding.EncodeToString(pub))
	fmt.Println()
	fmt.Println("PRIVATE KEY (set as GitHub Secret NETCATCHER_UPDATE_SIGNING_PRIVATE_KEY):")
	fmt.Println(base64.StdEncoding.EncodeToString(priv))
	fmt.Println()
	fmt.Println("Store the PRIVATE KEY in a password manager. Do NOT commit it.")
}
```

- [ ] **Step 2: Build and dry-run**

```bash
go build -o /tmp/keygen ./cmd/updater-keygen
/tmp/keygen | head -10
rm /tmp/keygen
```

Expected: prints both keys (do not record the output; this is a smoke test).

- [ ] **Step 3: Commit**

```bash
git add cmd/updater-keygen/main.go
git commit -m "cmd/updater-keygen: Ed25519 keypair generator for release signing"
```

---

## Task 20: `.github/scripts/sign-checksums.go` — CI signing helper

**Files:**
- Create: `.github/scripts/sign-checksums.go`

- [ ] **Step 1: Implement the signer**

Create `.github/scripts/sign-checksums.go`:

```go
// sign-checksums signs SHA256SUMS with the Ed25519 private key supplied
// in the env var UPDATE_SIGNING_PRIVATE_KEY (base64 of the 64-byte raw
// key). It writes a single line containing base64(signature) to the
// output path.
//
// Usage: go run .github/scripts/sign-checksums.go <sums-file> <sig-out>
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: sign-checksums <sums-file> <sig-out>")
		os.Exit(2)
	}
	keyB64 := os.Getenv("UPDATE_SIGNING_PRIVATE_KEY")
	if keyB64 == "" {
		fmt.Fprintln(os.Stderr, "UPDATE_SIGNING_PRIVATE_KEY env var is required")
		os.Exit(2)
	}
	rawKey, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode key: %v\n", err)
		os.Exit(1)
	}
	if len(rawKey) != ed25519.PrivateKeySize {
		fmt.Fprintf(os.Stderr, "private key size %d, want %d\n", len(rawKey), ed25519.PrivateKeySize)
		os.Exit(1)
	}

	sums, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read sums: %v\n", err)
		os.Exit(1)
	}
	sig := ed25519.Sign(ed25519.PrivateKey(rawKey), sums)
	out := base64.StdEncoding.EncodeToString(sig) + "\n"
	if err := os.WriteFile(os.Args[2], []byte(out), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write sig: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("signed %s (%d bytes) -> %s\n", os.Args[1], len(sums), os.Args[2])
}
```

- [ ] **Step 2: Smoke-test against a temp file**

```bash
# Generate a throwaway keypair, sign a temp sums file, then verify the
# pair using the same private key via inline Go.
go run cmd/updater-keygen/main.go > /tmp/keypair.txt
PRIV=$(grep -A1 "PRIVATE KEY" /tmp/keypair.txt | tail -1)
PUB=$(grep -A1 "PUBLIC KEY" /tmp/keypair.txt | tail -1)
echo "deadbeef  example.txt" > /tmp/SHA256SUMS
UPDATE_SIGNING_PRIVATE_KEY="$PRIV" go run .github/scripts/sign-checksums.go /tmp/SHA256SUMS /tmp/SHA256SUMS.sig
cat /tmp/SHA256SUMS.sig | head -c 20 && echo
rm /tmp/keypair.txt /tmp/SHA256SUMS /tmp/SHA256SUMS.sig
```

Expected: prints `signed /tmp/SHA256SUMS …`, and the cat shows a base64 prefix.

- [ ] **Step 3: Commit**

```bash
git add .github/scripts/sign-checksums.go
git commit -m "ci: Ed25519 signer for SHA256SUMS"
```

---

## Task 21: Update release workflow

**Files:**
- Modify: `.github/workflows/release.yaml`

- [ ] **Step 1: Plumb the public key into builds and add `.app.tar.gz` packaging**

Edit `.github/workflows/release.yaml`. In the `build` job, extend the existing `env:` block (around line 24-26) with the verify key:

```yaml
    env:
      GOARCH: ${{ matrix.goarch }}
      CGO_ENABLED: 1
      UPDATE_VERIFY_PUBLIC_KEY: ${{ secrets.NETCATCHER_UPDATE_VERIFY_PUBLIC_KEY }}
```

Replace the existing **Build Go binary** step (around lines 75-83) with this version that embeds both ldflags variables:

```yaml
      - name: Build Go binary
        shell: bash
        env:
          APP_VERSION: ${{ steps.ver.outputs.version }}
        run: |
          mkdir -p build/bin
          LD="-s -w -X main.AppVersion=${APP_VERSION} -X main.UpdateVerifyPublicKey=${UPDATE_VERIFY_PUBLIC_KEY}"
          if [ "${{ matrix.platform }}" = "windows" ]; then
            LD="$LD -H=windowsgui"
            go build -ldflags="$LD" -o "build/bin/netcatcher.exe" .
          else
            go build -ldflags="$LD" -o "build/bin/netcatcher" .
          fi
```

After the existing **Package macOS .dmg** step (ends around line 152), add the tarball step:

```yaml
      - name: Package macOS .app.tar.gz
        if: matrix.platform == 'darwin'
        shell: bash
        run: |
          # The .dmg step deletes build/NetCatcher.app after creating the DMG.
          # Rebuild a tar.gz directly from the freshly extracted .app inside the
          # DMG so the updater can fetch a structure-identical archive.
          DMG="build/bin/NetCatcher-${{ matrix.goarch }}.dmg"
          MOUNT=$(mktemp -d)
          hdiutil attach -nobrowse -readonly -mountpoint "$MOUNT" "$DMG"
          STAGE=$(mktemp -d)
          cp -R "$MOUNT/NetCatcher.app" "$STAGE/"
          hdiutil detach "$MOUNT"
          tar -czf "build/bin/NetCatcher-${{ matrix.goarch }}.app.tar.gz" -C "$STAGE" NetCatcher.app
          rm -rf "$STAGE"
```

- [ ] **Step 2: Add a `sign-checksums` job**

Insert the following job between the existing `build` and `release` jobs:

```yaml
  sign-checksums:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: Download all build artifacts
        uses: actions/download-artifact@v4
        with:
          merge-multiple: true
          path: artifacts

      - name: List artifacts
        run: find artifacts -type f

      - name: Generate SHA256SUMS
        working-directory: artifacts
        run: |
          : > ../SHA256SUMS
          for f in $(find . -maxdepth 1 -type f \( -name "*.dmg" -o -name "*.exe" -o -name "*.app.tar.gz" \) | sort); do
              # Strip leading ./ so the file references the bare basename.
              base=$(basename "$f")
              sum=$(sha256sum "$f" | awk '{print $1}')
              echo "$sum  $base" >> ../SHA256SUMS
          done
          cat ../SHA256SUMS

      - name: Sign SHA256SUMS
        env:
          UPDATE_SIGNING_PRIVATE_KEY: ${{ secrets.NETCATCHER_UPDATE_SIGNING_PRIVATE_KEY }}
        run: go run ./.github/scripts/sign-checksums.go SHA256SUMS SHA256SUMS.sig

      - name: Upload checksums
        uses: actions/upload-artifact@v4
        with:
          name: checksums
          path: |
            SHA256SUMS
            SHA256SUMS.sig
```

- [ ] **Step 3: Update the `release` job to depend on the new sign job**

In the `release` job, change `needs: build` to `needs: [build, sign-checksums]`:

```yaml
  release:
    needs: [build, sign-checksums]
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - name: Download all artifacts
        uses: actions/download-artifact@v4
        with:
          merge-multiple: true
          path: artifacts

      - name: List artifacts
        run: find artifacts -type f

      - name: Create Release
        uses: softprops/action-gh-release@v2
        with:
          files: artifacts/*
          generate_release_notes: true
```

(`softprops/action-gh-release` globs `artifacts/*`, which now includes `SHA256SUMS` and `SHA256SUMS.sig` via the new artifact.)

- [ ] **Step 4: Lint the workflow file**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yaml')); print('ok')"
```

Expected: prints `ok`.

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/release.yaml
git commit -m "ci: embed updater public key, produce .app.tar.gz, sign SHA256SUMS"
```

---

## Task 22: README + manual verification checklist

**Files:**
- Modify: `README.md`
- Modify: `README.zh-CN.md`

- [ ] **Step 1: Add an `Auto-update` section to `README.md`**

Append to `README.md`, before the existing license / contributing footer (place near the build / release section if one exists):

```markdown
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
```

Mirror the same section into `README.zh-CN.md` translated to Chinese.

- [ ] **Step 2: Commit**

```bash
git add README.md README.zh-CN.md
git commit -m "docs: add auto-update section to READMEs"
```

- [ ] **Step 3: Manual verification checklist (record results in PR)**

Run through these locally before opening the PR. Each item is "passes" / "fails: why":

1. `go test ./... -v` — all tests green.
2. `go build ./...` — clean build, no warnings.
3. `GOOS=windows GOARCH=amd64 go build ./...` — cross-build succeeds.
4. `cd frontend && npx vite build` — frontend builds clean.
5. Open the app locally (dev build). Settings → Software Update shows
   "Development build — updates disabled" and the buttons are disabled.
   Banner does not appear.
6. Build a release-mode binary with a test key:
   ```bash
   go run ./cmd/updater-keygen > /tmp/kp.txt
   PUB=$(grep -A1 "PUBLIC KEY" /tmp/kp.txt | tail -1)
   PRIV=$(grep -A1 "PRIVATE KEY" /tmp/kp.txt | tail -1)
   APP_VERSION=1.0.0 UPDATE_VERIFY_PUBLIC_KEY="$PUB" \
     go build -ldflags="-X main.AppVersion=1.0.0 -X main.UpdateVerifyPublicKey=$PUB" \
     -o /tmp/nc-old .
   ```
   (Skip the local install/relaunch flow on CI runners — only run it on
   your dev machine where you can observe the relaunch.)
7. Confirm Settings → Software Update shows version `1.0.0` and `Check now` actually contacts GitHub. Errors are surfaced inline.

---

## Self-Review

**Spec coverage:**
- §3 Architecture → Tasks 3-11, 12-13 (Go layers wired).
- §4 Go interfaces → Tasks 3 (State), 4 (Verify), 5 (GitHub), 8-9 (Installer), 11 (Orchestrator), 12 (App bindings).
- §5 Frontend → Tasks 14 (i18n), 15 (store), 16 (banner), 17 (settings section), 18 (App.vue mount).
- §6 Configuration → Task 1.
- §7 Signing & CI → Tasks 19 (keygen), 20 (signer), 21 (workflow).
- §8 Platform scripts → Task 7.
- §9 Error handling → covered structurally by the orchestrator's state transitions in Task 11 (test cases exercise rate-limit, checksum mismatch, sig failure, asset missing); the helper-script failure-recovery path is covered by Task 10's `CleanupStale`.
- §10 Testing → unit tests in Tasks 1, 3, 4, 5, 10, 11; manual verification list in Task 22.

**Placeholder scan:** No "TBD"/"TODO"/"fill in" strings remain. Tests show real code; commands show real flags.

**Type consistency check:**
- `State` field names match exactly between `state.go`, the Pinia store JSON shape, and the test fixtures.
- `Options` field names (`CurrentVersion`, `PublicKey`, `Repo`, `ConfigDir`, `HTTPClient`, `APIBaseURL`, `Platform`, `Emit`, `Logger`, `Now`) are stable across `updater.go`, the test file, and `main.go`.
- Binding method names (`GetUpdateState`, `CheckUpdate`, `StartDownload`, `InstallAndQuit`, `SkipVersion`, `SetAutoCheck`) match between `app.go`, the Pinia store calls, and the spec.
- Asset filename convention (`NetCatcher-<arch>.app.tar.gz`, `NetCatcher-<arch>.exe`) matches between CI (Task 21), `PlatformAssetName` (Task 5), and tests.
- `Platform` struct vs `platform` alias in tests: Task 11 step 3 explicitly aligns them.

No gaps found that require revision.
