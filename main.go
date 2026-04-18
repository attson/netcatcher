package main

import (
	"embed"
	_ "embed"
	"log"
	"os"
	"runtime"
	"strings"

	"netcatcher/config"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

//go:embed frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

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
		log.Printf("[info] system notifications unavailable (run from .app bundle on macOS)")
	}

	app := NewApp(configPath, wailsApp, notifSvc)

	// Register App as a service so its exported methods are available to the frontend.
	wailsApp.RegisterService(application.NewService(app))

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
		log.Fatal(err)
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
