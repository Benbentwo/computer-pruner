package main

import (
	"context"
	"embed"
	"diskanalyzer/internal/fileops"
	"diskanalyzer/internal/preferences"
	"diskanalyzer/internal/scanner"
	"diskanalyzer/internal/volume"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create the application struct
	app := NewApp()

	// Create service instances
	scannerService := scanner.NewScannerService()
	volumeService := volume.NewVolumeService()
	fileOpsService := fileops.NewFileOpsService()
	preferencesService := preferences.NewPreferencesService()

	// Register startup callback to pass context to services
	err := wails.Run(&options.App{
		Title:  "Disk Analyzer",
		Width:  1200,
		Height: 800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 27, B: 27, A: 1},
		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
			// Pass context to all services
			scannerService.SetContext(ctx)
			volumeService.SetContext(ctx)
			fileOpsService.SetContext(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			app.Shutdown(ctx)
		},
		Bind: []interface{}{
			app,
			scannerService,
			volumeService,
			fileOpsService,
			preferencesService,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
		EnableFileDrops: true,
	})

	if err != nil {
		panic(err)
	}
}
