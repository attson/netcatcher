package main

import (
	"context"
	"embed"
	_ "embed"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"netcatcher/config"
	"netcatcher/llog"
	"netcatcher/updater"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

//go:embed frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

// Populated at build time via -ldflags="-X main.AppVersion=… -X main.UpdateVerifyPublicKey=…".
// Empty UpdateVerifyPublicKey means a development build: the updater skips
// Ed25519 verification (SHA256 is still enforced) and disables the 24h loop.
var (
	AppVersion            = "dev"
	UpdateVerifyPublicKey = ""
)

func main() {
	configPath := config.DefaultConfigPath()

	wailsApp := application.New(application.Options{
		Name:        "NetCatcher",
		Description: "Network route manager",
		Icon:        appIcon,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})

	var notifSvc *notifications.NotificationService
	if shouldRegisterNotifications() {
		notifSvc = notifications.New()
		wailsApp.RegisterService(application.NewService(notifSvc))
	} else {
		llog.Infof("notify", "system notifications unavailable (run from .app bundle on macOS)")
	}

	app := NewApp(configPath, wailsApp, notifSvc)

	// Register App as a service so its exported methods are available to the frontend.
	wailsApp.RegisterService(application.NewService(app))

	// Updater setup (no-op on dev builds; see updater.Start docstring).
	configDir := filepath.Dir(configPath)
	cfg, _ := config.Load(configPath)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	upd, err := updater.New(updater.Options{
		CurrentVersion: AppVersion,
		PublicKey:      UpdateVerifyPublicKey,
		Repo:           "attson/netcatcher",
		ConfigDir:      configDir,
		Emit:           func(event string, data any) { wailsApp.Event.Emit(event, data) },
		Logger:         logger,
	})
	if err != nil {
		llog.Errorf("updater", "init failed: %v", err)
	} else {
		app.SetUpdater(upd)
		// Apply the persisted skipped-version into runtime state so the
		// banner stays hidden across restarts.
		if cfg.Updater.SkippedVersion != "" {
			_ = upd.Skip(cfg.Updater.SkippedVersion)
		}
		if cfg.Updater.AutoCheck {
			go upd.Start(context.Background())
		}
		// Clean up leftover staging dirs from a previous unfinished install.
		if err := updater.CleanupStale(configDir); err != nil {
			llog.Warnf("updater", "cleanup stale: %v", err)
		}
	}

	// Create main window
	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "NetCatcher",
		Width:            900,
		Height:           600,
		MinWidth:         700,
		MinHeight:        450,
		Frameless:        false,
		URL:              "/",
		BackgroundColour: application.NewRGBA(13, 17, 23, 255),
	})

	// Hide window instead of closing (intercept close via hook).
	mainWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		mainWindow.Hide()
	})

	// System tray
	trayMenu := wailsApp.Menu.New()
	trayMenu.Add("Show Window").OnClick(func(ctx *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	trayMenu.AddSeparator()
	trayMenu.Add("Quit").OnClick(func(ctx *application.Context) {
		wailsApp.Quit()
	})

	systray := wailsApp.SystemTray.New()
	systray.SetIcon(appIcon)
	systray.SetMenu(trayMenu)


	// Register shutdown hook so monitoring stops cleanly.
	wailsApp.OnShutdown(func() {
		app.OnShutdown()
	})

	// Start monitoring on launch.
	app.OnStartup(nil)

	if err := wailsApp.Run(); err != nil {
		llog.Errorf("app", "wails run failed: %v", err)
		os.Exit(1)
	}
}

func shouldRegisterNotifications() bool {
	if runtime.GOOS == "darwin" {
		exe, err := os.Executable()
		if err != nil {
			return false
		}
		return strings.Contains(exe, ".app/Contents/MacOS/")
	}
	return true
}
