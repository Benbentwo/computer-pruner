// Package platform isolates every OS-specific behaviour ComputerPruner needs:
// the list of locations that must never be modified, revealing a path in the
// native file manager, previewing a file, reading the system theme and moving
// a file to the platform trash/recycle bin.
//
// The exported API is identical on every OS; the implementations live in
// build-tagged files (platform_darwin.go, platform_windows.go and
// platform_other.go, the last one being a safe fallback that merely keeps the
// tree compiling and testable on Linux).
//
// # Safety model
//
// ProtectedPaths always returns absolute, cleaned, fully expanded paths. A
// literal "~" must never leave this package: an unexpanded tilde used to make
// the delete guard silently compare "~/Documents" against
// "/Users/example/Documents" and never fire.
//
// IsProtected is the only function callers should use to decide whether a path
// may be destroyed. It fails closed: anything it cannot confidently classify —
// an empty path, a relative path, a tilde path, a filesystem or volume root, a
// machine whose home directory cannot be resolved — is reported as protected.
//
// How far a protected entry reaches is per-entry, see protected_paths.go:
// OS-owned trees are protected all the way down, while containers of user data
// (/Users, the home directory, ~/Documents, %LOCALAPPDATA%, ...) protect the
// directory itself but not the files inside it, which are exactly what the
// application exists to clean up.
package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// userHomeDir is os.UserHomeDir behind a variable so tests can pin a synthetic
// home directory without touching the environment of the test process.
var userHomeDir = os.UserHomeDir

// protectedSource resolves the protected-path list for the running OS. It is a
// variable so tests can substitute a synthetic list (including one that
// reports a resolution failure, to exercise the fail-closed branch).
//
// It returns the best list it can build even when it also returns an error:
// callers that only display the list (the scanner) keep working, while
// IsProtected treats the error as "refuse everything".
var protectedSource = platformProtectedPaths

// ProtectedPaths returns the locations that must never be deleted or moved.
//
// Every element is absolute, cleaned and platform-correct, and contains no "~"
// segment. When the user's home directory cannot be resolved the static system
// list is returned on its own — and IsProtected then refuses every path rather
// than letting a home-directory path slip through unguarded.
func ProtectedPaths() []string {
	entries, _ := protectedSource()
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.Path)
	}
	return paths
}

// ProtectedEntry is one protected location together with how far below it the
// protection reaches.
//
// Depth is ProtectionWholeSubtree for OS-owned trees, 0 when only the directory
// itself is protected, or a positive number of levels. Path may contain
// WildcardSegment, which matches any single path segment.
//
// It exists so that a caller which must classify millions of paths (the
// scanner, deciding whether to show a shield in the UI) can build its own index
// over the same policy IsProtected enforces, instead of re-implementing a
// weaker one.
type ProtectedEntry struct {
	Path  string `json:"path"`
	Depth int    `json:"depth"`
}

// ProtectionWholeSubtree is the ProtectedEntry.Depth value meaning "this entry
// and everything beneath it".
const ProtectionWholeSubtree = depthSubtree

// ProtectedEntries returns the protected locations with their depths. The
// second result reports whether the list could be resolved completely; when it
// is false the list is still usable for display but IsProtected refuses every
// path.
func ProtectedEntries() ([]ProtectedEntry, bool) {
	entries, err := protectedSource()
	out := make([]ProtectedEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, ProtectedEntry{Path: e.Path, Depth: e.Depth})
	}
	return out, err == nil
}

// GetProtectedPaths is the original name of ProtectedPaths.
//
// Deprecated: use ProtectedPaths. Kept so existing callers keep compiling; it
// now returns fully expanded absolute paths rather than a mix of absolute and
// "~/"-prefixed strings.
func GetProtectedPaths() []string {
	return ProtectedPaths()
}

