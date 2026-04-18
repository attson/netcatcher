package netcatcher

import (
	"context"
	"testing"
	"time"

	"netcatcher/config"
)

func TestNewNetCatcher(t *testing.T) {
	cfg := config.Interface{Name: "lo0", Routes: []string{}}
	nc := NewNetCatcher(cfg, nil)
	if nc == nil {
		t.Fatal("expected non-nil NetCatcher")
	}
	status := nc.GetStatus()
	if status.InterfaceName != "lo0" {
		t.Fatalf("expected lo0, got %s", status.InterfaceName)
	}
	if status.Connected {
		t.Fatal("expected disconnected initially")
	}
}

func TestWatchCancellation(t *testing.T) {
	cfg := config.Interface{Name: "nonexistent_iface_xyz", Routes: []string{}}
	nc := NewNetCatcher(cfg, nil)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		nc.Watch(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Watch returned after cancel — correct
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not return after context cancellation")
	}
}
