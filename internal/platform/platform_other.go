//go:build !darwin && !windows

package platform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// This file is the fallback for platforms ComputerPruner does not ship on
// (Linux, BSD). It exists so the tree compiles, vets and unit-tests on a Linux
// CI runner. Its behaviour is best-effort, and every destructive operation
// either works through a well-known desktop helper or refuses outright.

// pathsAreCaseInsensitive is false here: ext4, btrfs, xfs and friends are
// case-sensitive.
const pathsAreCaseInsensitive = false

// platformProtectedPaths resolves a conservative generic-Unix protected list.
func platformProtectedPaths() ([]protectedEntry, error) {
	home, err := userHomeDir()
	if err != nil {
		return buildGenericUnixProtectedPaths(""), fmt.Errorf("cannot resolve the user home directory: %w", err)
	}
	return buildGenericUnixProtectedPaths(home), nil
}

// RevealInFileManager opens the directory containing path. xdg-open cannot
// select an individual file, so the parent directory is the closest analogue
// of "reveal".
func RevealInFileManager(path string) error {
	return exec.Command("xdg-open", filepath.Dir(path)).Start()
}

// PreviewFile opens path with the desktop's default handler.
func PreviewFile(path string) error {
	return exec.Command("xdg-open", path).Start()
}

// GetSystemTheme reports "dark" or "light", asking GNOME's settings daemon when
// it is available and defaulting to the application's own dark theme.
func GetSystemTheme() string {
	output, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "color-scheme").Output()
	if err != nil {
		return "dark"
	}
	if strings.Contains(strings.ToLower(string(output)), "light") {
		return "light"
	}
	return "dark"
}

// MoveToTrash delegates to gio, the GNOME/freedesktop trash helper. If gio is
// not installed the caller gets a clear refusal rather than a silent
// permanent delete.
func MoveToTrash(path string) error {
	gio, err := exec.LookPath("gio")
	if err != nil {
		return fmt.Errorf(
			"moving items to the trash is not supported on %s without the 'gio' helper; "+
				"install glib2 tools or use permanent delete", runtime.GOOS)
	}
	output, err := exec.Command(gio, "trash", "--", path).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return fmt.Errorf("move to trash failed for %q: %w", path, err)
		}
		return fmt.Errorf("move to trash failed for %q: %w — %s", path, err, detail)
	}
	return nil
}
