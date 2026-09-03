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

func TestHasDomainRoutes(t *testing.T) {
	tests := []struct {
		name   string
		routes []string
		want   bool
	}{
		{name: "empty", routes: nil, want: false},
		{name: "only IP and CIDR", routes: []string{"192.168.1.10", "10.0.0.0/8"}, want: false},
		{name: "domain", routes: []string{"192.168.1.10", "git.example.com"}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			n := NewNetCatcher(config.Interface{Name: "test", Routes: test.routes}, nil)
			if got := n.hasDomainRoutes(); got != test.want {
				t.Fatalf("hasDomainRoutes() = %v, want %v", got, test.want)
			}
		})
	}
}
