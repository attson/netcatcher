package main

import (
	"netcatcher/config"
	"testing"
	"time"
)

func TestManagerStartStop(t *testing.T) {
	cfg := config.Config{
		Interfaces: []config.Interface{
			{Name: "nonexistent_iface_test", Routes: []string{}},
		},
	}
	m := NewManager(cfg, nil)

	if m.IsRunning() {
		t.Fatal("expected not running initially")
	}

	m.Start()
	time.Sleep(100 * time.Millisecond)

	if !m.IsRunning() {
		t.Fatal("expected running after Start")
	}

	statuses := m.GetAllStatus()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].InterfaceName != "nonexistent_iface_test" {
		t.Fatalf("expected nonexistent_iface_test, got %s", statuses[0].InterfaceName)
	}

	m.Stop()
	time.Sleep(100 * time.Millisecond)

	if m.IsRunning() {
		t.Fatal("expected not running after Stop")
	}
}

func TestManagerDoubleStart(t *testing.T) {
	cfg := config.Config{
		Interfaces: []config.Interface{},
	}
	m := NewManager(cfg, nil)
	m.Start()
	m.Start() // should not panic
	m.Stop()
}
