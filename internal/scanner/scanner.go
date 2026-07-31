// Package scanner walks a directory tree, builds a size-annotated node tree for
// the disk analyser, streams progress to the frontend and caches finished scans.
//
// # Scan lifecycle
//
// Exactly one scan goroutine may be running at a time, and that goroutine owns
// the scan state for its whole life. Every scan is stamped with a monotonically
// increasing epoch; any write to the shared tree or progress is dropped unless
// the writer's epoch is still the current one. Cancellation therefore never
// leaves a half-finished scan able to overwrite the results of the scan that
// replaced it, and `scanning` is cleared by the goroutine as it exits — never by
// the caller of CancelScan.
package scanner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Benbentwo/computer-pruner/internal/platform"
	"github.com/Benbentwo/computer-pruner/internal/preferences"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	// maxChildrenPerNode is how many children a directory may contribute to the
	// pruned tree before the remainder is grouped into a single summary node.
	maxChildrenPerNode = 80

	// minChildSizeFraction groups children smaller than this fraction of their
	// parent. 0.005 is half a percent — below that a slice is not clickable.
	minChildSizeFraction = 0.005

	// cacheTTL is how long a saved scan stays usable.
	cacheTTL = 24 * time.Hour

	// progressEmitInterval throttles scan:progress events to one per N entries.
	progressEmitInterval = 200

	// scanDrainTimeout bounds how long StartScan waits for a cancelled scan's
	// goroutine to exit before giving up. The walk checks for cancellation once
	// per directory entry, so in practice this is microseconds; the timeout only
	// exists so a wedged syscall cannot deadlock the UI thread forever.
	scanDrainTimeout = 30 * time.Second
)

// maxWalkDepth bounds recursion so a pathologically deep tree cannot exhaust
// the goroutine stack. It is a variable purely so tests can lower it.
var maxWalkDepth = 4096

// TreeNode represents a file or directory in the scan tree.
//
// Changing this struct changes the gob layout of the on-disk scan cache; bump
// cacheSchemaVersion in cache.go when you do.
type TreeNode struct {
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	Size        int64       `json:"size"`
	IsDir       bool        `json:"isDir"`
	IsProtected bool        `json:"isProtected"`
	ModTime     string      `json:"modTime"`
	Children    []*TreeNode `json:"children"`
	FileCount   int         `json:"fileCount"`
	DirCount    int         `json:"dirCount"`
}

// ScanProgress represents the current progress of a disk scan.
//
// TotalSize is the number of bytes actually accounted for so far. Only leaf
// files contribute to it: a directory's size is the sum of its descendants, so
// adding directory totals as the recursion unwinds would count the same bytes
// once per level. When a scan completes, TotalSize equals the root node's Size.
type ScanProgress struct {
	ScannedItems int    `json:"scannedItems"`
	TotalSize    int64  `json:"totalSize"`
	CurrentPath  string `json:"currentPath"`
	ElapsedMs    int    `json:"elapsedMs"`
	IsComplete   bool   `json:"isComplete"`
}

// PreferencesProvider supplies the user's scan settings to the scanner.
// *preferences.PreferencesService satisfies it; tests substitute a stub.
type PreferencesProvider interface {
	GetPreferences() *preferences.AppPreferences
}

// ScannerService provides disk scanning functionality.
type ScannerService struct {
	mu sync.Mutex

	// ctx is the Wails runtime context, set once at startup.
	ctx context.Context

	// scanTree and progress belong to the scan identified by epoch.
	scanTree *TreeNode
	progress ScanProgress

	// epoch identifies the current scan. A goroutine whose epoch no longer
	// matches has been superseded and must not touch shared state.
	epoch uint64
	// scanning is true from the moment a scan goroutine is created until that
	// goroutine returns. It is cleared only by the goroutine itself.
	scanning bool
	// cancelRequested records that CancelScan has fired for the running scan,
	// which is what lets a subsequent StartScan wait for the goroutine to drain
	// instead of refusing as "already scanning".
	cancelRequested bool
	cancelFn        context.CancelFunc
	// done is closed by the scan goroutine as its very last action.
	done chan struct{}

	cache *CacheService
	prefs PreferencesProvider

	// emitFn replaces the Wails event emitter in tests. The real emitter calls
	// log.Fatalf when handed a context that did not come from a Wails lifecycle
	// hook, so tests must never reach it.
	emitFn func(name string, data ...interface{})
}

