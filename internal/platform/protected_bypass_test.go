package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file is the regression suite for the protected-path bypasses found in
// the adversarial review. Every test here fails against the pre-fix guard.
//
// Everything is written to run on any host: the Windows rules are exercised
// through the separator-parameterised helpers (matchDepthSep, isAbsSep,
// splitPathSep) and the pure builders, never through the running OS.

// ---------------------------------------------------------------------------
// 1. The parent of a protected directory must itself be protected.
//
// Before the fix, ~/Library/Application Support was protected while ~/Library
// was not, and depth does not propagate upwards — so one RemoveAll("~/Library")
// destroyed Keychains, Mail, Messages and all three "protected" children.
// ---------------------------------------------------------------------------

func TestEveryAncestorOfAProtectedEntryIsAlsoProtected(t *testing.T) {
	cases := []struct {
		name    string
		entries []protectedEntry
		sep     rune
	}{
		{name: "darwin", entries: buildDarwinProtectedPaths("/Users/testuser"), sep: '/'},
		{name: "generic unix", entries: buildGenericUnixProtectedPaths("/home/testuser"), sep: '/'},
		{
			name: "windows",
			entries: buildWindowsProtectedPaths(func(k string) string {
				return map[string]string{"SystemDrive": "C:", "USERPROFILE": `C:\Users\testuser`}[k]
			}, `C:\Users\testuser`),
			sep: '\\',
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			listed := make(map[string]bool, len(tc.entries))
			for _, e := range tc.entries {
				listed[e.Path] = true
			}
			for _, e := range tc.entries {
				volume, segs := splitPathSep(e.Path, tc.sep)
				for k := len(segs) - 1; k >= 1; k-- {
					ancestor := joinSegments(volume, segs[:k], tc.sep)
					if !listed[ancestor] {
						t.Errorf("%q is protected but its ancestor %q is not; "+
							"one delete of the ancestor destroys the protected child",
							e.Path, ancestor)
					}
				}
			}
		})
	}
}

func TestDarwinLibraryAndCredentialDirectoriesAreProtected(t *testing.T) {
	const home = "/Users/testuser"
	withProtectedSource(t, buildDarwinProtectedPaths(home), nil)

	blocked := []string{
		// The trunk, not just the leaves.
		home + "/Library",
		home + "/Library/Application Support",
		home + "/Library/Preferences",
		home + "/Library/Caches",
		// Credential stores: protected all the way down, not just the dir.
		home + "/Library/Keychains",
		home + "/Library/Keychains/login.keychain-db",
		home + "/.ssh",
		home + "/.ssh/id_ed25519",
		home + "/.gnupg/private-keys-v1.d",
		home + "/.aws/credentials",
		home + "/.kube/config",
		// Newly covered system roots.
		"/cores",
		"/Network/Servers",
	}
	for _, p := range blocked {
		if ok, _ := IsProtected(p); !ok {
			t.Errorf("IsProtected(%q) = false, want true", p)
		}
	}
}

