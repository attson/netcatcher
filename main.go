package main

import (
	"embed"
	_ "embed"
	"log"

	"netcatcher/config"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
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

	app := NewApp(configPath, wailsApp)

	// Register App as a service so its exported methods are available to the frontend.
	wailsApp.RegisterService(application.NewService(app))

	// Create main window
	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "NetCatcher",
		Width:            900,
		Height:           600,
		MinWidth:         700,
		MinHeight:        450,
		Frameless:        true,
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
	systray.SetLabel("NetCatcher")

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