// There is deliberately no NewScannerService() convenience constructor.
//
// One existed, it silently pinned the scanner to preferences.DefaultPreferences(),
// and main.go called it — so the user's configured scan depth and exclusion
// paths were dead for the whole life of that code. It compiled, it vetted, and
// no gate caught it. Requiring the dependency at the call site makes that
// mistake unrepresentable; pass nil explicitly if defaults really are what you
// want.

// NewScannerServiceWithPreferences creates a ScannerService that reads the
// user's scan depth and exclusion paths from prefs on every scan, so a settings
// change takes effect on the next scan without a restart.
//
// A nil prefs is legal and means "always use preferences.DefaultPreferences()".
func NewScannerServiceWithPreferences(prefs PreferencesProvider) *ScannerService {
	// A cache failure is not fatal: scanning still works, it is just never
	// served from disk.
	cache, _ := NewCacheService()
	return &ScannerService{
		cache: cache,
		prefs: prefs,
	}
}

// SetContext sets the wails runtime context for the service.
func (s *ScannerService) SetContext(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx = ctx
}

// Close releases the scan cache. It is safe to call on a nil-cache service.
func (s *ScannerService) Close() error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Close()
}

// StartScan begins a disk scan on the specified mount point.
func (s *ScannerService) StartScan(mountPoint string, forceRescan bool) error {
	return s.startScanInternal(mountPoint, forceRescan)
}

// StartFolderScan begins a scan on a specific folder path.
func (s *ScannerService) StartFolderScan(path string, forceRescan bool) error {
	return s.startScanInternal(path, forceRescan)
}

func (s *ScannerService) startScanInternal(rootPath string, forceRescan bool) error {
	rootPath = strings.TrimSpace(rootPath)
	if rootPath == "" {
		return errors.New("cannot scan an empty path")
	}

	s.mu.Lock()
	if s.scanning {
		if !s.cancelRequested {
			// A scan is genuinely in flight; leave it alone.
			s.mu.Unlock()
			return nil
		}
		// A cancellation is in flight. Wait for the goroutine to actually exit
		// before starting a new one — that, not the epoch counter, is what
		// guarantees there is only ever one writer.
		done := s.done
		s.mu.Unlock()
		select {
		case <-done:
		case <-time.After(scanDrainTimeout):
			return errors.New("the previous scan did not stop in time")
		}
		s.mu.Lock()
		if s.scanning {
			// Another caller won the race and started a scan first.
			s.mu.Unlock()
			return nil
		}
	}

	s.epoch++
	epoch := s.epoch

	scanCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	s.cancelFn = cancel
	s.cancelRequested = false
	s.done = done
	s.scanning = true
	s.progress = ScanProgress{}
	s.mu.Unlock()

	go s.runScan(scanCtx, cancel, done, epoch, rootPath, forceRescan)
	return nil
}

// CancelScan asks the running scan to stop.
//
// It deliberately does *not* clear `scanning`: only the scan goroutine may do
// that, as it exits. Clearing it here used to let a new scan start while the old
// goroutine was still walking the filesystem, with both writing scanTree and
// progress.
func (s *ScannerService) CancelScan() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.scanning || s.cancelFn == nil {
		return nil
	}
	s.cancelRequested = true
	s.cancelFn()
	return nil
}

// IsScanning reports whether a scan goroutine is currently running.
func (s *ScannerService) IsScanning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scanning
}

// GetScanTree returns the complete tree structure from the last scan.
func (s *ScannerService) GetScanTree() *TreeNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scanTree
}

// GetScanProgress returns the current progress of an ongoing scan.
func (s *ScannerService) GetScanProgress() *ScanProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.progress
	return &p
}

