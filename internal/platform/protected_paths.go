package platform

import (
	"strings"
)

// This file holds the *pure* protected-path builders. They take the user's
// home directory (and, on Windows, an environment lookup) as parameters and
// return fully resolved absolute paths. Keeping them free of build tags and of
// direct os.* calls means the whole "no tilde ever escapes into the delete
// guard" property can be unit tested for every target OS from a single host.

// How far below a protected entry the protection reaches.
//
// A blanket "entry plus its entire subtree" rule cannot be applied to every
// entry: on macOS every user file lives under /Users, and on Windows under
// %SystemDrive%\Users, so a blanket rule would refuse every delete the
// application exists to perform. The distinction below is the one the original
// list was reaching for:
//
//   - OS-owned trees (/System, %ProgramFiles%, ...) are protected all the way
//     down — nothing inside them is ever the user's to remove.
//   - Containers of user and application data (/Users, the home directory,
//     ~/Documents, %LOCALAPPDATA%, ...) are protected as *directories*: the
//     directory itself may never be removed, but its contents are exactly what
//     the user came here to clean up.
const (
	// depthSubtree protects the entry and everything beneath it.
	depthSubtree = -1
	// depthEntryOnly protects just the directory itself.
	depthEntryOnly = 0
	// depthEntryAndChildren protects the directory and its immediate children.
	// Used for the roots that hold per-user home directories, so that one
	// user cannot delete another user's entire home while still being able to
	// clean up inside their own.
	depthEntryAndChildren = 1
)

// wildcardSegment is a path segment in a protected entry that matches any
// single real segment.
//
// It exists so the per-user protections (~/Documents, ~/.ssh, %APPDATA%, ...)
// can be applied to *every* account on the machine without enumerating the
// filesystem: "/Users/*/Documents" protects Documents in every home directory,
// including accounts created after the application started.
//
// Two deliberate consequences:
//
//   - A real directory literally named "*" is over-protected. That is legal on
//     APFS (though not on NTFS) and vanishingly rare; refusing to delete it is
//     the safe direction to err in.
//   - The wildcard is only ever used at one position in an entry (immediately
//     below a users root). Matchers may rely on that; see
//     WildcardSegment's use in internal/scanner.
const wildcardSegment = "*"

// WildcardSegment is wildcardSegment exported for callers that need to build
// their own matcher over ProtectedEntries (the scanner does, so that the shield
// badge in the UI agrees with what the delete guard will actually refuse).
const WildcardSegment = wildcardSegment

// protectedEntry is one location the application refuses to modify.
type protectedEntry struct {
	// Path is absolute, cleaned and free of any "~" segment. It may contain
	// wildcardSegment.
	Path string
	// Depth is how far below Path the protection extends: depthSubtree,
	// depthEntryOnly or a positive number of levels.
	Depth int
}

// darwinSystemRoots are OS-owned macOS trees: protected all the way down.
var darwinSystemRoots = []string{
	"/System",
	"/Library",
	"/Applications",
	"/var",
	"/tmp",
	"/etc",
	"/bin",
	"/sbin",
	"/usr",
	"/opt",
	"/dev",
	"/private",
	// Kernel core dumps and the automounted network namespace. Neither is the
	// user's to remove, and both were missing from the original list.
	"/cores",
	"/Network",
}

// darwinVolumeSystemDirs are the directories that mark a mounted volume as
// holding a macOS installation (a bootable clone, a Time Machine target, a
// second internal drive). "/Volumes" alone is protected only one level deep, so
// without these a cloned system volume's entire /System was deletable.
//
// They are matched under "/Volumes/<any volume>/..." and protected all the way
// down, except the Users root which follows the same one-level rule as /Users.
var darwinVolumeSystemDirs = []string{
	"System",
	"Library",
	"Applications",
	"private",
	"usr",
	"bin",
	"sbin",
	"etc",
	"var",
	"opt",
	"cores",
}

// darwinHomeRelative are protected locations expressed relative to a home
// directory. Each directory itself is protected; the files inside are the
// user's own and remain deletable — that is the whole point of the app.
//
// "Library" is listed in its own right, not merely as the parent of the three
// entries below it: without it a single delete of ~/Library would have taken
// out Keychains, Mail, Messages and every "protected" subdirectory at once.
var darwinHomeRelative = []string{
	"Library",
	"Library/Application Support",
	"Library/Preferences",
	"Library/Caches",
	"Music",
	"Movies",
	"Pictures",
	"Documents",
	"Desktop",
	"Downloads",
}

// darwinHomeRelativeSubtree are home-relative locations protected all the way
// down. These hold credentials and small configuration files; none of them is
// ever a legitimate disk-cleanup target, and losing one is unrecoverable in a
// way that a stale cache directory is not.
var darwinHomeRelativeSubtree = []string{
	"Library/Keychains",
	".ssh",
	".gnupg",
	".aws",
	".kube",
}

