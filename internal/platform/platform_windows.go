//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// pathsAreCaseInsensitive is true on Windows: NTFS is case-preserving but
// case-insensitive, so C:\WINDOWS and C:\Windows are the same directory.
const pathsAreCaseInsensitive = true

// platformProtectedPaths resolves the Windows protected-path list from the
// environment (%SystemRoot%, %ProgramFiles%, %ProgramData%, %APPDATA%, ...)
// with literal fallbacks, so a machine with Windows installed on a drive other
// than C: is still guarded correctly.
func platformProtectedPaths() ([]protectedEntry, error) {
	home, err := userHomeDir()
	if err != nil {
		return buildWindowsProtectedPaths(os.Getenv, ""), fmt.Errorf("cannot resolve the user profile directory: %w", err)
	}
	return buildWindowsProtectedPaths(os.Getenv, home), nil
}

// RevealInFileManager opens Explorer with path selected.
func RevealInFileManager(path string) error {
	// The "/select," switch and the path form a single argument. Start (rather
	// than Run) is deliberate: explorer.exe routinely exits non-zero even on
	// success.
	return exec.Command("explorer.exe", "/select,"+path).Start()
}

// PreviewFile opens path with its default handler. Windows has no QuickLook
// equivalent that can be driven from a plain process launch, so "preview"
// degrades to "open".
func PreviewFile(path string) error {
	return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", path).Start()
}

// GetSystemTheme reports the Windows app appearance as "dark" or "light".
func GetSystemTheme() string {
	const key = `HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	output, err := exec.Command("reg", "query", key, "/v", "AppsUseLightTheme").Output()
	if err != nil {
		return "dark"
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return "dark"
	}
	switch strings.ToLower(fields[len(fields)-1]) {
	case "0x1":
		return "light"
	case "0x0":
		return "dark"
	}
	return "dark"
}

// MoveToTrash moves path to the Recycle Bin. See trash_windows.go.
func MoveToTrash(path string) error {
	return recycle(path)
}
