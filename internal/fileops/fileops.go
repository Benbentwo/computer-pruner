// Package fileops implements the destructive half of ComputerPruner: moving
// items to the platform trash, deleting them permanently, and the read-only
// helpers the UI needs around them.
//
// Every path that reaches a destructive call is first cleared by
// platform.IsProtected. That check is deliberately the *only* gate: it resolves
// symlinks, normalises the path and matches on path boundaries, so this package
// never has to reason about path syntax itself.
package fileops

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/Benbentwo/computer-pruner/internal/platform"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// checkProtected decides whether a path may be modified and, if not, why.
//
// It is a package-level variable so tests can substitute a synthetic policy and
// exercise the guard without depending on the host's real filesystem layout.
// Production code must never reassign it.
var checkProtected = platform.IsProtected

// The two destructive primitives, also indirected for tests so a unit test can
// prove a protected path is skipped without trashing anything for real.
var (
	moveToTrash = platform.MoveToTrash
	removeAll   = os.RemoveAll
)

// DeleteResult represents the result of a delete operation
type DeleteResult struct {
	DeletedCount int      `json:"deletedCount"`
	FreedBytes   int64    `json:"freedBytes"`
	Errors       []string `json:"errors"`
}

// DeleteProgress represents progress during a batch delete
type DeleteProgress struct {
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	CurrentPath string `json:"currentPath"`
}

// FileInfo represents information about a single file
type FileInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	IsDir       bool   `json:"isDir"`
	ModTime     string `json:"modTime"`
	Permissions string `json:"permissions"`
}

// FileOpsService provides file operation functionality
type FileOpsService struct {
	ctx context.Context
}

// NewFileOpsService creates a new instance of FileOpsService
func NewFileOpsService() *FileOpsService {
	return &FileOpsService{}
}

// SetContext sets the wails runtime context for the service
func (f *FileOpsService) SetContext(ctx context.Context) {
	f.ctx = ctx
}

// DeletePaths moves the given paths to the platform trash / Recycle Bin, which
// keeps them recoverable. Protected paths are skipped and reported in Errors.
func (f *FileOpsService) DeletePaths(paths []string) (*DeleteResult, error) {
	return f.deleteBatch(paths, func(path string) error {
		return moveToTrash(path)
	}), nil
}

// DeletePathsPermanently removes the given paths with no possibility of
// recovery. Protected paths are skipped and reported in Errors.
func (f *FileOpsService) DeletePathsPermanently(paths []string) (*DeleteResult, error) {
	return f.deleteBatch(paths, func(path string) error {
		return removeAll(path)
	}), nil
}

// deleteBatch validates every path up front, and then validates each one again
// immediately before it is destroyed.
//
// The up-front pass means a batch containing a protected path is reported as
// such without half of it having already been deleted. It is not, on its own, a
// sufficient guard: by the time path N is destroyed its verdict is N-1 full
// deletes old, and a directory can be renamed into place underneath a stale
// "allowed". The second check immediately before each destroy shrinks that
// window to the interval between the check and the syscall.
//
// That interval is not zero. Closing it completely needs fd-relative deletion
// (openat/unlinkat with O_NOFOLLOW), which the macOS trash path — an AppleScript
// round-trip through Finder — cannot express at all. The residual race is
// documented in SECURITY.md under "Known limitations"; it requires an attacker
// who can already write inside the directory being cleaned.
func (f *FileOpsService) deleteBatch(paths []string, destroy func(string) error) *DeleteResult {
	result := &DeleteResult{Errors: []string{}}

	allowed := make([]string, 0, len(paths))
	for _, path := range paths {
		protected, reason := checkProtected(path)
		if protected {
			result.Errors = append(result.Errors,
				fmt.Sprintf("Protected path, skipped: %s — %s", path, reason))
			continue
		}
		allowed = append(allowed, path)
	}

	total := len(allowed)
	for i, path := range allowed {
		// Re-validate: the verdict from the loop above may be stale.
		if protected, reason := checkProtected(path); protected {
			result.Errors = append(result.Errors,
				fmt.Sprintf("Protected path, skipped: %s — %s", path, reason))
			continue
		}

		// Measure before destroying; a failed delete simply contributes zero.
		size := pathSize(path)

		f.emit("delete:progress", DeleteProgress{
			Current:     i + 1,
			Total:       total,
			CurrentPath: path,
		})

		if err := destroy(path); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", path, err.Error()))
			continue
		}

		result.DeletedCount++
		result.FreedBytes += size
	}

	f.emit("delete:complete", result)
	return result
}

// emit publishes a Wails event, tolerating a service that has not been given a
// runtime context yet (which is the case in unit tests and before startup).
func (f *FileOpsService) emit(event string, data interface{}) {
	if f.ctx == nil {
		return
	}
	runtime.EventsEmit(f.ctx, event, data)
}

// RevealInFileManager reveals path in the platform file manager.
func (f *FileOpsService) RevealInFileManager(path string) error {
	return platform.RevealInFileManager(path)
}

// PreviewFile previews path using the platform's preview mechanism
// (QuickLook on macOS, the default handler on Windows).
func (f *FileOpsService) PreviewFile(path string) error {
	return platform.PreviewFile(path)
}

// GetFileInfo returns detailed information about a specific file
func (f *FileOpsService) GetFileInfo(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		Name:        info.Name(),
		Path:        path,
		Size:        info.Size(),
		IsDir:       info.IsDir(),
		ModTime:     info.ModTime().Format(time.RFC3339),
		Permissions: info.Mode().Perm().String(),
	}, nil
}

// pathSize reports the on-disk size attributable to path. Symlinks are not
// followed, so a link into a large tree is not counted as that tree.
func pathSize(path string) int64 {
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	if info.IsDir() {
		return dirSize(path)
	}
	return info.Size()
}

// dirSize recursively calculates the total size of the regular files under path
func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip unreadable entries rather than abandoning the walk
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}
