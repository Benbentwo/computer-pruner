package scanner

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Benbentwo/computer-pruner/internal/preferences"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// stubPreferences is a PreferencesProvider backed by a fixed value.
type stubPreferences struct {
	prefs preferences.AppPreferences
}

func (s stubPreferences) GetPreferences() *preferences.AppPreferences {
	p := s.prefs
	return &p
}

type recordedEvent struct {
	name string
	data []interface{}
}

type eventRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (r *eventRecorder) record(name string, data ...interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{name: name, data: data})
}

func (r *eventRecorder) last(name string) (recordedEvent, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.events) - 1; i >= 0; i-- {
		if r.events[i].name == name {
			return r.events[i], true
		}
	}
	return recordedEvent{}, false
}

func (r *eventRecorder) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.name)
	}
	return out
}

// newTestScanner builds a ScannerService with no on-disk cache (so tests never
// touch the real configuration directory) and a recording event emitter (so the
// Wails emitter, which calls log.Fatalf on a non-Wails context, is never hit).
func newTestScanner(t *testing.T, prefs *preferences.AppPreferences) (*ScannerService, *eventRecorder) {
	t.Helper()
	rec := &eventRecorder{}
	var provider PreferencesProvider
	if prefs != nil {
		provider = stubPreferences{prefs: *prefs}
	}
	return &ScannerService{prefs: provider, emitFn: rec.record}, rec
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeBusyTree creates dirs*filesPerDir files so a scan takes long enough to be
// cancelled mid-flight. It returns the total number of bytes written.
func writeBusyTree(t *testing.T, root string, dirs, filesPerDir, fileSize int) int64 {
	t.Helper()
	var total int64
	for d := 0; d < dirs; d++ {
		for f := 0; f < filesPerDir; f++ {
			writeFile(t, filepath.Join(root, fmt.Sprintf("dir%02d", d), fmt.Sprintf("f%03d.bin", f)), fileSize)
			total += int64(fileSize)
		}
	}
	return total
}

func waitForScan(t *testing.T, s *ScannerService) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if !s.IsScanning() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("scan did not finish within the deadline")
}

func mustCompleteTree(t *testing.T, rec *eventRecorder) *TreeNode {
	t.Helper()
	ev, ok := rec.last("scan:complete")
	if !ok {
		t.Fatalf("no scan:complete event; got %v", rec.names())
	}
	if len(ev.data) != 1 {
		t.Fatalf("scan:complete carried %d payloads, want 1", len(ev.data))
	}
	node, ok := ev.data[0].(*TreeNode)
	if !ok {
		t.Fatalf("scan:complete payload is %T, want *TreeNode", ev.data[0])
	}
	return node
}

// treeEdgeDepth returns the number of edges on the longest root-to-leaf path.
func treeEdgeDepth(n *TreeNode) int {
	best := 0
	for _, c := range n.Children {
		if d := treeEdgeDepth(c) + 1; d > best {
			best = d
		}
	}
	return best
}

func findChild(n *TreeNode, name string) *TreeNode {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// progress accounting
// ---------------------------------------------------------------------------

// TestProgressTotalSizeEqualsRootSize pins bug #4: scanDir used to add a
// directory child's whole subtree size at every level of the recursion, so the
// progress figure grew with depth. With the layout below the buggy accumulator
// reported 140 bytes for a 60-byte tree.
func TestProgressTotalSizeEqualsRootSize(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), 10)
	writeFile(t, filepath.Join(root, "sub1", "b.txt"), 20)
	writeFile(t, filepath.Join(root, "sub1", "sub2", "c.txt"), 30)

	s, _ := newTestScanner(t, nil)
	if err := s.StartScan(root, true); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	waitForScan(t, s)

	tree := s.GetScanTree()
	if tree == nil {
		t.Fatal("no tree was produced")
	}
	if tree.Size != 60 {
		t.Fatalf("root size = %d, want 60", tree.Size)
	}

	progress := s.GetScanProgress()
	if !progress.IsComplete {
		t.Fatal("progress should be marked complete")
	}
	if progress.TotalSize != tree.Size {
		t.Fatalf("progress TotalSize = %d, want it to equal the root size %d "+
			"(each byte must be accounted for exactly once)", progress.TotalSize, tree.Size)
	}
	if progress.ScannedItems != 5 {
		t.Fatalf("ScannedItems = %d, want 5 (3 files + 2 directories)", progress.ScannedItems)
	}
}

func TestScanCountsFilesAndDirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), 1)
	writeFile(t, filepath.Join(root, "sub1", "b.txt"), 1)
	writeFile(t, filepath.Join(root, "sub1", "sub2", "c.txt"), 1)

	s, _ := newTestScanner(t, nil)
	if err := s.StartScan(root, true); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	waitForScan(t, s)

	tree := s.GetScanTree()
	if tree.FileCount != 3 {
		t.Fatalf("FileCount = %d, want 3", tree.FileCount)
	}
	if tree.DirCount != 2 {
		t.Fatalf("DirCount = %d, want 2", tree.DirCount)
	}
}

// ---------------------------------------------------------------------------
// cancellation lifecycle
// ---------------------------------------------------------------------------

// TestCancelThenImmediateRestart pins bug #1. CancelScan used to clear
// `scanning` straight away, so the next StartScan launched a second goroutine
// while the first was still walking; both then wrote scanTree and progress.
// Run under -race, this is the regression test for that.
func TestCancelThenImmediateRestart(t *testing.T) {
	busy := t.TempDir()
	writeBusyTree(t, busy, 24, 40, 64)

	quiet := t.TempDir()
	writeFile(t, filepath.Join(quiet, "one.bin"), 100)
	writeFile(t, filepath.Join(quiet, "two.bin"), 200)
	writeFile(t, filepath.Join(quiet, "nested", "three.bin"), 300)
	const quietTotal = 600

	s, _ := newTestScanner(t, nil)

	for i := 0; i < 5; i++ {
		if err := s.StartScan(busy, true); err != nil {
			t.Fatalf("iteration %d: StartScan(busy): %v", i, err)
		}
		if err := s.CancelScan(); err != nil {
			t.Fatalf("iteration %d: CancelScan: %v", i, err)
		}
		if err := s.StartScan(quiet, true); err != nil {
			t.Fatalf("iteration %d: StartScan(quiet): %v", i, err)
		}
		waitForScan(t, s)

		tree := s.GetScanTree()
		if tree == nil {
			t.Fatalf("iteration %d: no tree", i)
		}
		if tree.Path != quiet {
			t.Fatalf("iteration %d: tree root is %q, want the second scan's root %q "+
				"(the cancelled scan leaked its results)", i, tree.Path, quiet)
		}
		if tree.Size != quietTotal {
			t.Fatalf("iteration %d: tree size = %d, want %d", i, tree.Size, quietTotal)
		}

		progress := s.GetScanProgress()
		if progress.TotalSize != quietTotal {
			t.Fatalf("iteration %d: progress TotalSize = %d, want %d "+
				"(the cancelled scan's bytes bled into the new scan)", i, progress.TotalSize, quietTotal)
		}
		if progress.ScannedItems != 4 {
			t.Fatalf("iteration %d: ScannedItems = %d, want 4", i, progress.ScannedItems)
		}
	}
}

// TestCancelScanDoesNotClearScanningItself is the direct statement of the fix:
// only the scan goroutine may clear `scanning`.
func TestCancelScanLeavesLifecycleToTheGoroutine(t *testing.T) {
	busy := t.TempDir()
	writeBusyTree(t, busy, 24, 40, 64)

	s, _ := newTestScanner(t, nil)
	if err := s.StartScan(busy, true); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	if err := s.CancelScan(); err != nil {
		t.Fatalf("CancelScan: %v", err)
	}

	// The goroutine may already have finished — but if it has not, `scanning`
	// must still be set, because CancelScan is not allowed to clear it.
	s.mu.Lock()
	scanning, cancelRequested, done := s.scanning, s.cancelRequested, s.done
	s.mu.Unlock()
	if scanning && !cancelRequested {
		t.Fatal("a cancelled but still-running scan must record that cancellation was requested")
	}

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the scan goroutine never closed its done channel")
	}
	if s.IsScanning() {
		t.Fatal("scanning must be false once the goroutine has exited")
	}
}

// TestConcurrentStartCancelAndReads hammers every exported entry point at once;
// its value is what the race detector says about it.
func TestConcurrentStartCancelAndReads(t *testing.T) {
	root := t.TempDir()
	writeBusyTree(t, root, 12, 25, 32)

	s, _ := newTestScanner(t, nil)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				switch (n + j) % 4 {
				case 0:
					_ = s.StartScan(root, true)
				case 1:
					_ = s.CancelScan()
				case 2:
					_ = s.GetScanTree()
				case 3:
					_ = s.GetScanProgress()
				}
			}
		}(i)
	}
	wg.Wait()

	_ = s.CancelScan()
	waitForScan(t, s)
}

