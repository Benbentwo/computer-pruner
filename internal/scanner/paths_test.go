package scanner

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Benbentwo/computer-pruner/internal/platform"
)

// abs builds a cleaned absolute path in the running platform's flavour, so the
// same table runs on darwin, windows and linux.
func abs(parts ...string) string {
	return filepath.Join(append([]string{string(filepath.Separator)}, parts...)...)
}

func TestPathPartsContainsIsBoundaryAware(t *testing.T) {
	tests := []struct {
		name   string
		parent string
		child  string
		want   bool
	}{
		{"identical paths contain each other", abs("Users", "bob"), abs("Users", "bob"), true},
		{"direct child", abs("Users", "bob"), abs("Users", "bob", "Documents"), true},
		{"deep descendant", abs("Users"), abs("Users", "bob", "a", "b", "c"), true},
		{"sibling with a shared string prefix is NOT contained", abs("Users", "bob"), abs("Users", "bobby"), false},
		{"prefix within a single segment is NOT contained", abs("var", "log"), abs("var", "logs"), false},
		{"parent is not contained in its child", abs("Users", "bob", "x"), abs("Users", "bob"), false},
		{"unrelated trees", abs("opt"), abs("srv"), false},
		{"trailing separators are ignored", abs("var", "log") + string(filepath.Separator), abs("var", "log"), true},
		{"dot segments are ignored", abs("var", ".", "log"), abs("var", "log", "nginx"), true},
		{"dot dot is resolved", abs("var", "tmp", "..", "log"), abs("var", "log", "nginx"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitPath(tt.parent).contains(splitPath(tt.child))
			if got != tt.want {
				t.Fatalf("contains(%q, %q) = %v, want %v", tt.parent, tt.child, got, tt.want)
			}
		})
	}
}

func TestPathPartsCaseHandlingMatchesPlatform(t *testing.T) {
	parent := abs("Users", "Bob")
	child := abs("users", "bob", "Documents")

	got := splitPath(parent).contains(splitPath(child))
	if got != pathsAreCaseInsensitive {
		t.Fatalf("contains(%q, %q) = %v, want %v on this platform",
			parent, child, got, pathsAreCaseInsensitive)
	}
}

func TestPathPartsChildMatchesAFullSplit(t *testing.T) {
	parent := abs("Users", "bob")
	for _, name := range []string{"Documents", "a b c", ".hidden", "weird..name"} {
		derived := splitPath(parent).child(name)
		full := splitPath(filepath.Join(parent, name))

		if derived.key() != full.key() {
			t.Fatalf("child(%q) = %v, want the same as splitting %q => %v",
				name, derived.segs, filepath.Join(parent, name), full.segs)
		}
	}

	// Deriving twice from the same parent must not let one child clobber the
	// other through a shared backing array.
	p := splitPath(parent)
	a, b := p.child("alpha"), p.child("beta")
	if a.segs[len(a.segs)-1] != "alpha" || b.segs[len(b.segs)-1] != "beta" {
		t.Fatalf("child() results alias each other: %v / %v", a.segs, b.segs)
	}
	if len(p.segs) != 2 {
		t.Fatalf("child() mutated the parent: %v", p.segs)
	}
}

func TestPathPartsIsRoot(t *testing.T) {
	root := string(filepath.Separator)
	if !splitPath(root).isRoot() {
		t.Fatalf("%q should be recognised as a filesystem root", root)
	}
	if splitPath(abs("var")).isRoot() {
		t.Fatal("/var is not a filesystem root")
	}
}

// TestProtectionMatcherHonoursEntryDepth is the regression test for the UI
// badge: the scanner used to flag a node protected only on an *exact* string
// match against platform.ProtectedPaths(), throwing away each entry's depth. It
// therefore disagreed with the delete guard in both directions — no shield on
// /System/Library (which the guard refuses) and a shield on ~/Documents whose
// children the guard allows.
func TestProtectionMatcherHonoursEntryDepth(t *testing.T) {
	m := newProtectionMatcher([]platform.ProtectedEntry{
		{Path: abs("System"), Depth: platform.ProtectionWholeSubtree},
		{Path: abs("Users"), Depth: 1},
		{Path: abs("Users", "bob"), Depth: 0},
		{Path: abs("Users", "bob", "Documents"), Depth: 0},
		{Path: "", Depth: 0},
		{Path: "   ", Depth: 0},
	})

	cases := map[string]bool{
		// Subtree entries reach all the way down.
		abs("System"): true,
		abs("System") + string(filepath.Separator):    true,
		abs("System", "Library"):                      true,
		abs("System", "Library", "CoreServices", "x"): true,
		abs("Systemx"):            false,
		abs("Systemx", "Library"): false,
		// depth 1: the root and each home, but not their contents.
		abs("Users"):                       true,
		abs("Users", "carol"):              true,
		abs("Users", "carol", "Downloads"): false,
		// depth 0: the directory itself only.
		abs("Users", "bob"):                            true,
		abs("Users", "bob", "Documents"):               true,
		abs("Users", "bob", "Documents", "report.pdf"): false,
		abs("Users", "bob", "scratch.iso"):             false,
		abs("tmp", "unrelated"):                        false,
	}
	for path, want := range cases {
		if got := m.matches(path); got != want {
			t.Errorf("matches(%q) = %v, want %v", path, got, want)
		}
	}

	if newProtectionMatcher(nil).matches(abs("anything")) {
		t.Fatal("an empty matcher must never report protection")
	}
}