// unixSystemRoots is the fallback list used on platforms that are not a
// shipping target (Linux, BSD). It exists so the package still behaves
// sensibly — and fails closed — when compiled outside macOS/Windows.
var unixSystemRoots = []string{
	"/bin",
	"/boot",
	"/dev",
	"/etc",
	"/lib",
	"/lib64",
	"/opt",
	"/proc",
	"/root",
	"/run",
	"/sbin",
	"/srv",
	"/sys",
	"/usr",
	"/var",
}

// unixHomeRelative mirrors darwinHomeRelative for the generic Unix fallback.
var unixHomeRelative = []string{
	"Documents",
	"Desktop",
	"Downloads",
	"Music",
	"Pictures",
	"Videos",
	".config",
	".local/share",
	".cache",
}

// unixHomeRelativeSubtree mirrors darwinHomeRelativeSubtree.
var unixHomeRelativeSubtree = []string{
	".ssh",
	".gnupg",
	".aws",
	".kube",
}

// posixLists bundles the four inputs a POSIX protected list is built from.
type posixLists struct {
	systemRoots []string
	// userRoots hold per-user home directories. The home-relative templates
	// below are applied to "<userRoot>/*" as well as to the running user's
	// own home.
	userRoots []string
	// plainUserRoots are per-user roots that get no home template (currently
	// none on macOS; /media, /mnt and /tmp on generic Unix).
	plainUserRoots  []string
	homeRelative    []string
	homeSubtree     []string
	volumeRoots     []string
	volumeSystemDir []string
}

// buildDarwinProtectedPaths returns the macOS protected-path list with every
// home-relative entry expanded against home *and* against every account under
// /Users. Pass an empty home when the home directory could not be determined:
// the caller then gets the static list only (which, thanks to the /Users/*
// templates, still guards every real account), and IsProtected fails closed on
// top of it.
func buildDarwinProtectedPaths(home string) []protectedEntry {
	return buildPosixProtectedPaths(posixLists{
		systemRoots:     darwinSystemRoots,
		userRoots:       []string{"/Users"},
		plainUserRoots:  nil,
		homeRelative:    darwinHomeRelative,
		homeSubtree:     darwinHomeRelativeSubtree,
		volumeRoots:     []string{"/Volumes"},
		volumeSystemDir: darwinVolumeSystemDirs,
	}, home)
}

// buildGenericUnixProtectedPaths is the non-target fallback list.
func buildGenericUnixProtectedPaths(home string) []protectedEntry {
	return buildPosixProtectedPaths(posixLists{
		systemRoots:    unixSystemRoots,
		userRoots:      []string{"/home"},
		plainUserRoots: []string{"/media", "/mnt", "/tmp"},
		homeRelative:   unixHomeRelative,
		homeSubtree:    unixHomeRelativeSubtree,
	}, home)
}

func buildPosixProtectedPaths(lists posixLists, home string) []protectedEntry {
	out := make([]protectedEntry, 0, 64)
	add := func(p string, depth int) {
		if c := posixClean(p); c != "" && c != "/" {
			out = append(out, protectedEntry{Path: c, Depth: depth})
		}
	}

	for _, root := range lists.systemRoots {
		add(root, depthSubtree)
	}
	for _, root := range lists.plainUserRoots {
		add(root, depthEntryAndChildren)
	}

	// Per-user roots: the root and each home directory in it, then the
	// home-relative template applied to *every* account, not only to the
	// account the process happens to be running as. Without the template,
	// "/Users/alice/Documents" was freely deletable by any process that could
	// write there, and a tampered $HOME relocated all of these protections to
	// a directory that does not exist.
	for _, root := range lists.userRoots {
		add(root, depthEntryAndChildren)
		anyUser := root + "/" + wildcardSegment
		for _, rel := range lists.homeRelative {
			add(anyUser+"/"+rel, depthEntryOnly)
		}
		for _, rel := range lists.homeSubtree {
			add(anyUser+"/"+rel, depthSubtree)
		}
	}

	// Mounted volumes: the mount root and each mount point, plus the
	// directories that mark a mount as a system installation.
	for _, root := range lists.volumeRoots {
		add(root, depthEntryAndChildren)
		anyVolume := root + "/" + wildcardSegment
		for _, dir := range lists.volumeSystemDir {
			add(anyVolume+"/"+dir, depthSubtree)
		}
		// A cloned system volume has its own /Users; guard whole home
		// directories on it the same way as on the boot volume.
		add(anyVolume+"/Users", depthEntryAndChildren)
	}

	// The running user's own home, which may legitimately sit outside the
	// users root (a network account, a relocated home). The prefix test runs
	// on the raw value: posixClean would turn a relative string into an
	// absolute one and smuggle a bogus entry into the list.
	if rawHome := strings.TrimSpace(home); strings.HasPrefix(rawHome, "/") {
		h := posixClean(rawHome)
		if h == "" || h == "/" {
			return finaliseEntries(out, '/')
		}
		add(h, depthEntryOnly)
		for _, rel := range lists.homeRelative {
			add(h+"/"+rel, depthEntryOnly)
		}
		for _, rel := range lists.homeSubtree {
			add(h+"/"+rel, depthSubtree)
		}
	}

	return finaliseEntries(out, '/')
}

