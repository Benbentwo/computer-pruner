package main

import (
	"context"
	"embed"

	"github.com/Benbentwo/computer-pruner/internal/fileops"
	"github.com/Benbentwo/computer-pruner/internal/preferences"
	"github.com/Benbentwo/computer-pruner/internal/scanner"
	"github.com/Benbentwo/computer-pruner/internal/volume"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create the application struct
	app := NewApp()

	// Create service instances.
	//
	// The scanner reads the user's scan depth and exclusion paths from the
	// preferences service on every scan, so preferences must be constructed
	// first and handed to the scanner. Passing nil here would silently pin the
	// scanner to the built-in defaults and a settings change would never take
	// effect.
	preferencesService := preferences.NewPreferencesService()
	scannerService := scanner.NewScannerServiceWithPreferences(preferencesService)
	volumeService := volume.NewVolumeService()
	fileOpsService := fileops.NewFileOpsService()

	// Register startup callback to pass context to services
	err := wails.Run(&options.App{
		Title:     "ComputerPruner",
		Width:     1200,
		Height:    800,
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
			// Release the on-disk scan cache so its lock file does not survive
			// the process.
			_ = scannerService.Close()
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
	})

	if err != nil {
		panic(err)
	}
}
