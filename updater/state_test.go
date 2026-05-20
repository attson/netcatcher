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
