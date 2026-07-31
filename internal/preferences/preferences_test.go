package preferences

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestClampScanDepth(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"zero means unset", 0, DefaultScanDepth},
		{"negative means unset", -5, DefaultScanDepth},
		{"absurdly negative means unset", -1 << 30, DefaultScanDepth},
		{"minimum is allowed", MinScanDepth, MinScanDepth},
		{"typical value passes through", 12, 12},
		{"maximum is allowed", MaxScanDepth, MaxScanDepth},
		{"above maximum clamps down", MaxScanDepth + 1, MaxScanDepth},
		{"absurdly large clamps down", 1 << 30, MaxScanDepth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClampScanDepth(tt.input); got != tt.want {
				t.Fatalf("ClampScanDepth(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeExclusionPaths(t *testing.T) {
	sep := string(filepath.Separator)
	// abs builds a cleaned absolute path; rawAbs builds an uncleaned one, so the
	// cleaning behaviour is actually exercised rather than being done by the
	// test helper.
	abs := func(parts ...string) string {
		return filepath.Join(append([]string{sep}, parts...)...)
	}
	rawAbs := func(parts ...string) string {
		return sep + strings.Join(parts, sep)
	}

	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"nil becomes empty, never null", nil, []string{}},
		{"blanks are dropped", []string{"", "   ", "\t"}, []string{}},
		{
			"relative paths are dropped",
			[]string{"cache", filepath.Join("some", "where")},
			[]string{},
		},
		{
			"tilde paths are dropped rather than guessed at",
			[]string{"~", "~/Library", `~\AppData`},
			[]string{},
		},
		{
			"absolute paths are cleaned",
			[]string{rawAbs("var", "tmp", "..", "log", "")},
			[]string{abs("var", "log")},
		},
		{
			"duplicates collapse",
			[]string{abs("var", "log"), rawAbs("var", "", "log", "")},
			[]string{abs("var", "log")},
		},
		{
			"surrounding whitespace is trimmed",
			[]string{"  " + abs("data") + "  "},
			[]string{abs("data")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeExclusionPaths(tt.input)
			if got == nil {
				t.Fatal("NormalizeExclusionPaths returned nil; it must always return a non-nil slice")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NormalizeExclusionPaths(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDefaultPreferencesAreUsable(t *testing.T) {
	d := DefaultPreferences()
	if d.ScanDepthLimit != DefaultScanDepth {
		t.Fatalf("default ScanDepthLimit = %d, want %d", d.ScanDepthLimit, DefaultScanDepth)
	}
	if ClampScanDepth(d.ScanDepthLimit) != d.ScanDepthLimit {
		t.Fatal("the default scan depth must survive clamping unchanged")
	}
	if d.ExclusionPaths == nil {
		t.Fatal("default ExclusionPaths must be non-nil so it marshals as [] and not null")
	}
	if len(d.ExclusionPaths) != 0 {
		t.Fatalf("exclusions are now honoured by the scanner, so the default must be empty; got %q", d.ExclusionPaths)
	}
}

func TestGetPreferencesReturnsDefaultsWhenFileMissing(t *testing.T) {
	svc := NewPreferencesServiceAt(filepath.Join(t.TempDir(), "nope", "preferences.json"))
	got := svc.GetPreferences()
	if !reflect.DeepEqual(got, DefaultPreferences()) {
		t.Fatalf("GetPreferences() = %+v, want defaults %+v", got, DefaultPreferences())
	}
}

func TestGetPreferencesReturnsDefaultsWhenFileMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := NewPreferencesServiceAt(path).GetPreferences()
	if !reflect.DeepEqual(got, DefaultPreferences()) {
		t.Fatalf("GetPreferences() = %+v, want defaults %+v", got, DefaultPreferences())
	}
}

func TestSetThenGetPreferencesRoundTrips(t *testing.T) {
	excluded := filepath.Join(string(filepath.Separator), "var", "cache")
	path := filepath.Join(t.TempDir(), "sub", "preferences.json")
	svc := NewPreferencesServiceAt(path)

	in := AppPreferences{
		Theme:                 "dark",
		DefaultDeleteBehavior: "permanent",
		ScanDepthLimit:        11,
		ExclusionPaths:        []string{excluded},
	}
	if err := svc.SetPreferences(in); err != nil {
		t.Fatalf("SetPreferences: %v", err)
	}

	got := svc.GetPreferences()
	if !reflect.DeepEqual(*got, in) {
		t.Fatalf("round trip changed the preferences: got %+v, want %+v", *got, in)
	}
}

func TestSetPreferencesPersistsNormalizedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	svc := NewPreferencesServiceAt(path)

	if err := svc.SetPreferences(AppPreferences{
		Theme:                 "light",
		DefaultDeleteBehavior: "trash",
		ScanDepthLimit:        9999,
		ExclusionPaths:        []string{"relative/path", "", "~/Library"},
	}); err != nil {
		t.Fatalf("SetPreferences: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk AppPreferences
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("stored preferences are not valid JSON: %v", err)
	}

	if onDisk.ScanDepthLimit != MaxScanDepth {
		t.Fatalf("stored ScanDepthLimit = %d, want it clamped to %d", onDisk.ScanDepthLimit, MaxScanDepth)
	}
	if len(onDisk.ExclusionPaths) != 0 {
		t.Fatalf("unusable exclusion entries should not be persisted; got %q", onDisk.ExclusionPaths)
	}
}

func TestGetPreferencesClampsHandEditedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := os.WriteFile(path, []byte(`{"scanDepthLimit": -3, "theme": "  "}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got := NewPreferencesServiceAt(path).GetPreferences()
	if got.ScanDepthLimit != DefaultScanDepth {
		t.Fatalf("ScanDepthLimit = %d, want %d", got.ScanDepthLimit, DefaultScanDepth)
	}
	if got.Theme != DefaultPreferences().Theme {
		t.Fatalf("Theme = %q, want the default %q", got.Theme, DefaultPreferences().Theme)
	}
	if got.ExclusionPaths == nil {
		t.Fatal("ExclusionPaths must never be nil")
	}
}

func TestServiceWithUnresolvableConfigDirDegrades(t *testing.T) {
	svc := NewPreferencesServiceAt("")

	if got := svc.GetPreferences(); !reflect.DeepEqual(got, DefaultPreferences()) {
		t.Fatalf("GetPreferences() = %+v, want defaults", got)
	}
	if err := svc.SetPreferences(*DefaultPreferences()); err == nil {
		t.Fatal("SetPreferences must report an error when there is nowhere to write")
	}
}

func TestConfigDirUsesUserConfigDir(t *testing.T) {
	base := t.TempDir()
	restore := stubDirs(func() (string, error) { return base, nil }, nil)
	defer restore()

	got, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	want := filepath.Join(base, AppDirName)
	if got != want {
		t.Fatalf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestConfigDirFallsBackToHomeDotConfig(t *testing.T) {
	home := t.TempDir()
	restore := stubDirs(
		func() (string, error) { return "", errors.New("no config home") },
		func() (string, error) { return home, nil },
	)
	defer restore()

	got, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	want := filepath.Join(home, ".config", AppDirName)
	if got != want {
		t.Fatalf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestConfigDirFailsWhenNothingIsResolvable(t *testing.T) {
	restore := stubDirs(
		func() (string, error) { return "", errors.New("no config home") },
		func() (string, error) { return "", errors.New("no home") },
	)
	defer restore()

	if _, err := ConfigDir(); err == nil {
		t.Fatal("ConfigDir must fail rather than pick an unpredictable location")
	}
}

func TestEnsureConfigDirCreatesTheDirectory(t *testing.T) {
	base := t.TempDir()
	restore := stubDirs(func() (string, error) { return filepath.Join(base, "cfg"), nil }, nil)
	defer restore()

	dir, err := EnsureConfigDir()
	if err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("expected %q to exist: %v", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", dir)
	}
}

// stubDirs swaps the directory lookups used by ConfigDir. A nil function leaves
// the corresponding lookup untouched. The returned function restores both.
func stubDirs(config, home func() (string, error)) func() {
	oldConfig, oldHome := userConfigDir, userHomeDir
	if config != nil {
		userConfigDir = config
	}
	if home != nil {
		userHomeDir = home
	}
	return func() {
		userConfigDir, userHomeDir = oldConfig, oldHome
	}
}