// posixClean normalises a POSIX path without consulting the host's separator,
// so these lists build identically no matter which OS the tests run on.
func posixClean(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	volume, segs := splitPathSep(p, '/')
	return joinSegments(volume, segs, '/')
}

// windowsSystemRootNames are the OS-owned directories under a Windows system
// drive. They are protected under every spelling the environment offers *and*
// under the literal defaults — see buildWindowsProtectedPaths.
var windowsSystemRootNames = []string{
	"Windows",
	"Program Files",
	"Program Files (x86)",
	"ProgramData",
}

// windowsProfileFolders are the well-known folders inside a user profile. The
// folder itself is protected; its contents are the user's to clean up.
var windowsProfileFolders = []string{
	"AppData",
	`AppData\Roaming`,
	`AppData\Local`,
	`AppData\LocalLow`,
	"Documents",
	"Desktop",
	"Downloads",
	"Pictures",
	"Music",
	"Videos",
}

// windowsProfileSubtrees hold credentials and are protected all the way down.
var windowsProfileSubtrees = []string{
	".ssh",
	".gnupg",
	".aws",
	".kube",
}

// buildWindowsProtectedPaths resolves the Windows protected-path list from the
// environment. getenv is injected so the resolution rules can be tested from
// any host; home is os.UserHomeDir()'s result (empty when it failed).
//
// Environment values are added to the list, never substituted for the literal
// defaults. A machine may well have Windows on D:, but %SystemRoot% is also
// attacker-controllable by whatever launched the process — treating it as an
// override let a shortcut with SystemRoot=D:\Decoy shrink the protected list
// until C:\Windows was freely deletable, with no error to make IsProtected fail
// closed. Protecting the union costs nothing: the extra entries name
// directories that either do not exist or should not be touched anyway.
func buildWindowsProtectedPaths(getenv func(string) string, home string) []protectedEntry {
	env := func(key string) string {
		if getenv == nil {
			return ""
		}
		return strings.TrimSpace(getenv(key))
	}

	out := make([]protectedEntry, 0, 96)
	add := func(p string, depth int) {
		if c := winClean(p); c != "" && !isWindowsVolumeRootLiteral(c) {
			out = append(out, protectedEntry{Path: c, Depth: depth})
		}
	}

	// Every drive that might hold the OS: whatever the environment claims,
	// plus C: as the universal default.
	systemDrives := dedupeStrings([]string{driveOf(env("SystemDrive")), driveOf(env("SystemRoot")), driveOf(env("windir")), "C:"})

	for _, drive := range systemDrives {
		for _, name := range windowsSystemRootNames {
			add(winJoin(drive, name), depthSubtree)
		}
		usersRoot := winJoin(drive, "Users")
		add(usersRoot, depthEntryAndChildren)

		anyUser := winJoin(usersRoot, wildcardSegment)
		for _, folder := range windowsProfileFolders {
			add(winJoin(anyUser, folder), depthEntryOnly)
		}
		for _, folder := range windowsProfileSubtrees {
			add(winJoin(anyUser, folder), depthSubtree)
		}
	}

	// Relocated OS directories named explicitly by the environment.
	for _, key := range []string{"SystemRoot", "windir", "ProgramFiles", "ProgramFiles(x86)", "ProgramW6432", "ProgramData"} {
		add(env(key), depthSubtree)
	}

	// The running user's profile, which may sit outside any Users root.
	userProfile := winClean(firstNonEmpty(strings.TrimSpace(home), env("USERPROFILE")))
	if userProfile != "" && !isWindowsVolumeRootLiteral(userProfile) {
		add(userProfile, depthEntryOnly)
		for _, folder := range windowsProfileFolders {
			add(winJoin(userProfile, folder), depthEntryOnly)
		}
		for _, folder := range windowsProfileSubtrees {
			add(winJoin(userProfile, folder), depthSubtree)
		}
	}
	// Relocated per-user data directories named by the environment.
	for _, key := range []string{"APPDATA", "LOCALAPPDATA", "OneDrive", "USERPROFILE"} {
		add(env(key), depthEntryOnly)
	}

	return finaliseEntries(out, '\\')
}