func TestWindowsAppDataParentIsProtected(t *testing.T) {
	const home = `C:\Users\testuser`
	entries := buildWindowsProtectedPaths(func(k string) string {
		return map[string]string{"SystemDrive": "C:", "USERPROFILE": home}[k]
	}, home)

	for _, want := range []string{
		home + `\AppData`,
		home + `\AppData\Roaming`,
		home + `\AppData\Local`,
		home + `\AppData\LocalLow`,
		home + `\.ssh`,
	} {
		if !containsString(entryPaths(entries), want) {
			t.Errorf("expected %q in the Windows protected list", want)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Windows: alternate spellings of a volume must not evade the list.
// ---------------------------------------------------------------------------

func TestWindowsAlternateVolumeSpellingsAreCanonicalised(t *testing.T) {
	const entry = `C:\Windows`

	protectedSpellings := []string{
		`C:\Windows\System32\drivers`,
		`\\?\C:\Windows\System32\drivers`,
		`\\.\C:\Windows\System32\drivers`,
		`\??\C:\Windows\System32\drivers`,
		`\\?\c:\windows\system32`,
		`\\localhost\C$\Windows\System32`,
		`\\127.0.0.1\c$\Windows`,
		`\\?\UNC\localhost\C$\Windows\System32`,
		// Trailing dots and spaces are stripped by Windows on open.
		`C:\Windows.\System32`,
		`C:\Windows \System32`,
		`C:\Windows...\System32`,
	}
	for _, p := range protectedSpellings {
		if within, _ := matchDepthSep(entry, p, '\\', true); !within {
			t.Errorf("matchDepthSep(%q, %q) = false; this spelling bypasses the protected list", entry, p)
		}
	}

	// A genuinely different location must still be allowed.
	for _, p := range []string{
		`D:\Windows\System32`,
		`\\?\D:\Windows\System32`,
		`C:\Windowsy\System32`,
		`\\srv\share\Windows\System32`,
	} {
		if within, _ := matchDepthSep(entry, p, '\\', true); within {
			t.Errorf("matchDepthSep(%q, %q) = true; unrelated locations must stay deletable", entry, p)
		}
	}
}

func TestStripWindowsDevicePrefix(t *testing.T) {
	tests := []struct{ in, want string }{
		{`\\?\C:\Windows`, `C:\Windows`},
		{`\\.\C:\Windows`, `C:\Windows`},
		{`\??\C:\Windows`, `C:\Windows`},
		{`\\?\UNC\srv\share\dir`, `\\srv\share\dir`},
		{`\\.\UNC\srv\share`, `\\srv\share`},
		{`C:\Windows`, `C:\Windows`},
		{`\\srv\share\dir`, `\\srv\share\dir`},
		{`\\?\`, `\\?\`},
	}
	for _, tc := range tests {
		if got := stripWindowsDevicePrefix(tc.in); got != tc.want {
			t.Errorf("stripWindowsDevicePrefix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsAbsSepRefusesUnrecognisedWindowsVolumes(t *testing.T) {
	abs := []string{
		`C:\Windows`,
		`\\?\C:\Windows`,
		`\\.\C:\Windows`,
		`\\srv\share\dir`,
		`\\?\UNC\srv\share\dir`,
		`C:/Windows`,
	}
	for _, p := range abs {
		if !isAbsSep(p, '\\') {
			t.Errorf("isAbsSep(%q) = false, want true", p)
		}
	}

	// Every one of these is filepath.IsAbs()==true on Windows but names a
	// volume the protected list cannot reason about, or is not fully
	// qualified at all. All must be refused so IsProtected fails closed.
	notAbs := []string{
		``,
		`C:foo`,
		`\Windows\System32`,
		`Windows`,
		`\\srv`,
		`\\?\Volume{b75e2c83-0000-0000-0000-602f00000000}\Windows`,
		`\\?\GLOBALROOT\Device\HarddiskVolume2\Windows`,
		`\\.\PhysicalDrive0`,
	}
	for _, p := range notAbs {
		if isAbsSep(p, '\\') {
			t.Errorf("isAbsSep(%q) = true; an unrecognised volume must fail closed", p)
		}
	}

	if !isAbsSep("/System", '/') || isAbsSep("System", '/') {
		t.Error("POSIX absoluteness rule regressed")
	}
}

func TestRefuseOnSyntaxFailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		sep    rune
		refuse bool
	}{
		{name: "empty", path: "", sep: '/', refuse: true},
		{name: "tilde", path: "~/Documents", sep: '/', refuse: true},
		{name: "bare tilde", path: "~", sep: '/', refuse: true},
		{name: "windows tilde", path: `~\Documents`, sep: '\\', refuse: true},
		{name: "relative posix", path: "a/b", sep: '/', refuse: true},
		{name: "absolute posix", path: "/a/b", sep: '/', refuse: false},
		{name: "drive relative", path: `C:foo`, sep: '\\', refuse: true},
		{name: "rooted no drive", path: `\Windows`, sep: '\\', refuse: true},
		{name: "volume guid", path: `\\?\Volume{1}\x`, sep: '\\', refuse: true},
		{name: "extended length drive", path: `\\?\C:\Windows`, sep: '\\', refuse: false},
		{name: "unc", path: `\\srv\share\x`, sep: '\\', refuse: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := refuseOnSyntax(tc.path, tc.sep)
			if got != tc.refuse {
				t.Fatalf("refuseOnSyntax(%q, %q) = %v (%s), want %v", tc.path, string(tc.sep), got, reason, tc.refuse)
			}
			if got && reason == "" {
				t.Fatal("refusal with no reason")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Unicode normalisation (NFC vs NFD) must not defeat the guard.
// ---------------------------------------------------------------------------

func TestUnicodeNormalisationDoesNotDefeatTheGuard(t *testing.T) {
	// A home outside the users root, so that the /Users/* template cannot mask
	// the normalisation question. These entries exist only because
	// os.UserHomeDir() reported this path, and the probe arrives in the other
	// normal form — which is what happens when the path comes from the
	// scanner's readdir walk on HFS+ rather than from $HOME.
	const nfcHome = "/Volumes/Server/Jos\u00e9"            // é as a single code point
	const nfdHome = "/Volumes/Server/Jose\u0301"           // e + combining acute
	const nfdKafe = "/Volumes/Server/Jos\u00e9/Cafe\u0301" // NFD trailing segment

	entries := buildDarwinProtectedPaths(nfcHome)
	withProtectedSource(t, entries, nil)

	for _, p := range []string{
		nfcHome + "/Documents",
		nfdHome + "/Documents",
		nfdHome + "/Library/Caches",
		nfdHome,
	} {
		if ok, _ := IsProtected(p); !ok {
			t.Errorf("IsProtected(%q) = false; NFC/NFD spellings name the same directory on macOS", p)
		}
	}

	// The user's own files inside those directories stay deletable in both
	// spellings — the fix must not over-protect.
	for _, p := range []string{
		nfcHome + "/Documents/big.mov",
		nfdHome + "/Documents/big.mov",
		nfdKafe,
	} {
		if ok, reason := IsProtected(p); ok {
			t.Errorf("IsProtected(%q) = true (%s), want false", p, reason)
		}
	}
}

func TestEqualSegmentNormalises(t *testing.T) {
	tests := []struct {
		a, b       string
		ignoreCase bool
		want       bool
	}{
		{a: "Jos\u00e9", b: "Jose\u0301", want: true},
		{a: "Jose\u0301", b: "Jos\u00e9", want: true},
		{a: "JOS\u00c9", b: "jose\u0301", ignoreCase: true, want: true},
		{a: "Jos\u00e9", b: "Josef", want: false},
		{a: "Documents", b: "Documents", want: true},
		{a: "Documents", b: "documents", want: false},
		{a: "Documents", b: "documents", ignoreCase: true, want: true},
	}
	for _, tc := range tests {
		if got := equalSegment(tc.a, tc.b, tc.ignoreCase); got != tc.want {
			t.Errorf("equalSegment(%q, %q, %v) = %v, want %v", tc.a, tc.b, tc.ignoreCase, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// 4. Environment variables must not be able to shrink the Windows list.
// ---------------------------------------------------------------------------

func TestWindowsEnvironmentCannotShrinkTheProtectedList(t *testing.T) {
	// A shortcut or parent process that sets these can otherwise relocate the
	// entire protected list to directories that do not exist, and IsProtected
	// returns (false, "") with no error — the fail-closed branch never runs.
	decoy := map[string]string{
		"SystemDrive":       `D:`,
		"SystemRoot":        `D:\Decoy`,
		"windir":            `D:\Decoy2`,
		"ProgramFiles":      `D:\Decoy3`,
		"ProgramFiles(x86)": `D:\Decoy4`,
		"ProgramData":       `D:\Decoy5`,
		"APPDATA":           `D:\Decoy6`,
		"LOCALAPPDATA":      `D:\Decoy7`,
		"USERPROFILE":       `D:\Decoy8`,
	}
	paths := entryPaths(buildWindowsProtectedPaths(func(k string) string { return decoy[k] }, ""))

	mustHave := []string{
		`C:\Windows`,
		`C:\Program Files`,
		`C:\Program Files (x86)`,
		`C:\ProgramData`,
		`C:\Users`,
		`C:\Users\*\AppData`,
		`D:\Windows`,
		`D:\Decoy`,
	}
	for _, want := range mustHave {
		if !containsString(paths, want) {
			t.Errorf("FAIL-OPEN: %q is not in the protected list built from a hostile environment: %v", want, paths)
		}
	}

	// And the real system directory must actually be refused.
	withProtectedSource(t, buildWindowsProtectedPaths(func(k string) string { return decoy[k] }, ""), nil)
	if within, _ := matchDepthSep(`C:\Windows`, `C:\Windows\System32`, '\\', true); !within {
		t.Error(`C:\Windows\System32 is not inside C:\Windows`)
	}
}

// ---------------------------------------------------------------------------
// 5 + 7. A bogus $HOME, and other users' data.
// ---------------------------------------------------------------------------

func TestHostileHomeDirectoryDoesNotUnprotectRealAccounts(t *testing.T) {
	// os.UserHomeDir() on darwin is literally $HOME with no validation, and it
	// returns no error for a nonsense value — so the fail-closed branch never
	// fires. The users-root template is what makes the tampering ineffective.
	withProtectedSource(t, buildDarwinProtectedPaths("/tmp/attacker-home"), nil)

	blocked := []string{
		"/Users/bob",
		"/Users/bob/Documents",
		"/Users/bob/Desktop",
		"/Users/bob/Library",
		"/Users/bob/Library/Application Support",
		"/Users/bob/.ssh/id_rsa",
	}
	for _, p := range blocked {
		if ok, _ := IsProtected(p); !ok {
			t.Errorf("IsProtected(%q) = false with a tampered $HOME; the users-root template must still apply", p)
		}
	}
}

func TestOtherUsersDataIsProtectedTheSameAsYourOwn(t *testing.T) {
	withProtectedSource(t, buildDarwinProtectedPaths("/Users/testuser"), nil)

	for _, p := range []string{
		"/Users/alice",
		"/Users/alice/Documents",
		"/Users/alice/Library",
		"/Users/alice/Library/Application Support",
		"/Users/alice/.ssh",
		"/Users/alice/.ssh/config",
	} {
		if ok, _ := IsProtected(p); !ok {
			t.Errorf("IsProtected(%q) = false; another account's data must be guarded like your own", p)
		}
	}

	// Symmetry the other way: the *contents* of another user's Downloads stay
	// deletable, exactly as they are in your own.
	for _, p := range []string{
		"/Users/alice/Downloads/shared.zip",
		"/Users/alice/Documents/old-project",
	} {
		if ok, reason := IsProtected(p); ok {
			t.Errorf("IsProtected(%q) = true (%s); user content must stay deletable", p, reason)
		}
	}
}

// ---------------------------------------------------------------------------
// 8. A second macOS volume's system directories.
// ---------------------------------------------------------------------------

func TestClonedSystemVolumeIsProtected(t *testing.T) {
	withProtectedSource(t, buildDarwinProtectedPaths("/Users/testuser"), nil)

	blocked := []string{
		"/Volumes/Backup HD/System",
		"/Volumes/Backup HD/System/Library/CoreServices",
		"/Volumes/Backup HD/usr/lib",
		"/Volumes/Backup HD/private/var/db",
		"/Volumes/Backup HD/Applications",
		"/Volumes/Backup HD/Users",
		"/Volumes/Backup HD/Users/bob",
	}
	for _, p := range blocked {
		if ok, _ := IsProtected(p); !ok {
			t.Errorf("IsProtected(%q) = false; a mounted system volume must not be deletable", p)
		}
	}

	// A plain data volume is still fully usable.
	for _, p := range []string{
		"/Volumes/Backup HD/old-snapshot",
		"/Volumes/Photos/2019/DCIM",
		"/Volumes/Backup HD/Users/bob/Downloads/x.iso",
	} {
		if ok, reason := IsProtected(p); ok {
			t.Errorf("IsProtected(%q) = true (%s); ordinary data on an external volume must stay deletable", p, reason)
		}
	}
}

// ---------------------------------------------------------------------------
// 9. Windows 8.3 short names must not be classified on the alias alone.
// ---------------------------------------------------------------------------

func TestWindowsShortNameSegment(t *testing.T) {
	shortNames := []string{"PROGRA~1", "progra~2", "PROGRA~1.TXT", "DOCUME~12"}
	for _, s := range shortNames {
		if !windowsShortNameSegment(s) {
			t.Errorf("windowsShortNameSegment(%q) = false, want true", s)
		}
	}
	ordinary := []string{"Program Files", "foo~bar", "~1", "~", "file~", "my~file.txt", ""}
	for _, s := range ordinary {
		if windowsShortNameSegment(s) {
			t.Errorf("windowsShortNameSegment(%q) = true, want false", s)
		}
	}

	if !hasWindowsShortNameSegment(`C:\PROGRA~1\Foo`) {
		t.Error("hasWindowsShortNameSegment missed an 8.3 alias")
	}
	if hasWindowsShortNameSegment(`C:\Program Files\Foo`) {
		t.Error("hasWindowsShortNameSegment flagged an ordinary path")
	}
	if anyCandidateFreeOfShortNames([]string{`C:\PROGRA~1\Foo`}) {
		t.Error("an unresolvable 8.3 path must be refused")
	}
	if !anyCandidateFreeOfShortNames([]string{`C:\PROGRA~1\Foo`, `C:\Program Files\Foo`}) {
		t.Error("a resolved long-name candidate must be accepted")
	}
}

// ---------------------------------------------------------------------------
// resolutionCandidates must resolve more than one missing level.
// ---------------------------------------------------------------------------

func TestSymlinkedAncestorSeveralLevelsUpIsResolved(t *testing.T) {
	base := realTempDir(t)

	protectedDir := filepath.Join(base, "protected")
	if err := os.MkdirAll(protectedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "shortcut")
	if err := os.Symlink(protectedDir, link); err != nil {
		t.Skipf("symlinks are unavailable on this host: %v", err)
	}

	withProtectedSource(t, subtreeEntries(protectedDir), nil)

	// Two and three missing components below a symlinked ancestor. Resolving
	// only the immediate parent classified these as allowed.
	for _, p := range []string{
		filepath.Join(link, "missing-a", "missing-b"),
		filepath.Join(link, "missing-a", "missing-b", "missing-c"),
	} {
		if ok, _ := IsProtected(p); !ok {
			t.Errorf("IsProtected(%q) = false; its symlinked ancestor is inside the protected tree", p)
		}
	}
}

// ---------------------------------------------------------------------------
// Boundary behaviour must survive the wildcard machinery.
// ---------------------------------------------------------------------------

func TestWildcardMatchingRespectsSegmentBoundaries(t *testing.T) {
	tests := []struct {
		entry     string
		candidate string
		want      bool
		wantBelow int
	}{
		{entry: "/Users/*/Documents", candidate: "/Users/bob/Documents", want: true},
		{entry: "/Users/*/Documents", candidate: "/Users/bob/Documents/a/b", want: true, wantBelow: 2},
		{entry: "/Users/*/Documents", candidate: "/Users/bob/Documentsss", want: false},
		{entry: "/Users/*/Documents", candidate: "/Users/Documents", want: false},
		{entry: "/Users/*/Documents", candidate: "/Users/bob", want: false},
		{entry: "/Users/*/Documents", candidate: "/Volumes/bob/Documents", want: false},
		{entry: "/Users/*", candidate: "/Users/bob", want: true},
		{entry: "/Users/*", candidate: "/Users", want: false},
	}
	for _, tc := range tests {
		within, below := matchDepthSep(tc.entry, tc.candidate, '/', false)
		if within != tc.want {
			t.Errorf("matchDepthSep(%q, %q) = %v, want %v", tc.entry, tc.candidate, within, tc.want)
		}
		if within && below != tc.wantBelow {
			t.Errorf("matchDepthSep(%q, %q) depth = %d, want %d", tc.entry, tc.candidate, below, tc.wantBelow)
		}
	}

	// The plain containment helper must NOT honour wildcards; only protected
	// entries get that treatment.
	if isWithinSep("/Users/*/Documents", "/Users/bob/Documents", '/', false) {
		t.Error("withinDepthSep must treat '*' literally")
	}
}

func TestSplitPathSepStripsWindowsTrailingDotsAndSpaces(t *testing.T) {
	tests := []struct {
		in       string
		sep      rune
		wantVol  string
		wantSegs []string
	}{
		{in: `C:\Windows.\System32`, sep: '\\', wantVol: "C:", wantSegs: []string{"Windows", "System32"}},
		{in: `C:\Windows \System32 `, sep: '\\', wantVol: "C:", wantSegs: []string{"Windows", "System32"}},
		{in: `\\?\C:\Windows`, sep: '\\', wantVol: "C:", wantSegs: []string{"Windows"}},
		{in: `\\srv\share\a`, sep: '\\', wantVol: `\\srv\share`, wantSegs: []string{"a"}},
		{in: `\\srv\C$\Windows`, sep: '\\', wantVol: "C:", wantSegs: []string{"Windows"}},
		// POSIX names may legitimately end in a dot or a space, and must be
		// left alone. (The whole path is TrimSpace'd first, so the trailing
		// space is exercised on an interior segment.)
		{in: "/a/b./c /d", sep: '/', wantVol: "", wantSegs: []string{"a", "b.", "c ", "d"}},
	}
	for _, tc := range tests {
		vol, segs := splitPathSep(tc.in, tc.sep)
		if vol != tc.wantVol || strings.Join(segs, "|") != strings.Join(tc.wantSegs, "|") {
			t.Errorf("splitPathSep(%q) = (%q, %v), want (%q, %v)", tc.in, vol, segs, tc.wantVol, tc.wantSegs)
		}
	}
}

// ---------------------------------------------------------------------------
// ProtectedEntries is the contract the scanner builds its UI badge from.
// ---------------------------------------------------------------------------

func TestProtectedEntriesExposesDepthAndResolutionState(t *testing.T) {
	withProtectedSource(t, []protectedEntry{
		{Path: "/System", Depth: depthSubtree},
		{Path: "/Users/*/Documents", Depth: depthEntryOnly},
	}, nil)

	entries, ok := ProtectedEntries()
	if !ok {
		t.Fatal("ProtectedEntries reported an unresolved list")
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Depth != ProtectionWholeSubtree {
		t.Errorf("/System depth = %d, want ProtectionWholeSubtree", entries[0].Depth)
	}
	if !strings.Contains(entries[1].Path, WildcardSegment) {
		t.Errorf("wildcard entry lost its wildcard: %q", entries[1].Path)
	}

	withProtectedSource(t, nil, errNoHome)
	if _, ok := ProtectedEntries(); ok {
		t.Error("ProtectedEntries must report an unresolved list when the source fails")
	}
}

// errNoHome stands in for a home-directory resolution failure.
var errNoHome = errors.New("no home directory")
