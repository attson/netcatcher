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
	var sigBytes []byte
	if u.opts.PublicKey != "" {
		sigB64, err := u.fetchBytes(ctx, sigAsset.URL)
		if err != nil {
			// Keep the .partial — could be a transient sig fetch failure.
			u.store.transition(func(s *State) { s.Status = StatusError; s.Error = err.Error() })
			return err
		}
		sigBytes, err = base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigB64)))
		if err != nil {
			u.store.transition(func(s *State) { s.Status = StatusError; s.Error = err.Error() })
			return err
		}
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
