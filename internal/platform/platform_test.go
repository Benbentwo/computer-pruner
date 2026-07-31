package platform

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// withProtectedSource swaps the protected-path source for the duration of a
// test. Tests must not run in parallel while using it.
func withProtectedSource(t *testing.T, entries []protectedEntry, err error) {
	t.Helper()
	original := protectedSource
	protectedSource = func() ([]protectedEntry, error) { return entries, err }
	t.Cleanup(func() { protectedSource = original })
}

// subtreeEntries builds a protected list where every entry guards its whole
// subtree, which is the strictest policy and the useful default for tests.
func subtreeEntries(paths ...string) []protectedEntry {
	entries := make([]protectedEntry, 0, len(paths))
	for _, p := range paths {
		entries = append(entries, protectedEntry{Path: p, Depth: depthSubtree})
	}
	return entries
}

// entryPaths projects a builder's output down to plain paths.
func entryPaths(entries []protectedEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Path)
	}
	return out
}

func withHomeDir(t *testing.T, home string, err error) {
	t.Helper()
	original := userHomeDir
	userHomeDir = func() (string, error) { return home, err }
	t.Cleanup(func() { userHomeDir = original })
}

// ---------------------------------------------------------------------------
// Regression guard for the original bug: a "~" must never escape this package.
// ---------------------------------------------------------------------------

func TestProtectedPathBuildersAreAbsoluteAndTildeFree(t *testing.T) {
	const posixHome = "/Users/testuser"
	const winHome = `C:\Users\testuser`

	winEnv := map[string]string{
		"SystemDrive":        "C:",
		"SystemRoot":         `C:\Windows`,
		"ProgramFiles":       `C:\Program Files`,
		"ProgramFiles(x86)":  `C:\Program Files (x86)`,
		"ProgramData":        `C:\ProgramData`,
		"APPDATA":            `C:\Users\testuser\AppData\Roaming`,
		"LOCALAPPDATA":       `C:\Users\testuser\AppData\Local`,
		"USERPROFILE":        winHome,
		"NOT_A_PATH_AT_ALL":  "",
		"SHOULD_BE_IGNORED2": "~/nope",
	}
	getenv := func(key string) string { return winEnv[key] }

	tests := []struct {
		name     string
		paths    []string
		isAbs    func(string) bool
		wantSome []string
	}{
		{
			name:  "darwin",
			paths: entryPaths(buildDarwinProtectedPaths(posixHome)),
			isAbs: func(p string) bool { return strings.HasPrefix(p, "/") },
			wantSome: []string{
				"/System", "/Library", "/Applications", "/Users", "/Volumes",
				"/var", "/tmp", "/etc", "/bin", "/sbin", "/usr", "/opt", "/dev", "/private",
				posixHome,
				posixHome + "/Library/Application Support",
				posixHome + "/Library/Preferences",
				posixHome + "/Library/Caches",
				posixHome + "/Music",
				posixHome + "/Pictures",
				posixHome + "/Documents",
				posixHome + "/Desktop",
				posixHome + "/Downloads",
			},
		},
		{
			name:  "windows",
			paths: entryPaths(buildWindowsProtectedPaths(getenv, winHome)),
			isAbs: isWindowsAbsForTest,
			wantSome: []string{
				`C:\Windows`,
				`C:\Program Files`,
				`C:\Program Files (x86)`,
				`C:\ProgramData`,
				`C:\Users`,
				winHome,
				winHome + `\AppData\Roaming`,
				winHome + `\AppData\Local`,
				winHome + `\Documents`,
				winHome + `\Desktop`,
				winHome + `\Downloads`,
				winHome + `\Pictures`,
				winHome + `\Music`,
			},
		},
		{
			name:  "generic unix",
			paths: entryPaths(buildGenericUnixProtectedPaths(posixHome)),
			isAbs: func(p string) bool { return strings.HasPrefix(p, "/") },
			wantSome: []string{
				"/etc", "/usr", "/var",
				posixHome,
				posixHome + "/Documents",
				posixHome + "/Downloads",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.paths) == 0 {
				t.Fatal("builder returned no protected paths")
			}

			seen := make(map[string]bool, len(tc.paths))
			for _, p := range tc.paths {
				if strings.Contains(p, "~") {
					t.Errorf("protected path %q still contains a tilde; the delete guard would never match a real path", p)
				}
				if !tc.isAbs(p) {
					t.Errorf("protected path %q is not absolute", p)
				}
				if strings.TrimSpace(p) != p {
					t.Errorf("protected path %q has surrounding whitespace", p)
				}
				if seen[p] {
					t.Errorf("protected path %q is listed twice", p)
				}
				seen[p] = true
			}

			for _, want := range tc.wantSome {
				if !seen[want] {
					t.Errorf("expected %q in the protected list, got %v", want, tc.paths)
				}
			}
		})
	}
}