// driveOf reduces a Windows path to its drive prefix ("D:\Decoy" -> "D:"),
// returning "" for anything that is not on a lettered drive.
func driveOf(p string) string {
	p = winClean(p)
	if len(p) >= 2 && p[1] == ':' && isASCIILetter(p[0]) {
		return strings.ToUpper(p[:1]) + ":"
	}
	return ""
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// winJoin joins Windows path components with a backslash. It returns "" when
// base is empty so that a missing environment variable never produces a
// dangling relative path such as `\Documents`.
func winJoin(base string, rel ...string) string {
	if strings.TrimSpace(base) == "" {
		return ""
	}
	parts := make([]string, 0, len(rel)+1)
	parts = append(parts, strings.TrimSpace(base))
	parts = append(parts, rel...)
	return winClean(strings.Join(parts, `\`))
}

// winClean normalises a Windows path: forward slashes become backslashes,
// repeated separators collapse (the leading pair of a UNC path is preserved)
// and a trailing separator is dropped unless the result would stop being a
// root. It deliberately does not resolve "." or ".." — isWithin does that on
// the segment level for both platforms at once.
func winClean(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, "/", `\`)

	unc := strings.HasPrefix(p, `\\`)
	var b strings.Builder
	b.Grow(len(p))
	prevSep := false
	for i, r := range p {
		if r == '\\' {
			if prevSep && !(unc && i == 1) {
				continue
			}
			prevSep = true
		} else {
			prevSep = false
		}
		b.WriteRune(r)
	}

	out := b.String()
	for len(out) > 1 && strings.HasSuffix(out, `\`) {
		trimmed := strings.TrimSuffix(out, `\`)
		// `C:\` and `\` are roots; keep their trailing separator.
		if trimmed == "" || strings.HasSuffix(trimmed, ":") {
			break
		}
		out = trimmed
	}
	return out
}

// isWindowsVolumeRootLiteral reports whether p is nothing more than a volume
// root such as `C:`, `C:\` or `\\server\share`.
func isWindowsVolumeRootLiteral(p string) bool {
	_, segs := splitPathSep(p, '\\')
	return len(segs) == 0
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// finaliseEntries turns a raw list into the list IsProtected matches against:
// every ancestor of every entry is added at depthEntryOnly, then duplicates are
// merged keeping the widest protection.
func finaliseEntries(in []protectedEntry, sep rune) []protectedEntry {
	return dedupeEntries(withSyntheticAncestors(in, sep))
}

// withSyntheticAncestors adds, for every entry, each of its ancestor
// directories at depthEntryOnly (the filesystem/volume root excepted).
//
// This closes a hole that made the whole per-entry depth scheme pointless:
// "~/Library/Application Support" was protected but "~/Library" was not, and
// depth does not propagate upwards, so one os.RemoveAll("~/Library") destroyed
// Keychains, Mail, Messages and all three "protected" children at once. The
// same shape applied to %USERPROFILE%\AppData, whose Roaming and Local children
// were guarded while AppData itself was not.
func withSyntheticAncestors(in []protectedEntry, sep rune) []protectedEntry {
	out := make([]protectedEntry, 0, len(in)*3)
	out = append(out, in...)
	for _, e := range in {
		volume, segs := splitPathSep(e.Path, sep)
		for k := len(segs) - 1; k >= 1; k-- {
			out = append(out, protectedEntry{
				Path:  joinSegments(volume, segs[:k], sep),
				Depth: depthEntryOnly,
			})
		}
	}
	return out
}

// joinSegments is the inverse of splitPathSep. It never produces a bare root.
func joinSegments(volume string, segs []string, sep rune) string {
	if len(segs) == 0 {
		return volume
	}
	if sep == '\\' {
		return volume + `\` + strings.Join(segs, `\`)
	}
	return volume + "/" + strings.Join(segs, "/")
}

// dedupeEntries removes duplicate paths, keeping the widest protection when the
// same path is listed twice.
func dedupeEntries(in []protectedEntry) []protectedEntry {
	index := make(map[string]int, len(in))
	out := make([]protectedEntry, 0, len(in))
	for _, e := range in {
		if e.Path == "" {
			continue
		}
		if at, ok := index[e.Path]; ok {
			if widerDepth(e.Depth, out[at].Depth) {
				out[at].Depth = e.Depth
			}
			continue
		}
		index[e.Path] = len(out)
		out = append(out, e)
	}
	return out
}

// widerDepth reports whether a protects more than b. depthSubtree is widest.
func widerDepth(a, b int) bool {
	if a == depthSubtree {
		return b != depthSubtree
	}
	if b == depthSubtree {
		return false
	}
	return a > b
}
