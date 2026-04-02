package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct is the main application struct passed to wails.Run()
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// Startup is called when the app starts. The context is saved
// so runtime methods can be called
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// Shutdown is called when the app shuts down
func (a *App) Shutdown(ctx context.Context) {
	// Perform any cleanup here
}

// GetContext returns the saved wails runtime context
func (a *App) GetContext() context.Context {
	return a.ctx
}
