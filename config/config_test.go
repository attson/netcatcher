package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load non-existent should not error, got: %v", err)
	}
	if len(cfg.Interfaces) != 0 {
		t.Fatalf("expected empty interfaces, got %d", len(cfg.Interfaces))
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	cfg := Config{
		Interfaces: []Interface{
			{
				Name:        "ppp0",
				Routes:      []string{"github.com", "192.168.1.0/24"},
				IPv4Gateway: "10.0.0.1",
				IPv6Gateway: "fe80::1%ppp0",
			},
		},
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(loaded.Interfaces))
	}
	if loaded.Interfaces[0].Name != "ppp0" {
		t.Fatalf("expected ppp0, got %s", loaded.Interfaces[0].Name)
	}
	if len(loaded.Interfaces[0].Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(loaded.Interfaces[0].Routes))
	}
	if loaded.Interfaces[0].IPv4Gateway != "10.0.0.1" {
		t.Fatalf("expected IPv4 gateway to survive round-trip, got %q", loaded.Interfaces[0].IPv4Gateway)
	}
	if loaded.Interfaces[0].IPv6Gateway != "fe80::1%ppp0" {
		t.Fatalf("expected IPv6 gateway to survive round-trip, got %q", loaded.Interfaces[0].IPv6Gateway)
	}
}

func TestValidateGateways(t *testing.T) {
	tests := []struct {
		name    string
		iface   Interface
		wantErr bool
	}{
		{name: "auto", iface: Interface{Name: "en0"}},
		{name: "IPv4", iface: Interface{Name: "en0", IPv4Gateway: "192.168.1.1"}},
		{name: "IPv6", iface: Interface{Name: "en0", IPv6Gateway: "2001:db8::1"}},
		{name: "scoped IPv6", iface: Interface{Name: "en0", IPv6Gateway: "fe80::1%en0"}},
		{name: "wrong IPv4 family", iface: Interface{Name: "en0", IPv4Gateway: "fe80::1"}, wantErr: true},
		{name: "wrong IPv6 family", iface: Interface{Name: "en0", IPv6Gateway: "192.168.1.1"}, wantErr: true},
		{name: "numeric IPv6 scope", iface: Interface{Name: "Ethernet", IPv6Gateway: "fe80::1%12"}},
		{name: "empty IPv6 scope", iface: Interface{Name: "en0", IPv6Gateway: "fe80::1%"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := (Config{Interfaces: []Interface{test.iface}}).Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestLoadRejectsInvalidGateway(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"interfaces":[{"name":"en0","routes":[],"ipv4Gateway":"fe80::1"}]}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid gateway to be rejected")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Fatal("DefaultConfigPath returned empty string")
	}
	dir := filepath.Dir(path)
	if dir == "" {
		t.Fatal("config directory is empty")
	}
}

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
	if !loaded.Updater.AutoCheck {
		t.Fatalf("expected AutoCheck=true to survive round-trip")
	}
}