func TestScanOfMissingPathReportsAnErrorAndStops(t *testing.T) {
	s, rec := newTestScanner(t, nil)
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	if err := s.StartScan(missing, true); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	waitForScan(t, s)

	if _, ok := rec.last("scan:error"); !ok {
		t.Fatalf("expected a scan:error event, got %v", rec.names())
	}
	if s.IsScanning() {
		t.Fatal("scanning must be cleared after a failed scan")
	}
}

func TestStartScanRejectsAnEmptyPath(t *testing.T) {
	s, _ := newTestScanner(t, nil)
	if err := s.StartScan("   ", true); err == nil {
		t.Fatal("StartScan must reject a blank path")
	}
	if s.IsScanning() {
		t.Fatal("a rejected StartScan must not mark the service as scanning")
	}
}

// ---------------------------------------------------------------------------
// preferences wiring
// ---------------------------------------------------------------------------

// TestExclusionPathsSkipSubtrees pins half of bug #2: ExclusionPaths was
// persisted but never consulted.
func TestExclusionPathsSkipSubtrees(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "keep.bin"), 100)
	writeFile(t, filepath.Join(root, "keepdir", "keep2.bin"), 50)
	writeFile(t, filepath.Join(root, "skipme", "big.bin"), 9000)
	writeFile(t, filepath.Join(root, "skipme", "deep", "bigger.bin"), 90000)
	// A sibling whose name shares a string prefix with the excluded directory
	// must survive: this is what naive strings.HasPrefix would get wrong.
	writeFile(t, filepath.Join(root, "skipme-not", "safe.bin"), 7)

	prefs := preferences.DefaultPreferences()
	prefs.ExclusionPaths = []string{filepath.Join(root, "skipme")}

	s, _ := newTestScanner(t, prefs)
	if err := s.StartScan(root, true); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	waitForScan(t, s)

	tree := s.GetScanTree()
	if findChild(tree, "skipme") != nil {
		t.Fatal("the excluded directory must not appear in the tree")
	}
	if findChild(tree, "skipme-not") == nil {
		t.Fatal("a sibling sharing a string prefix with the exclusion must NOT be excluded")
	}
	const want = 100 + 50 + 7
	if tree.Size != want {
		t.Fatalf("tree size = %d, want %d (excluded bytes must not be counted)", tree.Size, want)
	}
	if got := s.GetScanProgress().TotalSize; got != want {
		t.Fatalf("progress TotalSize = %d, want %d", got, want)
	}
}

// TestExclusionCoveringTheScanRootIsIgnored: asking to scan a directory that is
// itself excluded must not silently return an empty tree. The explicit request
// wins; exclusions below the root still apply.
func TestExclusionCoveringTheScanRootIsIgnored(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "target")
	writeFile(t, filepath.Join(root, "a.bin"), 42)
	writeFile(t, filepath.Join(root, "nested", "b.bin"), 8)

	for _, exclusion := range []string{root, parent} {
		t.Run(filepath.Base(exclusion), func(t *testing.T) {
			prefs := preferences.DefaultPreferences()
			// A nested exclusion is listed alongside the swallowing one, and
			// must still take effect.
			prefs.ExclusionPaths = []string{exclusion, filepath.Join(root, "nested")}

			s, _ := newTestScanner(t, prefs)
			if err := s.StartScan(root, true); err != nil {
				t.Fatalf("StartScan: %v", err)
			}
			waitForScan(t, s)

			tree := s.GetScanTree()
			if tree == nil || tree.Size != 42 {
				t.Fatalf("expected the root to be scanned with the nested exclusion applied; got %+v", tree)
			}
			if findChild(tree, "nested") != nil {
				t.Fatal("the nested exclusion must still be honoured")
			}
		})
	}
}

// TestScanDepthLimitDrivesPruneDepth pins the other half of bug #2:
// pruneTree hardcoded a depth of 8 and never looked at the preference.
func TestScanDepthLimitDrivesPruneDepth(t *testing.T) {
	root := t.TempDir()
	// A ten-level chain, each level holding one file so no level is empty.
	dir := root
	for i := 0; i < 10; i++ {
		dir = filepath.Join(dir, fmt.Sprintf("level%02d", i))
		writeFile(t, filepath.Join(dir, "f.bin"), 1000)
	}

	for _, depth := range []int{1, 2, 5, 8} {
		t.Run(fmt.Sprintf("depth-%d", depth), func(t *testing.T) {
			prefs := preferences.DefaultPreferences()
			prefs.ScanDepthLimit = depth

			s, rec := newTestScanner(t, prefs)
			if err := s.StartScan(root, true); err != nil {
				t.Fatalf("StartScan: %v", err)
			}
			waitForScan(t, s)

			pruned := mustCompleteTree(t, rec)
			if got := treeEdgeDepth(pruned); got != depth {
				t.Fatalf("pruned tree depth = %d, want %d (ScanDepthLimit is not driving the prune)", got, depth)
			}

			// The full tree is always kept in the service, whatever the limit.
			if got := treeEdgeDepth(s.GetScanTree()); got != 11 {
				t.Fatalf("stored tree depth = %d, want the full 11 levels", got)
			}
		})
	}
}

