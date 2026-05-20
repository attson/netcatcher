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
