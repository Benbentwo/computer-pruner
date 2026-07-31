package preferences

import (
	"fmt"
	"os"
	"path/filepath"
)

// AppDirName is the single directory name used for every on-disk artefact
// ComputerPruner owns (preferences, scan cache, anything added later).
const AppDirName = "computer-pruner"

// configDirPerm is deliberately owner-only: the preferences file records which
// directories the user browses and excludes, which is nobody else's business.
const configDirPerm os.FileMode = 0o700

// userConfigDir is os.UserConfigDir behind a variable so tests can pin a
// synthetic location without depending on the host's environment.
var userConfigDir = os.UserConfigDir

// userHomeDir is os.UserHomeDir behind a variable, for the same reason.
var userHomeDir = os.UserHomeDir

// ConfigDir returns the per-user directory that holds this application's state.
//
// It is the *only* function allowed to decide where that directory is. Both the
// preferences file and the scan cache go through it, because they used to
// disagree: preferences hard-coded $HOME/.config while the cache used
// os.UserConfigDir. On macOS those are two different places
// (~/.config vs ~/Library/Application Support) and on Windows $HOME/.config is
// simply wrong — the correct location is %AppData%.
//
// os.UserConfigDir only fails when the environment exposes no configuration
// home at all (no %AppData% on Windows, no $HOME on Unix). Rather than silently
// scattering files into the working directory, we fall back to $HOME/.config
// and, if even the home directory is unknown, return an error so the caller can
// degrade cleanly (run without a cache, serve default preferences) instead of
// writing somewhere unpredictable.
func ConfigDir() (string, error) {
	base, err := userConfigDir()
	if err != nil || base == "" {
		home, homeErr := userHomeDir()
		if homeErr != nil || home == "" {
			return "", fmt.Errorf(
				"cannot locate a user configuration directory (%v) and no home directory is available (%v)",
				err, homeErr,
			)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, AppDirName), nil
}

// EnsureConfigDir returns ConfigDir, creating it if it does not exist.
func EnsureConfigDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, configDirPerm); err != nil {
		return "", fmt.Errorf("cannot create configuration directory %q: %w", dir, err)
	}
	return dir, nil
}