// IsProtected reports whether path may not be modified, together with a
// human-readable reason suitable for surfacing in the UI. The reason is empty
// if and only if the first return value is false.
//
// A path is protected when it is a protected entry, when it lies beneath one,
// or when resolving it produces something that does. Comparison happens on
// cleaned path segments, so "/Users/bobby" is not considered to be inside
// "/Users/bob". Symlinks are resolved first, so a link pointing into a
// protected tree cannot be used as a bypass.
func IsProtected(path string) (bool, string) {
	protected, err := protectedSource()
	if err != nil {
		return true, fmt.Sprintf(
			"cannot determine this machine's protected locations (%v); refusing to modify %q",
			err, path,
		)
	}
	return isProtectedAgainst(path, protected)
}

// isProtectedAgainst is IsProtected against an explicit list, so the policy can
// be tested without depending on the host's real filesystem layout.
func isProtectedAgainst(input string, protected []protectedEntry) (bool, string) {
	raw := strings.TrimSpace(input)
	if refuse, reason := refuseOnSyntax(raw, filepath.Separator); refuse {
		return true, reason
	}

	cleaned := filepath.Clean(raw)
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return true, fmt.Sprintf("cannot resolve %q to an absolute path (%v)", input, err)
	}
	cleaned = abs

	candidates := resolutionCandidates(cleaned)

	// An 8.3 short name ("C:\PROGRA~1\...") names the same directory as its
	// long form but shares no segment with it. Today filepath.EvalSymlinks
	// happens to expand it, which is why one of the candidates is normally
	// clean — but when resolution fails (a missing leaf, an unreadable
	// ancestor, a path past MAX_PATH) the alias is the only spelling left and
	// it matches nothing. Refuse rather than guess.
	if filepath.Separator == '\\' && !anyCandidateFreeOfShortNames(candidates) {
		return true, fmt.Sprintf(
			"%q contains an unresolved 8.3 short name; only fully resolved paths may be modified",
			input,
		)
	}

	for _, candidate := range candidates {
		if isFilesystemRoot(candidate) {
			return true, fmt.Sprintf("%q is a filesystem or volume root", candidate)
		}
		for _, entry := range protected {
			entryPath := strings.TrimSpace(entry.Path)
			if entryPath == "" {
				continue
			}

			within, below := matchDepth(entryPath, candidate)
			if !within {
				continue
			}
			if entry.Depth != depthSubtree && below > entry.Depth {
				// Inside a container of user data, but deeper than the
				// directory this entry actually protects.
				continue
			}
			if below == 0 {
				return true, fmt.Sprintf("%q is a protected system location", candidate)
			}
			return true, fmt.Sprintf("%q is inside the protected location %q", candidate, entryPath)
		}
	}

	return false, ""
}

// resolutionCandidates returns every spelling of cleaned that has to be
// checked against the protected list:
//
//  1. the cleaned path as given;
//  2. the path with all symlinks resolved, when it exists;
//  3. the path rebuilt on top of its resolved parent, which catches a path
//     that does not exist yet (or a dangling link) whose *ancestor* is a
//     symlink into a protected tree.
//
// filepath.EvalSymlinks failing is not an error: the cleaned path is still
// compared, so a failure can only ever add checks, never remove one.
func resolutionCandidates(cleaned string) []string {
	candidates := []string{cleaned}
	add := func(p string) {
		if p == "" {
			return
		}
		for _, existing := range candidates {
			if existing == p {
				return
			}
		}
		candidates = append(candidates, p)
	}

	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		add(filepath.Clean(resolved))
	}

	// Walk up until an ancestor does resolve, then rebuild the original tail on
	// top of it. Resolving only the immediate parent left
	// "/tmp/link/missing-a/missing-b" unclassified whenever more than one
	// trailing component was absent, so a path whose ancestor is a symlink into
	// a protected tree could be classified as allowed.
	tail := make([]string, 0, maxAncestorResolutionDepth)
	ancestor := cleaned
	for i := 0; i < maxAncestorResolutionDepth; i++ {
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			break
		}
		tail = append(tail, filepath.Base(ancestor))
		ancestor = parent

		resolvedAncestor, err := filepath.EvalSymlinks(ancestor)
		if err != nil {
			continue
		}
		rebuilt := filepath.Clean(resolvedAncestor)
		for j := len(tail) - 1; j >= 0; j-- {
			rebuilt = filepath.Join(rebuilt, tail[j])
		}
		add(rebuilt)
		break
	}

	return candidates
}

