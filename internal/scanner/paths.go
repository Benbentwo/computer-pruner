package scanner

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/Benbentwo/computer-pruner/internal/platform"
	"golang.org/x/text/unicode/norm"
)

// This file holds the scanner's path-containment primitives. They mirror the
// approach used by internal/platform's protected-path guard: a path is compared
// as a list of whole segments, never as a raw string prefix. Naive prefix
// matching would make "/Users/bobby" look like it lives inside "/Users/bob",
// which is wrong both when refusing a delete and when skipping an excluded
// subtree.

// pathParts is a path decomposed into its volume (empty on POSIX) and its
// normalised segments.
type pathParts struct {
	volume string
	segs   []string
}

// splitPath decomposes p using the running platform's separator, dropping empty
// and "." segments and resolving ".." the way filepath.Clean does on an
// absolute path (clamped at the root).
func splitPath(p string) pathParts {
	p = strings.TrimSpace(p)
	if filepath.Separator == '\\' {
		// Windows accepts both separators; normalise before splitting.
		p = strings.ReplaceAll(p, "/", `\`)
	}

	volume := filepath.VolumeName(p)
	rest := p[len(volume):]

	raw := strings.Split(rest, string(filepath.Separator))
	segs := make([]string, 0, len(raw))
	for _, s := range raw {
		switch s {
		case "", ".":
			continue
		case "..":
			if len(segs) > 0 {
				segs = segs[:len(segs)-1]
			}
		default:
			segs = append(segs, s)
		}
	}

	return pathParts{volume: volume, segs: segs}
}

// contains reports whether other is parent itself or lies anywhere beneath it.
func (parent pathParts) contains(other pathParts) bool {
	if !equalPathSegment(parent.volume, other.volume) {
		return false
	}
	if len(other.segs) < len(parent.segs) {
		return false
	}
	for i := range parent.segs {
		if !equalPathSegment(parent.segs[i], other.segs[i]) {
			return false
		}
	}
	return true
}

// child returns the parts of a direct child of p named name.
//
// A walk visits far more entries than directories, so deriving each child from
// its parent's already-split parts avoids re-parsing the full path string
// millions of times. The result never aliases p's backing array.
func (p pathParts) child(name string) pathParts {
	segs := make([]string, len(p.segs)+1)
	copy(segs, p.segs)
	segs[len(p.segs)] = name
	return pathParts{volume: p.volume, segs: segs}
}

// isRoot reports whether the path is nothing but a filesystem or volume root
// ("/", `C:\`, `\\server\share`).
func (p pathParts) isRoot() bool {
	return len(p.segs) == 0
}

// key returns a comparable form of the path, suitable as a map key. On
// case-insensitive platforms the segments are folded so that a lookup of
// "/users/ben" finds an entry stored as "/Users/Ben".
func (p pathParts) key() string {
	var b strings.Builder
	b.Grow(len(p.volume) + 8*len(p.segs))
	b.WriteString(foldPathSegment(p.volume))
	for _, s := range p.segs {
		b.WriteByte(0)
		b.WriteString(foldPathSegment(s))
	}
	return b.String()
}

func equalPathSegment(a, b string) bool {
	if pathsAreCaseInsensitive {
		return strings.EqualFold(normalisePathSegment(a), normalisePathSegment(b))
	}
	return normalisePathSegment(a) == normalisePathSegment(b)
}

func foldPathSegment(s string) string {
	if pathsAreCaseInsensitive {
		return strings.ToLower(normalisePathSegment(s))
	}
	return normalisePathSegment(s)
}

// normalisePathSegment puts a segment into NFC, so that the "é" macOS returns
// from readdir (NFD) and the "é" the same name carries in a protected entry
// (NFC) compare equal. internal/platform does the same thing for exactly the
// same reason; see equalSegment there.
//
// The ASCII fast path keeps the normaliser out of the hot loop for the paths
// that make up almost all of a filesystem walk.
func normalisePathSegment(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return norm.NFC.String(s)
		}
	}
	return s
}

// maxInlinePathSegments bounds the stack-allocated offset scratch in
// keyPrefixes. Real paths are far shallower; deeper ones just fall back to the
// heap.
const maxInlinePathSegments = 32

// keyPrefixes appends the same string key() produces into buf, and returns it
// together with the byte offset at which each ancestor prefix ends.
// offsets[k] is the end of the key for the first k segments, so
// key[:offsets[k]] is a ready-made lookup key for that ancestor — no extra
// allocation per ancestor probe.
//
// substitute, when >= 0, replaces the segment at that index with the wildcard,
// which is how the templated per-account entries are probed.
func (p pathParts) keyPrefixes(offsets []int, substitute int) (string, []int) {
	var b strings.Builder
	b.Grow(len(p.volume) + 8*len(p.segs))
	b.WriteString(foldPathSegment(p.volume))

	offsets = append(offsets[:0], b.Len())
	for i, s := range p.segs {
		b.WriteByte(0)
		if i == substitute {
			b.WriteString(platform.WildcardSegment)
		} else {
			b.WriteString(foldPathSegment(s))
		}
		offsets = append(offsets, b.Len())
	}
	return b.String(), offsets
}

// protectionMatcher answers "would the delete guard refuse this path?" for the
// millions of entries a whole-disk walk visits.
//
// It exists because the scanner used to badge a node as protected on an *exact*
// match against platform.ProtectedPaths(), discarding each entry's depth. The
// UI signal and the real policy then disagreed in both directions: /System
// showed no shield on /System/Library even though a delete of it is refused,
// while ~/Documents showed a shield whose children were freely deletable. This
// matcher consumes platform.ProtectedEntries() — the same paths *and depths*
// IsProtected enforces — so the badge and the refusal cannot drift apart.
//
// Cost per query is a handful of map lookups against a key string built once
// and then sliced, not a linear scan of the entry list:
//   - subtree entries are matched by probing each ancestor prefix, bounded by
//     the longest subtree entry (three or four segments in practice);
//   - depth-limited entries can only match the path itself or its nearest
//     ancestors, so they need two lookups;
//   - templated entries cost one extra key build, and only for paths that
//     actually sit under a root that has templated entries beneath it.
//
// This is not free. Measured against the old exact-match lookup on a 5-segment
// path (Xeon @2.8GHz, Go 1.25, including the caller's child() allocation):
//
//	old exact match          181 ns/op   2 allocs/op
//	new, outside /Users      241 ns/op   2 allocs/op
//	new, under /Users        436 ns/op   3 allocs/op
//
// On a two-million-entry scan that is roughly half a second of extra CPU
// against tens of seconds of I/O, in exchange for a shield badge that means
// what it says. If this ever shows up in a profile, the fix is to propagate
// the match state from parent to child during the walk rather than
// re-classifying each node from scratch.
type protectionMatcher struct {
	// subtree holds entries protecting themselves and everything below.
	subtree map[string]struct{}
	// limited maps an entry to how many levels below it stay protected (0 or
	// more, never depthSubtree).
	limited map[string]int
	// maxSubtreeSegs bounds the ancestor walk for subtree entries.
	maxSubtreeSegs int
	// maxLimitedDepth bounds how far above a path a depth-limited entry can
	// sit and still cover it.
	maxLimitedDepth int
	// wildcardPositions lists the segment indices at which any entry carries a
	// wildcard. Empty for a list with no templated entries.
	wildcardPositions []int
	// wildcardRoots holds the one-segment key prefix of every templated entry,
	// so a path that cannot possibly match one is rejected with a single map
	// lookup instead of building a substituted key.
	wildcardRoots map[string]bool
}

func newProtectionMatcher(entries []platform.ProtectedEntry) *protectionMatcher {
	m := &protectionMatcher{
		subtree:       make(map[string]struct{}, len(entries)),
		limited:       make(map[string]int, len(entries)),
		wildcardRoots: make(map[string]bool),
	}
	var scratch [maxInlinePathSegments + 1]int
	seenPos := make(map[int]bool)

	for _, e := range entries {
		if strings.TrimSpace(e.Path) == "" {
			continue
		}
		parts := splitPath(e.Path)
		if parts.isRoot() {
			continue
		}
		key := parts.key()

		for i, s := range parts.segs {
			if s != platform.WildcardSegment {
				continue
			}
			if !seenPos[i] {
				seenPos[i] = true
				m.wildcardPositions = append(m.wildcardPositions, i)
			}
			rootKey, offsets := parts.keyPrefixes(scratch[:0], -1)
			m.wildcardRoots[rootKey[:offsets[1]]] = true
		}

		if e.Depth == platform.ProtectionWholeSubtree {
			m.subtree[key] = struct{}{}
			if n := len(parts.segs); n > m.maxSubtreeSegs {
				m.maxSubtreeSegs = n
			}
			continue
		}
		if existing, ok := m.limited[key]; !ok || e.Depth > existing {
			m.limited[key] = e.Depth
		}
		if e.Depth > m.maxLimitedDepth {
			m.maxLimitedDepth = e.Depth
		}
	}

	sort.Ints(m.wildcardPositions)
	return m
}

// matchesParts reports whether an already-split path is protected.
func (m *protectionMatcher) matchesParts(parts pathParts) bool {
	if m == nil || (len(m.subtree) == 0 && len(m.limited) == 0) {
		return false
	}
	n := len(parts.segs)
	if n == 0 {
		// A volume root is refused outright by the guard.
		return true
	}

	var scratch [maxInlinePathSegments + 1]int
	key, offsets := parts.keyPrefixes(scratch[:0], -1)

	if m.matchAgainst(key, offsets, n) {
		return true
	}

	// Templated entries. Skipped entirely unless the path actually starts
	// under a root that has templated entries beneath it, which keeps the
	// extra key out of the hot loop for everything outside /Users, /Volumes
	// and their platform equivalents. Within such a root the wildcard key is
	// built once per position and then sliced, not rebuilt per ancestor probe.
	if len(m.wildcardPositions) == 0 || !m.wildcardRoots[key[:offsets[1]]] {
		return false
	}
	for _, pos := range m.wildcardPositions {
		if pos >= n {
			continue
		}
		var wscratch [maxInlinePathSegments + 1]int
		wkey, woffsets := parts.keyPrefixes(wscratch[:0], pos)
		if m.matchAgainst(wkey, woffsets, n) {
			return true
		}
	}
	return false
}

// matchAgainst probes one spelling of the path (literal, or with a wildcard
// substituted) against both entry maps.
func (m *protectionMatcher) matchAgainst(key string, offsets []int, n int) bool {
	// Subtree entries: any ancestor, including the path itself, matching is
	// enough. Bounded by the longest subtree entry in the list.
	upper := n
	if m.maxSubtreeSegs < upper {
		upper = m.maxSubtreeSegs
	}
	for k := 1; k <= upper; k++ {
		if _, ok := m.subtree[key[:offsets[k]]]; ok {
			return true
		}
	}

	// Depth-limited entries: an entry k segments long covers this path only
	// when n-k <= its depth, so only the path itself and its nearest
	// maxLimitedDepth ancestors can match.
	lowest := n - m.maxLimitedDepth
	if lowest < 1 {
		lowest = 1
	}
	for k := n; k >= lowest; k-- {
		if depth, ok := m.limited[key[:offsets[k]]]; ok && n-k <= depth {
			return true
		}
	}
	return false
}

// matches reports whether a path string is protected.
func (m *protectionMatcher) matches(path string) bool {
	return m.matchesParts(splitPath(path))
}

// exclusionSet answers "did the user ask us to skip this subtree?".
//
// Matching is boundary aware: an entry for "/var/log" excludes "/var/log" and
// everything under it, but not "/var/logs". Entries are pre-split once at
// construction so the per-entry cost during a walk is a segment comparison, not
// a re-parse.
type exclusionSet struct {
	roots []pathParts
}

// newExclusionSet builds a matcher from the user's configured exclusion paths.
//
// Entries that cannot be acted on are dropped, and dropping them here is the
// last line of defence — preferences.NormalizeExclusionPaths already removes
// blanks, tildes and relative paths when the value is read or written:
//   - a non-absolute entry has no meaning against an arbitrary scan root;
//   - a bare filesystem/volume root would suppress the entire scan, which is
//     never what someone configuring an exclusion list intended.
func newExclusionSet(paths []string) *exclusionSet {
	set := &exclusionSet{}
	for _, raw := range paths {
		p := strings.TrimSpace(raw)
		if p == "" || !filepath.IsAbs(p) {
			continue
		}
		parts := splitPath(filepath.Clean(p))
		if parts.isRoot() {
			continue
		}
		set.roots = append(set.roots, parts)
	}
	return set
}

func (e *exclusionSet) empty() bool {
	return e == nil || len(e.roots) == 0
}

// forRoot returns the subset of exclusions that make sense for a scan rooted at
// scanRoot: entries that contain the scan root are dropped.
//
// Excluding a directory the user has just explicitly asked to scan would return
// an empty tree and look like a broken application. The explicit request wins;
// exclusions still apply to everything *below* the root.
func (e *exclusionSet) forRoot(scanRoot string) *exclusionSet {
	if e.empty() {
		return e
	}

	root := splitPath(scanRoot)
	out := &exclusionSet{roots: make([]pathParts, 0, len(e.roots))}
	for _, entry := range e.roots {
		if entry.contains(root) {
			continue
		}
		out.roots = append(out.roots, entry)
	}
	return out
}

// excludesParts reports whether an already-split path is an excluded directory
// or lives inside one.
func (e *exclusionSet) excludesParts(parts pathParts) bool {
	if e.empty() {
		return false
	}
	for _, root := range e.roots {
		if root.contains(parts) {
			return true
		}
	}
	return false
}

// excludes reports whether path is an excluded directory or lives inside one.
func (e *exclusionSet) excludes(path string) bool {
	if e.empty() {
		return false
	}
	return e.excludesParts(splitPath(path))
}