// scanSettings is the snapshot of user preferences a single scan runs with.
// Taking it once per scan means a mid-scan settings change cannot alter the
// meaning of the tree that is being built.
type scanSettings struct {
	pruneDepth int
	exclusions *exclusionSet
}

func (s *ScannerService) settings() scanSettings {
	prefs := preferences.DefaultPreferences()
	if s.prefs != nil {
		if p := s.prefs.GetPreferences(); p != nil {
			prefs = p
		}
	}
	return scanSettings{
		pruneDepth: preferences.ClampScanDepth(prefs.ScanDepthLimit),
		exclusions: newExclusionSet(prefs.ExclusionPaths),
	}
}

// runScan performs the actual recursive directory walk. It owns the scan
// lifecycle: whatever happens, the deferred finish clears `scanning` (if this
// scan is still the current one) and closes `done` last, so a waiter that
// observes `done` also observes the cleared flag.
func (s *ScannerService) runScan(
	ctx context.Context,
	cancel context.CancelFunc,
	done chan struct{},
	epoch uint64,
	rootPath string,
	forceRescan bool,
) {
	defer close(done)
	defer cancel()
	defer s.finishScan(epoch)

	settings := s.settings()

	if !forceRescan && s.cache != nil {
		cachedTree, err := s.cache.LoadScan(rootPath, cacheTTL)
		if err == nil && cachedTree != nil {
			if s.publish(epoch, cachedTree, true) {
				s.emit(epoch, "scan:complete", pruneTree(cachedTree, settings.pruneDepth))
			}
			return
		}
	}

	startTime := time.Now()
	protectedEntries, _ := platform.ProtectedEntries()
	protected := newProtectionMatcher(protectedEntries)

	root := s.scanDir(ctx, epoch, rootPath, scanContext{
		protected: protected,
		// An exclusion that swallows the directory the user just asked to scan
		// is dropped; see exclusionSet.forRoot.
		exclusions: settings.exclusions.forRoot(rootPath),
		startTime:  startTime,
	}, 0)

	if root == nil {
		if ctx.Err() != nil {
			s.emit(epoch, "scan:error", "Scan was cancelled")
		} else {
			s.emit(epoch, "scan:error", fmt.Sprintf("Unable to read %s", rootPath))
		}
		return
	}

	if !s.publish(epoch, root, false) {
		// Superseded while we were finishing: the newer scan owns the state and
		// this tree is thrown away rather than cross-contaminating it.
		return
	}

	if s.cache != nil {
		// Saving is part of the scan's work, so it happens on this goroutine
		// rather than in yet another unsupervised one.
		_ = s.cache.SaveScan(rootPath, root)
	}

	// Prune the tree to a manageable size before sending it to the frontend.
	// The full tree stays in s.scanTree for lazy-loading.
	s.emit(epoch, "scan:complete", pruneTree(root, settings.pruneDepth))
}

// finishScan marks the scan identified by epoch as no longer running.
func (s *ScannerService) finishScan(epoch uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.epoch != epoch {
		// Superseded: the newer scan owns `scanning`, `cancelFn` and `done`.
		return
	}
	s.scanning = false
	s.cancelRequested = false
	s.cancelFn = nil
}

// publish stores a finished tree, but only if this scan is still the current
// one. It reports whether the write happened.
//
// fromCache tells it to synthesise the progress totals: a cache hit accumulates
// no per-file progress, so the figures have to be filled in from the tree.
func (s *ScannerService) publish(epoch uint64, root *TreeNode, fromCache bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.epoch != epoch {
		return false
	}
	s.scanTree = root
	s.progress.IsComplete = true
	if fromCache {
		s.progress.TotalSize = root.Size
		s.progress.ScannedItems = root.FileCount + root.DirCount
		s.progress.CurrentPath = root.Path
	}
	return true
}

