package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type Interface struct {
	Name   string   `json:"name"`
	Routes []string `json:"routes"`
	DNS    []string `json:"dns,omitempty"`
}

type Config struct {
	Interfaces []Interface `json:"interfaces"`
	// TunMode enables a local DNS forwarder + /etc/resolver entries so that
	// domain routes resolve correctly when the host uses a TUN-mode proxy
	// (Clash / Mihomo / Surge). Leave off in plain setups.
	TunMode bool `json:"tunMode,omitempty"`
}

func DefaultConfigPath() string {
	var dir string
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Library", "Application Support", "NetCatcher")
	case "windows":
		dir = filepath.Join(os.Getenv("APPDATA"), "NetCatcher")
	default:
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config", "netcatcher")
	}
	return filepath.Join(dir, "config.json")
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{Interfaces: []Interface{}}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Interfaces == nil {
		cfg.Interfaces = []Interface{}
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