// isWindowsAbsForTest mirrors what Windows considers a fully qualified path,
// without needing filepath's OS-specific behaviour.
func isWindowsAbsForTest(p string) bool {
	if strings.HasPrefix(p, `\\`) {
		return true
	}
	return len(p) >= 3 && isASCIILetter(p[0]) && p[1] == ':' && p[2] == '\\'
}

func TestProtectedPathsForRunningOSExpandsHome(t *testing.T) {
	synthetic := filepath.Join(string(filepath.Separator)+"synthetic-home", "tester")
	withHomeDir(t, synthetic, nil)

	paths := ProtectedPaths()
	if len(paths) == 0 {
		t.Fatal("ProtectedPaths() returned nothing")
	}

	foundHome := false
	for _, p := range paths {
		if strings.Contains(p, "~") {
			t.Errorf("ProtectedPaths() returned %q, which still contains a tilde", p)
		}
		if !filepath.IsAbs(p) {
			t.Errorf("ProtectedPaths() returned %q, which is not absolute", p)
		}
		if equalPath(p, synthetic) {
			foundHome = true
		}
	}
	if !foundHome {
		t.Errorf("the user's home directory %q is not itself protected; got %v", synthetic, paths)
	}
}

func TestGetProtectedPathsAliasMatchesProtectedPaths(t *testing.T) {
	withProtectedSource(t, subtreeEntries("/a", "/b"), nil)
	got := GetProtectedPaths()
	want := ProtectedPaths()
	if len(got) != len(want) {
		t.Fatalf("alias returned %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("alias returned %v, want %v", got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// isWithin boundary behaviour
// ---------------------------------------------------------------------------

func TestIsWithinBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		parent     string
		child      string
		sep        rune
		ignoreCase bool
		want       bool
	}{
		// The classic prefix bug.
		{name: "sibling with shared prefix is not inside", parent: "/Users/bob", child: "/Users/bobby", sep: '/', want: false},
		{name: "reverse shared prefix", parent: "/Users/bobby", child: "/Users/bob", sep: '/', want: false},

		// The four cases called out in the spec.
		{name: "identical", parent: "/a/b", child: "/a/b", sep: '/', want: true},
		{name: "descendant", parent: "/a/b", child: "/a/b/c", sep: '/', want: true},
		{name: "ancestor is not a descendant", parent: "/a/b/c", child: "/a/b", sep: '/', want: false},
		{name: "deep descendant", parent: "/a", child: "/a/b/c/d/e", sep: '/', want: true},

		// Normalisation.
		{name: "trailing separator on parent", parent: "/a/b/", child: "/a/b", sep: '/', want: true},
		{name: "trailing separator on child", parent: "/a/b", child: "/a/b/", sep: '/', want: true},
		{name: "duplicate separators", parent: "/a//b", child: "/a///b//c", sep: '/', want: true},
		{name: "dot segments", parent: "/a/./b", child: "/a/b/./c", sep: '/', want: true},
		{name: "dotdot in child climbs out", parent: "/a/b", child: "/a/b/c/..", sep: '/', want: true},
		{name: "dotdot in child escapes the parent", parent: "/a/b", child: "/a/b/../c", sep: '/', want: false},
		{name: "dotdot in parent", parent: "/a/b/..", child: "/a/z", sep: '/', want: true},
		{name: "dotdot clamps at root", parent: "/", child: "/../etc", sep: '/', want: true},
		{name: "root contains everything", parent: "/", child: "/anything/at/all", sep: '/', want: true},
		{name: "whitespace around paths", parent: "  /a/b  ", child: " /a/b/c ", sep: '/', want: true},

		// Case sensitivity.
		{name: "posix case sensitive", parent: "/a/B", child: "/a/b/c", sep: '/', ignoreCase: false, want: false},
		{name: "posix case insensitive", parent: "/a/B", child: "/a/b/c", sep: '/', ignoreCase: true, want: true},

		// Windows.
		{name: "windows same drive", parent: `C:\Windows`, child: `C:\Windows\System32`, sep: '\\', ignoreCase: true, want: true},
		{name: "windows case folded", parent: `C:\WINDOWS`, child: `c:\windows\system32`, sep: '\\', ignoreCase: true, want: true},
		{name: "windows case sensitive comparison", parent: `C:\WINDOWS`, child: `c:\windows\system32`, sep: '\\', ignoreCase: false, want: false},
		{name: "windows different drive", parent: `C:\Users`, child: `D:\Users\bob`, sep: '\\', ignoreCase: true, want: false},
		{name: "windows forward slashes", parent: `C:\Users`, child: "C:/Users/bob", sep: '\\', ignoreCase: true, want: true},
		{name: "windows shared prefix", parent: `C:\Users\bob`, child: `C:\Users\bobby`, sep: '\\', ignoreCase: true, want: false},
		{name: "windows drive root contains everything", parent: `C:\`, child: `C:\Users\bob`, sep: '\\', ignoreCase: true, want: true},
		{name: "unc same share", parent: `\\srv\share\a`, child: `\\srv\share\a\b`, sep: '\\', ignoreCase: true, want: true},
		{name: "unc different share", parent: `\\srv\share\a`, child: `\\srv\other\a\b`, sep: '\\', ignoreCase: true, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isWithinSep(tc.parent, tc.child, tc.sep, tc.ignoreCase); got != tc.want {
				t.Errorf("isWithinSep(%q, %q, %q, %v) = %v, want %v",
					tc.parent, tc.child, string(tc.sep), tc.ignoreCase, got, tc.want)
			}
		})
	}
}

func TestIsWithinUsesPlatformCaseRule(t *testing.T) {
	// Guarded to the platforms where the rule applies: macOS and Windows treat
	// paths case-insensitively, generic Unix does not.
	parent := filepath.Join(string(filepath.Separator)+"Protected", "Dir")
	child := filepath.Join(string(filepath.Separator)+"protected", "dir", "file.txt")

	got := isWithin(parent, child)
	switch runtime.GOOS {
	case "darwin", "windows":
		if !got {
			t.Errorf("on %s, isWithin(%q, %q) must be true (case-insensitive filesystem)", runtime.GOOS, parent, child)
		}
		if !pathsAreCaseInsensitive {
			t.Errorf("pathsAreCaseInsensitive must be true on %s", runtime.GOOS)
		}
	default:
		if got {
			t.Errorf("on %s, isWithin(%q, %q) must be false (case-sensitive filesystem)", runtime.GOOS, parent, child)
		}
		if pathsAreCaseInsensitive {
			t.Errorf("pathsAreCaseInsensitive must be false on %s", runtime.GOOS)
		}
	}
}

func TestIsFilesystemRoot(t *testing.T) {
	root := string(filepath.Separator)
	if !isFilesystemRoot(root) {
		t.Errorf("isFilesystemRoot(%q) = false, want true", root)
	}
	notRoot := filepath.Join(root, "somewhere")
	if isFilesystemRoot(notRoot) {
		t.Errorf("isFilesystemRoot(%q) = true, want false", notRoot)
	}
}

// ---------------------------------------------------------------------------
// IsProtected
// ---------------------------------------------------------------------------

func TestIsProtectedPolicy(t *testing.T) {
	sep := string(filepath.Separator)
	home := filepath.Join(sep+"synthetic", "users", "bob")
	protected := []protectedEntry{
		{Path: filepath.Join(sep + "System"), Depth: depthSubtree},
		{Path: filepath.Join(sep+"synthetic", "users"), Depth: depthEntryAndChildren},
		{Path: home, Depth: depthEntryOnly},
		{Path: filepath.Join(home, "Documents"), Depth: depthEntryOnly},
	}

	tests := []struct {
		name     string
		path     string
		want     bool
		contains string
	}{
		{name: "empty", path: "", want: true, contains: "empty path"},
		{name: "whitespace only", path: "   ", want: true, contains: "empty path"},
		{name: "tilde", path: "~/Documents", want: true, contains: "unexpanded home-relative"},
		{name: "bare tilde", path: "~", want: true, contains: "unexpanded home-relative"},
		{name: "relative", path: filepath.Join("relative", "thing"), want: true, contains: "not an absolute path"},
		{name: "dot relative", path: ".", want: true, contains: "not an absolute path"},
		{name: "filesystem root", path: sep, want: true, contains: "filesystem or volume root"},
		{name: "protected entry itself", path: filepath.Join(sep + "System"), want: true, contains: "protected system location"},
		{name: "child of protected entry", path: filepath.Join(sep+"System", "Library"), want: true, contains: "inside the protected location"},
		{name: "home itself", path: home, want: true, contains: "protected"},
		{name: "home with trailing separator", path: home + sep, want: true},
		{name: "documents", path: filepath.Join(home, "Documents"), want: true},
		{name: "dotdot back into protected", path: filepath.Join(home, "Documents", "..", "Documents"), want: true},
		{name: "another user home is protected by the users root", path: filepath.Join(sep+"synthetic", "users", "bobby"), want: true, contains: "inside the protected location"},
		{name: "content two levels under the users root is deletable", path: filepath.Join(sep+"synthetic", "users", "bobby", "scratch.iso"), want: false},
		{name: "user content inside a protected container is deletable", path: filepath.Join(home, "Documents", "big.mov"), want: false},
		{name: "user content directly in home is deletable", path: filepath.Join(home, "scratch.iso"), want: false},
		{name: "unrelated absolute path is allowed", path: filepath.Join(sep+"synthetic", "scratch", "big.iso"), want: false},
		{name: "prefix-only sibling of a protected root", path: filepath.Join(sep + "Systemically"), want: false},
	}

	withProtectedSource(t, protected, nil)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := IsProtected(tc.path)
			if got != tc.want {
				t.Fatalf("IsProtected(%q) = %v (%q), want %v", tc.path, got, reason, tc.want)
			}
			if got && reason == "" {
				t.Fatalf("IsProtected(%q) reported protected with an empty reason", tc.path)
			}
			if !got && reason != "" {
				t.Fatalf("IsProtected(%q) reported allowed with reason %q", tc.path, reason)
			}
			if tc.contains != "" && !strings.Contains(reason, tc.contains) {
				t.Fatalf("IsProtected(%q) reason = %q, want it to mention %q", tc.path, reason, tc.contains)
			}
		})
	}
}

func TestIsProtectedFailsClosedWhenProtectedListCannotBeResolved(t *testing.T) {
	withProtectedSource(t, buildGenericUnixProtectedPaths(""), errors.New("no home directory"))

	got, reason := IsProtected(filepath.Join(string(filepath.Separator)+"anywhere", "at", "all"))
	if !got {
		t.Fatal("IsProtected must fail closed when the protected list cannot be resolved")
	}
	if !strings.Contains(reason, "no home directory") {
		t.Fatalf("reason = %q, want it to surface the underlying failure", reason)
	}
}

func TestIsProtectedResolvesSymlinkBypass(t *testing.T) {
	base := realTempDir(t)

	protectedDir := filepath.Join(base, "protected")
	if err := os.MkdirAll(filepath.Join(protectedDir, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	openDir := filepath.Join(base, "open")
	if err := os.MkdirAll(openDir, 0o755); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(base, "shortcut")
	if err := os.Symlink(protectedDir, link); err != nil {
		t.Skipf("symlinks are unavailable on this host: %v", err)
	}

	withProtectedSource(t, subtreeEntries(protectedDir), nil)

	t.Run("symlink to a protected directory", func(t *testing.T) {
		got, reason := IsProtected(link)
		if !got {
			t.Fatalf("IsProtected(%q) = false; a symlink into a protected tree must not be a bypass", link)
		}
		if reason == "" {
			t.Fatal("expected a reason")
		}
	})

	t.Run("existing path underneath the symlink", func(t *testing.T) {
		via := filepath.Join(link, "inner")
		if got, _ := IsProtected(via); !got {
			t.Fatalf("IsProtected(%q) = false; the resolved path is inside the protected tree", via)
		}
	})

	t.Run("non-existent path underneath the symlink", func(t *testing.T) {
		via := filepath.Join(link, "does-not-exist-yet")
		if got, _ := IsProtected(via); !got {
			t.Fatalf("IsProtected(%q) = false; its resolved parent is inside the protected tree", via)
		}
	})

	t.Run("unrelated sibling is still deletable", func(t *testing.T) {
		if got, reason := IsProtected(filepath.Join(openDir, "junk.bin")); got {
			t.Fatalf("IsProtected of an unrelated path = true (%q)", reason)
		}
	})
}

// realTempDir returns t.TempDir() with symlinks resolved, so that hosts whose
// temp directory is itself a symlink (macOS: /var -> /private/var) do not make
// the symlink assertions trivially true.
func realTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return filepath.Clean(dir)
	}
	return filepath.Clean(resolved)
}

// ---------------------------------------------------------------------------
// AppleScript literal escaping (shared, so it is covered on every host)
// ---------------------------------------------------------------------------

func TestAppleScriptStringLiteral(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr string
	}{
		{name: "plain", in: "/Users/bob/file.txt", want: `"/Users/bob/file.txt"`},
		{name: "quote", in: `/tmp/say "hi"`, want: `"/tmp/say \"hi\""`},
		{name: "backslash", in: `/tmp/back\slash`, want: `"/tmp/back\\slash"`},
		{
			// Escaping order matters: backslashes first, then quotes. Escaping
			// quotes first would yield "a\\"b\\c", which is a broken literal.
			name: "backslash then quote ordering",
			in:   `a"b\c`,
			want: `"a\"b\\c"`,
		},
		{name: "trailing backslash", in: `/tmp/dir\`, want: `"/tmp/dir\\"`},
		{name: "unicode is fine", in: "/tmp/café/naïve.txt", want: `"/tmp/café/naïve.txt"`},
		{name: "empty", in: "", wantErr: "empty path"},
		{name: "newline", in: "/tmp/two\nlines", wantErr: "line break"},
		{name: "carriage return", in: "/tmp/two\rlines", wantErr: "line break"},
		{name: "tab", in: "/tmp/two\tparts", wantErr: "control character"},
		{name: "bell", in: "/tmp/ding\x07", wantErr: "control character"},
		{name: "delete char", in: "/tmp/del\x7f", wantErr: "control character"},
		{name: "invalid utf8", in: "/tmp/\xff\xfe", wantErr: "not valid UTF-8"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := appleScriptStringLiteral(tc.in)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("appleScriptStringLiteral(%q) = %q, want error mentioning %q", tc.in, got, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want it to mention %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("appleScriptStringLiteral(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Windows path helpers
// ---------------------------------------------------------------------------

func TestWinCleanAndJoin(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "join drive", got: winJoin("C:", "Users"), want: `C:\Users`},
		{name: "join drive root", got: winJoin(`C:\`, "Users"), want: `C:\Users`},
		{name: "join multi", got: winJoin(`C:\Users\bob`, "AppData", "Roaming"), want: `C:\Users\bob\AppData\Roaming`},
		{name: "join empty base", got: winJoin("", "Users"), want: ""},
		{name: "clean forward slashes", got: winClean("C:/Users/bob"), want: `C:\Users\bob`},
		{name: "clean duplicate separators", got: winClean(`C:\\Users\\\bob`), want: `C:\Users\bob`},
		{name: "clean trailing separator", got: winClean(`C:\Users\bob\`), want: `C:\Users\bob`},
		{name: "keep drive root", got: winClean(`C:\`), want: `C:\`},
		{name: "keep unc prefix", got: winClean(`\\srv\share\dir\`), want: `\\srv\share\dir`},
		{name: "empty", got: winClean("   "), want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

func TestBuildWindowsProtectedPathsUsesEnvironmentThenFallbacks(t *testing.T) {
	t.Run("honours a non-C system drive without dropping the C: defaults", func(t *testing.T) {
		env := map[string]string{"SystemDrive": "D:"}
		paths := entryPaths(buildWindowsProtectedPaths(func(k string) string { return env[k] }, ""))

		// The relocated drive must be covered...
		want := []string{`D:\Windows`, `D:\Program Files`, `D:\Program Files (x86)`, `D:\ProgramData`, `D:\Users`}
		// ...and so must the literal defaults. The environment adds to the
		// protected list, it never replaces it. See the fail-open regression
		// test below for why.
		want = append(want, `C:\Windows`, `C:\Program Files`, `C:\Program Files (x86)`, `C:\ProgramData`, `C:\Users`)
		for _, w := range want {
			if !containsString(paths, w) {
				t.Errorf("expected %q in %v", w, paths)
			}
		}
	})

	t.Run("falls back to literals when the environment is empty", func(t *testing.T) {
		paths := entryPaths(buildWindowsProtectedPaths(func(string) string { return "" }, ""))
		for _, w := range []string{`C:\Windows`, `C:\Program Files`, `C:\Users`} {
			if !containsString(paths, w) {
				t.Errorf("expected %q in %v", w, paths)
			}
		}
	})

	t.Run("nil getenv is tolerated", func(t *testing.T) {
		if paths := buildWindowsProtectedPaths(nil, `C:\Users\bob`); len(paths) == 0 {
			t.Error("expected a fallback list")
		}
	})
}

func TestBuildProtectedPathsWithoutHomeReturnsStaticList(t *testing.T) {
	darwin := entryPaths(buildDarwinProtectedPaths(""))

	// Every static root must still be present...
	for _, want := range append(append([]string{}, darwinSystemRoots...), "/Users", "/Volumes") {
		if !containsString(darwin, want) {
			t.Errorf("expected the static root %q in %v", want, darwin)
		}
	}
	for _, p := range darwin {
		if strings.Contains(p, "~") {
			t.Errorf("tilde leaked into %q", p)
		}
		// ...and nothing home-specific may appear, because there is no home.
		// The per-account entries are templated ("/Users/*/Documents"), so any
		// path under /Users must carry the wildcard in the account position.
		if segs := strings.Split(strings.TrimPrefix(p, "/"), "/"); len(segs) >= 2 && segs[0] == "Users" && segs[1] != wildcardSegment {
			t.Errorf("home-specific path %q leaked into the no-home list", p)
		}
	}

	win := entryPaths(buildWindowsProtectedPaths(func(string) string { return "" }, ""))
	for _, p := range win {
		if strings.Contains(p, "~") {
			t.Errorf("tilde leaked into %q", p)
		}
	}
}

func containsString(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Protection scope: OS-owned trees are guarded all the way down, containers of
// user data guard only the directory itself.
// ---------------------------------------------------------------------------

func depthOf(t *testing.T, entries []protectedEntry, path string) int {
	t.Helper()
	for _, e := range entries {
		if e.Path == path {
			return e.Depth
		}
	}
	t.Fatalf("%q is not in the protected list %v", path, entryPaths(entries))
	return 0
}

func TestProtectionScopePerEntry(t *testing.T) {
	const posixHome = "/Users/testuser"
	const winHome = `C:\Users\testuser`

	darwin := buildDarwinProtectedPaths(posixHome)
	for path, want := range map[string]int{
		"/System":                          depthSubtree,
		"/Library":                         depthSubtree,
		"/usr":                             depthSubtree,
		"/Users":                           depthEntryAndChildren,
		"/Volumes":                         depthEntryAndChildren,
		posixHome:                          depthEntryOnly,
		posixHome + "/Documents":           depthEntryOnly,
		posixHome + "/Library/Caches":      depthEntryOnly,
		posixHome + "/Library/Preferences": depthEntryOnly,
	} {
		if got := depthOf(t, darwin, path); got != want {
			t.Errorf("darwin %q depth = %d, want %d", path, got, want)
		}
	}

	windows := buildWindowsProtectedPaths(func(k string) string {
		return map[string]string{"SystemDrive": "C:", "USERPROFILE": winHome}[k]
	}, winHome)
	for path, want := range map[string]int{
		`C:\Windows`:               depthSubtree,
		`C:\Program Files`:         depthSubtree,
		`C:\ProgramData`:           depthSubtree,
		`C:\Users`:                 depthEntryAndChildren,
		winHome:                    depthEntryOnly,
		winHome + `\Downloads`:     depthEntryOnly,
		winHome + `\AppData\Local`: depthEntryOnly,
	} {
		if got := depthOf(t, windows, path); got != want {
			t.Errorf("windows %q depth = %d, want %d", path, got, want)
		}
	}
}

// TestProtectionScopeKeepsTheApplicationUsable is the counterpart to the
// regression test above: the guard must block system locations *and* still let
// the user clean up their own files, otherwise the disk analyzer can delete
// nothing at all.
func TestProtectionScopeKeepsTheApplicationUsable(t *testing.T) {
	const home = "/Users/testuser"
	withProtectedSource(t, buildDarwinProtectedPaths(home), nil)

	blocked := []string{
		"/",
		"/System",
		"/System/Library",
		"/System/Library/CoreServices/Finder.app",
		"/usr/lib/dyld",
		"/Users",
		home,
		home + "/",
		home + "/./Documents",
		home + "/Documents",
		home + "/Library/Caches",
		"/Users/someone-else",
	}
	for _, p := range blocked {
		if ok, _ := IsProtected(p); !ok {
			t.Errorf("IsProtected(%q) = false, want true", p)
		}
	}

	allowed := []string{
		home + "/Downloads/ubuntu.iso",
		home + "/Documents/old-project",
		home + "/Library/Caches/com.example.app",
		home + "/Movies/render.mov",
		"/Users/someone-else/Downloads/shared.zip",
		"/Volumes/Backup/old-snapshot",
	}
	for _, p := range allowed {
		if ok, reason := IsProtected(p); ok {
			t.Errorf("IsProtected(%q) = true (%s), want false — the user must be able to clean up their own files", p, reason)
		}
	}
}