// emit sends an event to the frontend, unless this scan has been superseded.
func (s *ScannerService) emit(epoch uint64, name string, data ...interface{}) {
	s.mu.Lock()
	current := s.epoch == epoch
	ctx := s.ctx
	fn := s.emitFn
	s.mu.Unlock()

	if !current {
		return
	}
	if fn != nil {
		fn(name, data...)
		return
	}
	if ctx == nil {
		// No Wails context yet (or running under test): there is nothing to
		// emit to, and the real emitter aborts the process on a nil context.
		return
	}
	wailsruntime.EventsEmit(ctx, name, data...)
}

// scanContext carries the per-scan values that never change during a walk.
type scanContext struct {
	protected  *protectionMatcher
	exclusions *exclusionSet
	startTime  time.Time
}

// scanDir recursively scans a directory and builds a TreeNode tree. It returns
// nil when the walk was cancelled or the path could not be stat'ed.
//
// # Symlinks
//
// Symbolic links are counted as visited but never followed, and contribute zero
// bytes. This is a deliberate trade-off, not an oversight:
//
//   - Following links makes the walk unsound. A link to an ancestor turns the
//     tree into a cycle, and even acyclic links cause the same bytes to be
//     counted under several parents, so the reported totals stop adding up.
//   - Not following them under-reports: a home directory whose big folders are
//     links to another volume looks small.
//
// For a tool whose job is "show me what to delete", double-counting is the worse
// failure — it would attribute reclaimable space to a directory that does not
// own it. Deduplicating link targets properly needs device+inode identity
// tracking, which is a separate change and does not exist on Windows in the same
// form. Until then the walk stays acyclic by construction.
func (s *ScannerService) scanDir(ctx context.Context, epoch uint64, dirPath string, sc scanContext, depth int) *TreeNode {
	if ctx.Err() != nil {
		return nil
	}

	info, err := os.Lstat(dirPath)
	if err != nil {
		return nil
	}

	// The directory is split once and each entry's parts are derived from it, so
	// a walk of millions of files does not re-parse a full path string per entry.
	dirParts := splitPath(dirPath)

	node := &TreeNode{
		Name:        info.Name(),
		Path:        dirPath,
		IsDir:       info.IsDir(),
		IsProtected: sc.protected.matchesParts(dirParts),
		ModTime:     info.ModTime().Format(time.RFC3339),
		Children:    []*TreeNode{},
	}

	if !info.IsDir() {
		node.Size = info.Size()
		return node
	}

	// Guard against pathological nesting exhausting the stack. Deeper levels are
	// reported as an empty directory rather than crashing the application.
	if depth >= maxWalkDepth {
		return node
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		// Permission denied or other error — return the node with zero size.
		return node
	}

	var totalSize int64
	var fileCount, dirCount int

	for _, entry := range entries {
		if ctx.Err() != nil {
			return nil
		}

		// See the symlink note in this function's doc comment.
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		childPath := filepath.Join(dirPath, entry.Name())
		childParts := dirParts.child(entry.Name())

		// Skip anything the user excluded. Checking here (rather than at the top
		// of the recursion) means an excluded subtree is never even opened, and
		// an excluded path contributes nothing at all — no size, no counts, no
		// progress.
		if sc.exclusions.excludesParts(childParts) {
			continue
		}

		// progressBytes is the number of bytes this iteration newly accounts
		// for. Only leaf files add anything: a directory's bytes were already
		// added, one file at a time, while recursing into it.
		var progressBytes int64

		if entry.IsDir() {
			child := s.scanDir(ctx, epoch, childPath, sc, depth+1)
			if child == nil {
				if ctx.Err() != nil {
					return nil
				}
				// Just an unreadable directory — skip it and keep going.
				continue
			}
			node.Children = append(node.Children, child)
			totalSize += child.Size
			dirCount += 1 + child.DirCount
			fileCount += child.FileCount
		} else {
			childInfo, err := entry.Info()
			if err != nil {
				continue
			}
			child := &TreeNode{
				Name:        entry.Name(),
				Path:        childPath,
				Size:        childInfo.Size(),
				IsDir:       false,
				IsProtected: sc.protected.matchesParts(childParts),
				ModTime:     childInfo.ModTime().Format(time.RFC3339),
				Children:    []*TreeNode{},
			}
			node.Children = append(node.Children, child)
			totalSize += child.Size
			fileCount++
			progressBytes = child.Size
		}

		if p, shouldEmit := s.noteProgress(epoch, childPath, progressBytes, sc.startTime); shouldEmit {
			s.emit(epoch, "scan:progress", p)
		}
	}

	node.Size = totalSize
	node.FileCount = fileCount
	node.DirCount = dirCount

	return node
}

