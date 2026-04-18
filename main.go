package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "NetCatcher",
		Description: "Network route manager",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "NetCatcher",
		Width:     900,
		Height:    600,
		MinWidth:  700,
		MinHeight: 450,
		Frameless: true,
		URL:       "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