// TestProtectionMatcherHandlesWildcardEntries covers the templated per-account
// entries ("/Users/*/Documents") that make the guard apply to every home
// directory, not just the one the process is running as.
func TestProtectionMatcherHandlesWildcardEntries(t *testing.T) {
	w := platform.WildcardSegment
	m := newProtectionMatcher([]platform.ProtectedEntry{
		{Path: abs("Users"), Depth: 1},
		{Path: abs("Users", w), Depth: 0},
		{Path: abs("Users", w, "Documents"), Depth: 0},
		{Path: abs("Users", w, ".ssh"), Depth: platform.ProtectionWholeSubtree},
		{Path: abs("Volumes", w, "System"), Depth: platform.ProtectionWholeSubtree},
	})

	cases := map[string]bool{
		abs("Users", "alice"):                            true,
		abs("Users", "alice", "Documents"):               true,
		abs("Users", "alice", "Documents", "thesis.pdf"): false,
		abs("Users", "alice", ".ssh"):                    true,
		abs("Users", "alice", ".ssh", "id_ed25519"):      true,
		abs("Users", "alice", "Downloads", "big.iso"):    false,
		abs("Volumes", "Backup HD", "System", "Library"): true,
		abs("Volumes", "Backup HD", "old-snapshot"):      false,
	}
	for path, want := range cases {
		if got := m.matches(path); got != want {
			t.Errorf("matches(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestProtectionMatcherAgreesWithTheDeleteGuard is the property that matters:
// whatever the badge says, the guard must say the same thing. Any drift between
// the two implementations shows up here.
func TestProtectionMatcherAgreesWithTheDeleteGuard(t *testing.T) {
	entries, ok := platform.ProtectedEntries()
	if !ok || len(entries) == 0 {
		t.Skip("this host's protected list could not be resolved")
	}
	m := newProtectionMatcher(entries)

	probes := []string{
		abs("System"),
		abs("System", "Library"),
		abs("usr", "lib", "dyld"),
		abs("etc", "hosts"),
		abs("Users"),
		abs("Users", "somebody"),
		abs("Users", "somebody", "Documents"),
		abs("Users", "somebody", "Documents", "big.mov"),
		abs("Users", "somebody", ".ssh", "id_rsa"),
		abs("home", "somebody", "Documents"),
		abs("home", "somebody", "Documents", "big.mov"),
		abs("tmp", "scratch", "thing.bin"),
		abs("definitely-not-protected", "x"),
	}
	for _, p := range probes {
		guard, _ := platform.IsProtected(p)
		if badge := m.matches(p); badge != guard {
			t.Errorf("scanner badge for %q = %v but platform.IsProtected = %v; "+
				"the UI signal and the delete policy must not disagree", p, badge, guard)
		}
	}
}

func TestExclusionSetSkipsSubtrees(t *testing.T) {
	set := newExclusionSet([]string{abs("var", "log")})

	cases := map[string]bool{
		abs("var", "log"):                     true,
		abs("var", "log", "nginx"):            true,
		abs("var", "log", "nginx", "acc.log"): true,
		abs("var", "logs"):                    false,
		abs("var"):                            false,
		abs("var", "lib"):                     false,
	}

	for path, want := range cases {
		if got := set.excludes(path); got != want {
			t.Errorf("excludes(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestExclusionSetIgnoresUnusableEntries(t *testing.T) {
	set := newExclusionSet([]string{
		"",
		"    ",
		"relative/dir",
		"~/Library",
		string(filepath.Separator), // a bare root would suppress the entire scan
	})

	if !set.empty() {
		t.Fatalf("expected every entry to be rejected, got %d roots", len(set.roots))
	}
	if set.excludes(abs("anywhere", "at", "all")) {
		t.Fatal("an empty exclusion set must never exclude anything")
	}
}

func TestExclusionSetForRootDropsSwallowingEntries(t *testing.T) {
	set := newExclusionSet([]string{
		abs("Users"),                     // contains the scan root
		abs("Users", "bob"),              // is the scan root
		abs("Users", "bob", "Caches"),    // below the scan root: kept
		abs("Users", "bobby"),            // unrelated sibling: kept
		abs("Users", "bob", "..", "amy"), // resolves to a sibling: kept
	})

	scoped := set.forRoot(abs("Users", "bob"))

	if scoped.excludes(abs("Users", "bob", "Documents")) {
		t.Fatal("an exclusion covering the scan root must be dropped")
	}
	if !scoped.excludes(abs("Users", "bob", "Caches", "x")) {
		t.Fatal("an exclusion below the scan root must survive")
	}
	if !scoped.excludes(abs("Users", "bobby")) {
		t.Fatal("an unrelated exclusion must survive")
	}
	if !scoped.excludes(abs("Users", "amy")) {
		t.Fatal("a sibling exclusion must survive")
	}

	// The original set is untouched.
	if !set.excludes(abs("Users", "bob", "Documents")) {
		t.Fatal("forRoot must not mutate the set it was called on")
	}
}

func TestNilExclusionSetIsInert(t *testing.T) {
	var set *exclusionSet
	if !set.empty() {
		t.Fatal("a nil exclusion set must report empty")
	}
	if set.excludes(abs("var")) {
		t.Fatal("a nil exclusion set must never exclude anything")
	}
}

func TestSplitPathNormalisesSeparators(t *testing.T) {
	// Whatever the platform, duplicated separators must collapse.
	sep := string(filepath.Separator)
	messy := sep + "var" + sep + sep + "log" + sep
	parts := splitPath(messy)
	if strings.Join(parts.segs, "/") != "var/log" {
		t.Fatalf("splitPath(%q).segs = %q, want [var log]", messy, parts.segs)
	}
}