// noteProgress records one visited entry. It returns the progress snapshot and
// whether the caller should emit it. Updates from a superseded scan are dropped.
func (s *ScannerService) noteProgress(epoch uint64, path string, bytes int64, startTime time.Time) (ScanProgress, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.epoch != epoch {
		return ScanProgress{}, false
	}

	s.progress.ScannedItems++
	s.progress.TotalSize += bytes
	s.progress.CurrentPath = path
	s.progress.ElapsedMs = int(time.Since(startTime).Milliseconds())

	if s.progress.ScannedItems%progressEmitInterval != 0 {
		return ScanProgress{}, false
	}
	return s.progress, true
}

// pruneTree creates a depth-limited copy of the tree suitable for frontend
// rendering. It caps depth, limits children per node and groups tiny items into
// a single "(other small items)" node. The original tree is not modified.
//
// maxDepth comes from the user's ScanDepthLimit preference and is clamped to the
// range preferences allows, so a hand-edited settings file cannot ask for a tree
// with a million levels.
func pruneTree(node *TreeNode, maxDepth int) *TreeNode {
	if node == nil {
		return nil
	}
	return pruneNode(node, preferences.ClampScanDepth(maxDepth), maxChildrenPerNode, minChildSizeFraction, 0)
}

func pruneNode(node *TreeNode, maxDepth, maxChildren int, minFraction float64, depth int) *TreeNode {
	// Copy the node (shallow copy, we rebuild the children).
	out := &TreeNode{
		Name:        node.Name,
		Path:        node.Path,
		Size:        node.Size,
		IsDir:       node.IsDir,
		IsProtected: node.IsProtected,
		ModTime:     node.ModTime,
		FileCount:   node.FileCount,
		DirCount:    node.DirCount,
		Children:    []*TreeNode{},
	}

	if !node.IsDir || len(node.Children) == 0 {
		return out
	}

	// At max depth, collapse all children — sizes are preserved on this node.
	if depth >= maxDepth {
		return out
	}

	// Sort children by size descending so the biggest items are kept.
	sorted := make([]*TreeNode, len(node.Children))
	copy(sorted, node.Children)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Size > sorted[j].Size
	})

	sizeThreshold := int64(float64(node.Size) * minFraction)
	var kept []*TreeNode
	var otherSize int64
	var otherFiles, otherDirs, otherCount int

	for i, child := range sorted {
		// Keep if within the maxChildren limit AND above the size threshold.
		if i < maxChildren && child.Size >= sizeThreshold {
			kept = append(kept, pruneNode(child, maxDepth, maxChildren, minFraction, depth+1))
			continue
		}

		otherSize += child.Size
		otherCount++
		if child.IsDir {
			// The directory itself, plus everything already counted inside it.
			// Adding child.DirCount *and* incrementing separately used to count
			// the directory's own subtree twice.
			otherDirs += 1 + child.DirCount
			otherFiles += child.FileCount
		} else {
			otherFiles++
		}
	}

	out.Children = kept

	// If there are grouped items, add a synthetic "Other" node.
	if otherCount > 0 {
		out.Children = append(out.Children, &TreeNode{
			Name:      "(other small items)",
			Path:      node.Path + "/<other>",
			Size:      otherSize,
			IsDir:     true,
			ModTime:   node.ModTime,
			FileCount: otherFiles,
			DirCount:  otherDirs,
			Children:  []*TreeNode{},
		})
	}

	return out
}
