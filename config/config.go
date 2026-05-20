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

type UpdaterConfig struct {
	// AutoCheck controls whether the updater polls GitHub on startup and
	// every 24h. Defaults to true on legacy configs that lack the field.
	AutoCheck bool `json:"autoCheck"`
	// SkippedVersion is the latest version the user explicitly dismissed.
	// Banner stays hidden as long as the latest release equals this string.
	SkippedVersion string `json:"skippedVersion,omitempty"`
}

type Config struct {
	Interfaces []Interface `json:"interfaces"`
	// TunMode enables a local DNS forwarder + /etc/resolver entries so that
	// domain routes resolve correctly when the host uses a TUN-mode proxy
	// (Clash / Mihomo / Surge). Leave off in plain setups.
	TunMode bool          `json:"tunMode,omitempty"`
	// Updater holds user preferences for the in-app auto-update flow.
	// Always serialized: omitempty does not apply to struct values, so
	// once a config is saved, the `updater` key is present even if zero.
	Updater UpdaterConfig `json:"updater"`
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