// maxAncestorResolutionDepth bounds how far resolutionCandidates walks up
// looking for an ancestor that exists. Real paths are far shallower; the cap
// only exists so a pathological input cannot spin.
const maxAncestorResolutionDepth = 64

// isWithin reports whether child is parent itself, or lies anywhere beneath it,
// using the running platform's separator and case sensitivity.
func isWithin(parent, child string) bool {
	within, _ := withinDepth(parent, child)
	return within
}

// withinDepth reports whether child is parent or lies beneath it and, when it
// does, how many levels below parent the child sits (0 when they are the same
// location). Wildcards are not honoured; use matchDepth for protected entries.
func withinDepth(parent, child string) (bool, int) {
	return withinDepthSep(parent, child, filepath.Separator, pathsAreCaseInsensitive)
}

// matchDepth is withinDepth for a protected *entry*, whose path may contain
// wildcardSegment.
func matchDepth(entry, candidate string) (bool, int) {
	return matchDepthSep(entry, candidate, filepath.Separator, pathsAreCaseInsensitive)
}

// matchDepthSep is the separator- and case-parameterised form of matchDepth, so
// the Windows rules can be exercised from a POSIX test host.
func matchDepthSep(entry, candidate string, sep rune, ignoreCase bool) (bool, int) {
	return containDepth(entry, candidate, sep, ignoreCase, true)
}

// equalPath reports whether two paths denote the same location.
func equalPath(a, b string) bool {
	within, below := withinDepth(a, b)
	return within && below == 0
}

// isWithinSep is the separator- and case-parameterised form of isWithin.
func isWithinSep(parent, child string, sep rune, ignoreCase bool) bool {
	within, _ := withinDepthSep(parent, child, sep, ignoreCase)
	return within
}

// withinDepthSep is the separator- and case-parameterised core of the
// containment test, so the Windows and POSIX rules can both be exercised from a
// single test host.
//
// Algorithm: split both paths into (volume, segments), normalising away empty
// segments (duplicate separators, trailing separator), "." segments and ".."
// segments (clamped at the root, matching filepath.Clean on absolute paths).
// The volumes must match, the child must have at least as many segments as the
// parent, and every parent segment must equal the child segment at the same
// index. Comparing whole segments — never raw string prefixes — is what stops
// "/Users/bobby" from being treated as living inside "/Users/bob". The depth is
// simply how many segments the child has in excess of the parent.
func withinDepthSep(parent, child string, sep rune, ignoreCase bool) (bool, int) {
	return containDepth(parent, child, sep, ignoreCase, false)
}

// containDepth is the shared core. When wildcard is true a parent segment equal
// to wildcardSegment matches any child segment; the volume is always compared
// literally.
func containDepth(parent, child string, sep rune, ignoreCase, wildcard bool) (bool, int) {
	parentVol, parentSegs := splitPathSep(parent, sep)
	childVol, childSegs := splitPathSep(child, sep)

	if !equalSegment(parentVol, childVol, ignoreCase) {
		return false, 0
	}
	if len(childSegs) < len(parentSegs) {
		return false, 0
	}
	for i := range parentSegs {
		if wildcard && parentSegs[i] == wildcardSegment {
			continue
		}
		if !equalSegment(parentSegs[i], childSegs[i], ignoreCase) {
			return false, 0
		}
	}
	return true, len(childSegs) - len(parentSegs)
}

