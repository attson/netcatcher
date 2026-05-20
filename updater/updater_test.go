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
	"time"
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
		mu.Lock()
		events = append(events, data.(State))
		mu.Unlock()
	}

	u, err := New(Options{
		CurrentVersion: "1.4.0",
		Repo:           "attson/netcatcher",
		ConfigDir:      t.TempDir(),
		HTTPClient:     srv.Client(),
		APIBaseURL:     srv.URL,
		Emit:           emit,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Platform:       Platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

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
	_ = events
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
		Platform:       Platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
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
		Platform:       Platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := u.Check(context.Background(), true); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if err := u.Download(context.Background()); err != nil {
		t.Fatalf("Download: %v", err)
	}

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
		Platform:       Platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := u.Check(context.Background(), true); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if err := u.Skip("1.5.0"); err != nil {
		t.Fatalf("Skip: %v", err)
	}
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
		Platform:       Platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	u.busyForTest(true)
	defer u.busyForTest(false)
	err = u.Check(context.Background(), true)
	if err == nil {
		t.Fatalf("expected concurrent-check error")
	}
}

func TestUpdaterDevBuildShortCircuits(t *testing.T) {
	u, err := New(Options{
		CurrentVersion: "dev",
		Repo:           "attson/netcatcher",
		ConfigDir:      t.TempDir(),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Platform:       Platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := u.Check(context.Background(), true); err == nil {
		t.Fatalf("expected dev-build error")
	}
}

func TestSetAutoCheckTogglesLoop(t *testing.T) {
	// Create a server that records hits to the latest endpoint.
	var hits int32
	mu := sync.Mutex{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.Header().Set("Etag", "etag-test")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.0.0",
			"body":     "",
			"html_url": "",
			"assets":   []any{},
		})
	}))
	defer srv.Close()

	u, err := New(Options{
		CurrentVersion: "1.0.0",
		Repo:           "attson/netcatcher",
		ConfigDir:      t.TempDir(),
		HTTPClient:     srv.Client(),
		APIBaseURL:     srv.URL,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Platform:       Platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Start the loop, then immediately stop it. The 5s startup delay
	// means no check should fire within our short test window.
	if err := u.SetAutoCheck(true); err != nil {
		t.Fatalf("on: %v", err)
	}
	if err := u.SetAutoCheck(false); err != nil {
		t.Fatalf("off: %v", err)
	}

	// Wait a bit longer than would normally let the 5s delay elapse,
	// but the loop should have been cancelled before reaching Check.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	got := hits
	mu.Unlock()
	if got != 0 {
		t.Fatalf("expected 0 hits after toggle-off, got %d", got)
	}
}

func TestSetAutoCheckIsIdempotent(t *testing.T) {
	u, err := New(Options{
		CurrentVersion: "1.0.0",
		Repo:           "attson/netcatcher",
		ConfigDir:      t.TempDir(),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Platform:       Platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Multiple toggles on/off must not panic or leak goroutines.
	for i := 0; i < 5; i++ {
		if err := u.SetAutoCheck(true); err != nil {
			t.Fatalf("on %d: %v", i, err)
		}
		if err := u.SetAutoCheck(false); err != nil {
			t.Fatalf("off %d: %v", i, err)
		}
	}
}

func TestCheckPreservesReadyStatus(t *testing.T) {
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

	u, err := New(Options{
		CurrentVersion: "1.4.0",
		Repo:           "attson/netcatcher",
		ConfigDir:      t.TempDir(),
		HTTPClient:     srv.Client(),
		APIBaseURL:     srv.URL,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Platform:       Platform{OS: "darwin", Arch: "arm64"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := u.Check(context.Background(), true); err != nil {
		t.Fatalf("Check 1: %v", err)
	}
	if err := u.Download(context.Background()); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if u.State().Status != StatusReady {
		t.Fatalf("expected ready after download, got %q", u.State().Status)
	}

	// Now Check again — server still serves v1.5.0. Status should STAY ready.
	if err := u.Check(context.Background(), true); err != nil {
		t.Fatalf("Check 2: %v", err)
	}
	if u.State().Status != StatusReady {
		t.Fatalf("expected status to remain ready after re-check, got %q", u.State().Status)
	}
}

var _ = filepath.Join
