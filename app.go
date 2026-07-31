package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

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

// OpenDirectoryDialog opens a native folder picker and returns the selected path
func (a *App) OpenDirectoryDialog(title string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: title,
	})
}

// DirEntry is a lightweight directory entry for the folder browser
type DirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// ListDirectory returns the immediate children of a directory for the folder browser
func (a *App) ListDirectory(path string) ([]DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []DirEntry
	for _, entry := range entries {
		// Skip hidden files in the browser view
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		result = append(result, DirEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(path, entry.Name()),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
	}

	return result, nil
}