func TestScanDepthLimitIsClamped(t *testing.T) {
	root := t.TempDir()
	dir := root
	for i := 0; i < 4; i++ {
		dir = filepath.Join(dir, fmt.Sprintf("l%d", i))
		writeFile(t, filepath.Join(dir, "f.bin"), 10)
	}

	// A hand-edited preferences file asking for a nonsensical depth must fall
	// back to the default rather than being used verbatim.
	prefs := preferences.DefaultPreferences()
	prefs.ScanDepthLimit = -17

	s, rec := newTestScanner(t, prefs)
	if err := s.StartScan(root, true); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	waitForScan(t, s)

	pruned := mustCompleteTree(t, rec)
	if got, want := treeEdgeDepth(pruned), 5; got != want {
		t.Fatalf("pruned depth = %d, want the whole %d-level tree (default depth is %d)",
			got, want, preferences.DefaultScanDepth)
	}
}

func TestScannerWithoutPreferencesUsesDefaults(t *testing.T) {
	s, _ := newTestScanner(t, nil)
	got := s.settings()
	if got.pruneDepth != preferences.DefaultScanDepth {
		t.Fatalf("pruneDepth = %d, want %d", got.pruneDepth, preferences.DefaultScanDepth)
	}
	if !got.exclusions.empty() {
		t.Fatal("a scanner with no preferences must not exclude anything")
	}
}

// ---------------------------------------------------------------------------
// walk behaviour
// ---------------------------------------------------------------------------

func TestSymlinksAreSkippedNotFollowed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "real", "data.bin"), 100)
	writeFile(t, filepath.Join(root, "file.bin"), 25)

	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "dirlink")); err != nil {
		t.Skipf("symlinks are unavailable on this host: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "file.bin"), filepath.Join(root, "filelink")); err != nil {
		t.Skipf("symlinks are unavailable on this host: %v", err)
	}
	// A link back to the root would turn the walk into an infinite loop if the
	// scanner ever started following links.
	if err := os.Symlink(root, filepath.Join(root, "loop")); err != nil {
		t.Skipf("symlinks are unavailable on this host: %v", err)
	}

	s, _ := newTestScanner(t, nil)
	if err := s.StartScan(root, true); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	waitForScan(t, s)

	tree := s.GetScanTree()
	if tree.Size != 125 {
		t.Fatalf("tree size = %d, want 125 (links contribute nothing and are never followed)", tree.Size)
	}
	for _, name := range []string{"dirlink", "filelink", "loop"} {
		if findChild(tree, name) != nil {
			t.Fatalf("symlink %q must not appear in the tree", name)
		}
	}
}

func TestWalkDepthGuardStopsRunawayRecursion(t *testing.T) {
	original := maxWalkDepth
	maxWalkDepth = 2
	defer func() { maxWalkDepth = original }()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a", "b", "c.bin"), 500)

	s, _ := newTestScanner(t, nil)
	if err := s.StartScan(root, true); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	waitForScan(t, s)

	tree := s.GetScanTree()
	if tree.Size != 0 {
		t.Fatalf("tree size = %d, want 0: everything below the depth guard is dropped", tree.Size)
	}
	if tree.DirCount != 2 {
		t.Fatalf("DirCount = %d, want 2", tree.DirCount)
	}
	if tree.FileCount != 0 {
		t.Fatalf("FileCount = %d, want 0", tree.FileCount)
	}
}

func TestUnreadableDirectoryDoesNotAbortTheScan(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "readable", "a.bin"), 30)
	locked := filepath.Join(root, "locked")
	writeFile(t, filepath.Join(locked, "b.bin"), 70)
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Skipf("cannot drop directory permissions on this host: %v", err)
	}
	defer func() { _ = os.Chmod(locked, 0o755) }()

	// Running as root, or on a filesystem that ignores the mode bits, the
	// directory stays readable and there is nothing to test.
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("this host can still read a 0000 directory; nothing to exercise")
	}

	s, _ := newTestScanner(t, nil)
	if err := s.StartScan(root, true); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	waitForScan(t, s)

	tree := s.GetScanTree()
	if tree == nil {
		t.Fatal("an unreadable subdirectory must not abort the whole scan")
	}
	if tree.Size != 30 {
		t.Fatalf("tree size = %d, want 30", tree.Size)
	}
}
