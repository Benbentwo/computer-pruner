//go:build darwin

package platform

import (
	"fmt"
	"os/exec"
	"strings"
)

// pathsAreCaseInsensitive is true on macOS by deliberate choice: APFS (and
// HFS+ before it) is formatted case-insensitive by default, so "/Users/Ben"
// and "/users/ben" are the same directory on a stock Mac. Comparing
// case-insensitively can only ever protect *more* paths than the filesystem
// strictly requires, which is the direction we want to err in. Users who
// deliberately formatted a case-sensitive volume lose nothing but the ability
// to delete a path whose name differs from a protected one only by case.
const pathsAreCaseInsensitive = true

// platformProtectedPaths resolves the macOS protected-path list.
//
// When the home directory cannot be determined the static system list is still
// returned, and the accompanying error makes IsProtected refuse everything
// rather than silently leaving the whole home directory unguarded.
func platformProtectedPaths() ([]protectedEntry, error) {
	home, err := userHomeDir()
	if err != nil {
		return buildDarwinProtectedPaths(""), fmt.Errorf("cannot resolve the user home directory: %w", err)
	}
	return buildDarwinProtectedPaths(home), nil
}

// RevealInFileManager reveals path in Finder.
func RevealInFileManager(path string) error {
	return exec.Command("open", "-R", path).Start()
}

// PreviewFile opens path in QuickLook.
func PreviewFile(path string) error {
	return exec.Command("qlmanage", "-p", path).Start()
}

// GetSystemTheme reports the macOS appearance as "dark" or "light".
func GetSystemTheme() string {
	output, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		// The key is absent entirely when the system is in light mode.
		return "light"
	}
	if strings.TrimSpace(string(output)) == "Dark" {
		return "dark"
	}
	return "light"
}

// MoveToTrash moves path to the user's Trash via Finder, which keeps the item
// recoverable and records a "Put Back" location.
func MoveToTrash(path string) error {
	literal, err := appleScriptStringLiteral(path)
	if err != nil {
		return fmt.Errorf("cannot move %q to the Trash: %w", path, err)
	}

	script := fmt.Sprintf(`tell application "Finder" to delete (POSIX file %s as alias)`, literal)
	output, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return fmt.Errorf("move to Trash failed for %q: %w", path, err)
		}
		return fmt.Errorf("move to Trash failed for %q: %w — %s", path, err, detail)
	}
	return nil
}