// refuseOnSyntax rejects, on spelling alone, every path the guard cannot
// confidently classify. It is separator-parameterised so the Windows rules are
// testable from a POSIX host.
func refuseOnSyntax(raw string, sep rune) (bool, string) {
	if raw == "" {
		return true, "an empty path cannot be resolved and is never safe to modify"
	}
	if raw == "~" || strings.HasPrefix(raw, "~/") || strings.HasPrefix(raw, `~\`) {
		return true, fmt.Sprintf(
			"%q is an unexpanded home-relative path; only fully resolved absolute paths may be modified",
			raw,
		)
	}
	if !isAbsSep(raw, sep) {
		return true, fmt.Sprintf(
			"%q is not an absolute path on a recognised volume; only fully resolved absolute paths may be modified",
			raw,
		)
	}
	return false, ""
}

// isAbsSep reports whether p is fully qualified for the given separator.
//
// On Windows this deliberately replaces filepath.IsAbs, which answers true for
// device and volume-GUID spellings such as `\\?\GLOBALROOT\Device\...` and
// `\\?\Volume{...}\...`. Those name a real directory that os.RemoveAll will
// happily delete, but they share no volume with any protected entry, so the
// guard matched nothing and let them through. Anything that does not reduce to
// a lettered drive or a UNC share is refused.
func isAbsSep(p string, sep rune) bool {
	p = strings.TrimSpace(p)
	if sep != '\\' {
		return strings.HasPrefix(p, "/")
	}

	q := stripWindowsDevicePrefix(strings.ReplaceAll(p, "/", `\`))
	if strings.HasPrefix(q, `\\`) {
		parts := strings.SplitN(q[2:], `\`, 3)
		return len(parts) >= 2 && parts[0] != "" && parts[1] != ""
	}
	return len(q) >= 3 && isASCIILetter(q[0]) && q[1] == ':' && q[2] == '\\'
}

// anyCandidateFreeOfShortNames reports whether at least one spelling of the
// path is free of 8.3 aliases.
func anyCandidateFreeOfShortNames(candidates []string) bool {
	for _, c := range candidates {
		if !hasWindowsShortNameSegment(c) {
			return true
		}
	}
	return false
}

// hasWindowsShortNameSegment reports whether any segment of p looks like an 8.3
// alias.
func hasWindowsShortNameSegment(p string) bool {
	_, segs := splitPathSep(p, '\\')
	for _, s := range segs {
		if windowsShortNameSegment(s) {
			return true
		}
	}
	return false
}

// windowsShortNameSegment reports whether seg has the shape of an 8.3 alias:
// a non-empty stem, a tilde, and a decimal index ("PROGRA~1", "PROGRA~1.TXT").
func windowsShortNameSegment(seg string) bool {
	tilde := strings.IndexByte(seg, '~')
	if tilde <= 0 || tilde == len(seg)-1 {
		return false
	}
	digits := 0
	for i := tilde + 1; i < len(seg); i++ {
		c := seg[i]
		if c == '.' {
			break
		}
		if c < '0' || c > '9' {
			return false
		}
		digits++
	}
	return digits > 0
}

// splitPathSep splits p into its volume name and its normalised segments.
func splitPathSep(p string, sep rune) (string, []string) {
	p = strings.TrimSpace(p)
	if sep == '\\' {
		// Windows accepts both separators; normalise before splitting.
		p = strings.ReplaceAll(p, "/", `\`)
	}

	volume, rest := splitVolume(p, sep)

	raw := strings.Split(rest, string(sep))
	segs := make([]string, 0, len(raw))
	for _, s := range raw {
		switch s {
		case "", ".":
			// Duplicate separators, a trailing separator and "." add nothing.
			continue
		case "..":
			if len(segs) > 0 {
				segs = segs[:len(segs)-1]
			}
			// Clamped at the root: filepath.Clean("/..") is "/".
			continue
		}
		if sep == '\\' {
			// Windows silently strips trailing dots and spaces when it opens a
			// path, so `C:\Windows.\System32` and `C:\Windows \System32` are
			// both C:\Windows\System32 on disk. Comparing the untrimmed
			// spelling matched no protected entry.
			s = strings.TrimRight(s, ". ")
			if s == "" {
				continue
			}
		}
		segs = append(segs, s)
	}
	return volume, segs
}

// splitVolume separates a Windows volume prefix (`C:` or `\\server\share`)
// from the rest of the path. POSIX paths have no volume.
//
// The volume is *canonicalised*, not merely extracted. Windows accepts several
// spellings of the same drive, and every one of them used to defeat the whole
// protected list because the volume comparison in containDepth is exact:
//
//	\\?\C:\Windows           extended-length prefix
//	\\.\C:\Windows           device-namespace prefix
//	\??\C:\Windows           NT object-manager prefix
//	\\localhost\C$\Windows   administrative share
//	\\?\UNC\srv\share\x      extended-length UNC
//
// filepath.EvalSymlinks does not rescue these: normVolumeName in
// path/filepath/symlink_windows.go returns any volume longer than two
// characters verbatim, so the alternate prefix survives resolution, while
// os.RemoveAll accepts it happily.
func splitVolume(p string, sep rune) (string, string) {
	if sep != '\\' {
		return "", p
	}

	p = stripWindowsDevicePrefix(p)

	if strings.HasPrefix(p, `\\`) {
		parts := strings.SplitN(p[2:], `\`, 3)
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			volume := `\\` + parts[0] + `\` + parts[1]
			// `\\host\C$` is drive C: of some machine. Which machine cannot be
			// decided from the string, so treat it as the local drive: that
			// over-protects a genuinely remote admin share, which is the safe
			// direction, and closes the local bypass.
			if drive := adminShareDrive(parts[1]); drive != "" {
				volume = drive
			}
			if len(parts) == 3 {
				return volume, `\` + parts[2]
			}
			return volume, ""
		}
		return p, ""
	}

	if len(p) >= 2 && p[1] == ':' && isASCIILetter(p[0]) {
		return p[:2], p[2:]
	}
	return "", p
}

// stripWindowsDevicePrefix removes an extended-length, device or NT
// object-manager prefix, mapping `\\?\UNC\srv\share` back to `\\srv\share`.
func stripWindowsDevicePrefix(p string) string {
	for _, prefix := range []string{`\\?\`, `\\.\`, `\??\`} {
		if len(p) <= len(prefix) || !strings.EqualFold(p[:len(prefix)], prefix) {
			continue
		}
		rest := p[len(prefix):]
		if len(rest) > 4 && strings.EqualFold(rest[:4], `UNC\`) {
			return `\\` + rest[4:]
		}
		return rest
	}
	return p
}

// adminShareDrive maps an administrative share name ("C$") to its drive ("C:"),
// returning "" for an ordinary share.
func adminShareDrive(share string) string {
	if len(share) == 2 && share[1] == '$' && isASCIILetter(share[0]) {
		return strings.ToUpper(share[:1]) + ":"
	}
	return ""
}

// isFilesystemRoot reports whether p is "/", a drive root such as `C:\`, or a
// UNC share root. Those are refused outright.
func isFilesystemRoot(p string) bool {
	_, segs := splitPathSep(p, filepath.Separator)
	return len(segs) == 0
}

// equalSegment compares two path segments under the platform's case rule and
// under Unicode normalisation.
//
// macOS filesystems are normalisation-insensitive: os.UserHomeDir() returns
// $HOME in whatever form the login record holds (usually NFC) while names read
// back from readdir on HFS+ arrive as NFD. "José" spelled NFC and NFD are
// byte-different and case folding does not reconcile them, so for any account
// with a non-ASCII short name every home-relative protection silently
// evaporated whenever the path came from the scanner's walk rather than from
// the environment.
//
// The ASCII fast path means the normaliser is never invoked for the paths that
// make up the overwhelming majority of a filesystem walk.
func equalSegment(a, b string, ignoreCase bool) bool {
	if equalSegmentExact(a, b, ignoreCase) {
		return true
	}
	if isASCII(a) && isASCII(b) {
		return false
	}
	return equalSegmentExact(norm.NFC.String(a), norm.NFC.String(b), ignoreCase)
}

func equalSegmentExact(a, b string, ignoreCase bool) bool {
	if ignoreCase {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
