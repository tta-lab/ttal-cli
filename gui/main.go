package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	dbPath := filepath.Join(home, ".ttal", "messages.db")

	mcfg, err := config.LoadAll()
	if err != nil {
		log.Printf("warn: could not load daemon config: %v", err)
		mcfg = nil
	}

	svc, err := NewChatService(dbPath, "", mcfg)
	if err != nil {
		log.Fatal(err)
	}
	defer svc.Close()

	app := application.New(application.Options{
		Name:        "ttal Chat",
		Description: "ttal agent chat interface",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "ttal Chat",
		Width:            1200,
		Height:           800,
		MinWidth:         800,
		MinHeight:        600,
		BackgroundColour: application.NewRGB(27, 38, 54),
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
