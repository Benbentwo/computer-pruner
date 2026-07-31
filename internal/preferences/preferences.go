// Package preferences stores the user's application settings and owns the
// single on-disk location every ComputerPruner artefact lives in (see
// ConfigDir).
//
// The settings are deliberately small and the file is rewritten atomically-ish
// on every change; there is no migration machinery because the application is
// pre-1.0. Values read back from disk are always normalised before being handed
// out, so callers never have to defend against a hand-edited preferences.json
// containing, say, a scan depth of 1e9.
package preferences

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// preferencesFileName is the JSON document inside ConfigDir.
const preferencesFileName = "preferences.json"

// preferencesFilePerm keeps the file readable only by its owner, matching the
// permissions of the directory it lives in.
const preferencesFilePerm os.FileMode = 0o600

// Scan depth bounds. The depth controls how many levels of the scanned tree are
// handed to the frontend renderer; the walk itself always goes all the way
// down. Anything past a few dozen levels is unrenderable, and a negative or
// zero value is meaningless, so both ends are clamped.
const (
	// DefaultScanDepth is used when no preference is set or the stored value is
	// not a usable depth at all.
	DefaultScanDepth = 8
	// MinScanDepth is the shallowest tree worth drawing: the root plus nothing.
	MinScanDepth = 1
	// MaxScanDepth caps the depth so a hand-edited preferences file cannot make
	// the renderer try to draw a pathologically deep tree.
	MaxScanDepth = 64
)

// AppPreferences represents the application preferences and settings.
type AppPreferences struct {
	Theme                 string `json:"theme"`
	DefaultDeleteBehavior string `json:"defaultDeleteBehavior"`
	// ScanDepthLimit is how many levels of the tree are sent to the frontend.
	// It is clamped to [MinScanDepth, MaxScanDepth] on both read and write.
	ScanDepthLimit int `json:"scanDepthLimit"`
	// ExclusionPaths are absolute directories whose subtrees the scanner skips.
	// Entries must be absolute and fully expanded — a literal "~" is ignored
	// rather than guessed at, because home-directory expansion is owned by
	// internal/platform.
	ExclusionPaths []string `json:"exclusionPaths"`
}

// PreferencesService manages application preferences.
type PreferencesService struct {
	mu sync.Mutex
	// filePath is empty exactly when pathErr is non-nil.
	filePath string
	pathErr  error
}

// NewPreferencesService creates a new instance of PreferencesService backed by
// the shared configuration directory.
//
// If the configuration directory cannot be resolved the service still works: it
// serves defaults and reports an error from SetPreferences, rather than writing
// a preferences file to an unpredictable location.
func NewPreferencesService() *PreferencesService {
	dir, err := ConfigDir()
	if err != nil {
		return &PreferencesService{pathErr: err}
	}
	return NewPreferencesServiceAt(filepath.Join(dir, preferencesFileName))
}

// NewPreferencesServiceAt creates a service backed by an explicit file path.
// It exists for tests and for any caller that needs to relocate the file.
func NewPreferencesServiceAt(filePath string) *PreferencesService {
	if strings.TrimSpace(filePath) == "" {
		return &PreferencesService{pathErr: errors.New("preferences file path is empty")}
	}
	return &PreferencesService{filePath: filePath}
}

// FilePath returns the location the preferences are read from and written to.
// It is empty when the configuration directory could not be resolved.
func (p *PreferencesService) FilePath() string {
	return p.filePath
}

// GetPreferences returns the current application preferences.
//
// It never fails: a missing, unreadable or malformed file yields the defaults,
// and individual out-of-range values are clamped. The returned value is always
// safe to feed straight into the scanner.
func (p *PreferencesService) GetPreferences() *AppPreferences {
	defaults := DefaultPreferences()

	p.mu.Lock()
	path := p.filePath
	p.mu.Unlock()

	if path == "" {
		return defaults
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return defaults
	}

	var prefs AppPreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return defaults
	}

	// Merge with defaults to fill any missing fields.
	if strings.TrimSpace(prefs.Theme) == "" {
		prefs.Theme = defaults.Theme
	}
	if strings.TrimSpace(prefs.DefaultDeleteBehavior) == "" {
		prefs.DefaultDeleteBehavior = defaults.DefaultDeleteBehavior
	}

	normalized := prefs.Normalized()
	return &normalized
}

// SetPreferences updates the application preferences and persists them.
// The stored document is the normalised form, so a value written by the UI and
// a value read back are always identical.
func (p *PreferencesService) SetPreferences(prefs AppPreferences) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.filePath == "" {
		return p.pathErr
	}

	if err := os.MkdirAll(filepath.Dir(p.filePath), configDirPerm); err != nil {
		return err
	}

	data, err := json.MarshalIndent(prefs.Normalized(), "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(p.filePath, data, preferencesFilePerm)
}

// Normalized returns a copy of the preferences with every field forced into a
// usable range: the scan depth clamped, and the exclusion list trimmed,
// cleaned, de-duplicated and stripped of entries the scanner could not act on.
func (p AppPreferences) Normalized() AppPreferences {
	out := p
	out.Theme = strings.TrimSpace(out.Theme)
	out.DefaultDeleteBehavior = strings.TrimSpace(out.DefaultDeleteBehavior)
	out.ScanDepthLimit = ClampScanDepth(out.ScanDepthLimit)
	out.ExclusionPaths = NormalizeExclusionPaths(out.ExclusionPaths)
	return out
}

// ClampScanDepth forces depth into [MinScanDepth, MaxScanDepth].
//
// A depth of zero or less is treated as "unset" and becomes DefaultScanDepth —
// clamping it to 1 would silently reduce every scan to a single level, which
// looks like a broken application rather than a corrected setting.
func ClampScanDepth(depth int) int {
	switch {
	case depth <= 0:
		return DefaultScanDepth
	case depth < MinScanDepth:
		return MinScanDepth
	case depth > MaxScanDepth:
		return MaxScanDepth
	default:
		return depth
	}
}

// NormalizeExclusionPaths cleans a user-supplied exclusion list. It always
// returns a non-nil slice so the JSON document contains [] rather than null.
//
// Entries that the scanner cannot act on are dropped here rather than being
// silently ignored later:
//   - blank entries;
//   - "~"-prefixed entries, because expanding a tilde is internal/platform's
//     job and guessing at it here is how the protected-path bug happened;
//   - relative paths, which have no stable meaning against an arbitrary scan
//     root.
func NormalizeExclusionPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))

	for _, raw := range paths {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
			continue
		}
		if !filepath.IsAbs(p) {
			continue
		}
		p = filepath.Clean(p)
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}

	return out
}

// DefaultPreferences returns the settings used when nothing has been saved yet.
//
// ExclusionPaths defaults to empty on purpose. The previous default excluded
// /System, /Library and /Applications, but nothing ever read the list — now
// that the scanner honours it, shipping those defaults would make a whole-disk
// scan on macOS silently omit most of the disk, which is the opposite of what a
// disk analyser is for. Exclusions are now an explicit opt-in.
func DefaultPreferences() *AppPreferences {
	return &AppPreferences{
		Theme:                 "auto",
		DefaultDeleteBehavior: "trash",
		ScanDepthLimit:        DefaultScanDepth,
		ExclusionPaths:        []string{},
	}
}
