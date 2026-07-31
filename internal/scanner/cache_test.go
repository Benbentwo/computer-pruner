package scanner

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.etcd.io/bbolt"
)

func newTestCache(t *testing.T) *CacheService {
	t.Helper()
	c, err := newCacheServiceAt(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("newCacheServiceAt: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// putRaw writes an arbitrary value under an arbitrary key, bypassing SaveScan,
// so the corruption paths can be exercised.
func putRaw(t *testing.T, c *CacheService, key, value []byte) {
	t.Helper()
	err := c.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(cacheBucket).Put(key, value)
	})
	if err != nil {
		t.Fatalf("putRaw: %v", err)
	}
}

func hasKey(t *testing.T, c *CacheService, key []byte) bool {
	t.Helper()
	var found bool
	err := c.db.View(func(tx *bbolt.Tx) error {
		found = tx.Bucket(cacheBucket).Get(key) != nil
		return nil
	})
	if err != nil {
		t.Fatalf("hasKey: %v", err)
	}
	return found
}

func sampleTree() *TreeNode {
	return dirNode("root",
		fileNode("a.bin", 100),
		dirNode("sub", fileNode("b.bin", 200)),
	)
}

func TestCacheRoundTrip(t *testing.T) {
	c := newTestCache(t)
	tree := sampleTree()

	if err := c.SaveScan("/scan/root", tree); err != nil {
		t.Fatalf("SaveScan: %v", err)
	}

	got, err := c.LoadScan("/scan/root", time.Hour)
	if err != nil {
		t.Fatalf("LoadScan: %v", err)
	}
	if got == nil {
		t.Fatal("LoadScan returned nothing for a freshly saved scan")
	}
	if got.Size != tree.Size || got.Name != tree.Name || len(got.Children) != len(tree.Children) {
		t.Fatalf("round trip changed the tree: got %+v", got)
	}
}

func TestLoadScanMissingEntry(t *testing.T) {
	c := newTestCache(t)
	got, err := c.LoadScan("/never/scanned", time.Hour)
	if err != nil || got != nil {
		t.Fatalf("LoadScan = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestLoadScanHonoursTTL(t *testing.T) {
	c := newTestCache(t)
	if err := c.SaveScan("/scan/root", sampleTree()); err != nil {
		t.Fatalf("SaveScan: %v", err)
	}

	// A zero TTL expires anything that is not from the future.
	got, err := c.LoadScan("/scan/root", -time.Second)
	if err != nil || got != nil {
		t.Fatalf("LoadScan on an expired entry = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestDeleteScanRemovesTheEntry(t *testing.T) {
	c := newTestCache(t)
	if err := c.SaveScan("/scan/root", sampleTree()); err != nil {
		t.Fatalf("SaveScan: %v", err)
	}
	if err := c.DeleteScan("/scan/root"); err != nil {
		t.Fatalf("DeleteScan: %v", err)
	}
	got, err := c.LoadScan("/scan/root", time.Hour)
	if err != nil || got != nil {
		t.Fatalf("LoadScan after delete = (%v, %v), want (nil, nil)", got, err)
	}
}

// TestCacheKeyIsSchemaVersioned pins the requirement that a stale gob layout can
// never be handed to the decoder: entries written under a different schema
// version are simply not addressable.
func TestCacheKeyIsSchemaVersioned(t *testing.T) {
	c := newTestCache(t)

	key := string(cacheKey("/scan/root"))
	if !strings.HasPrefix(key, cacheKeyPrefix()) {
		t.Fatalf("cache key %q does not carry the schema version prefix %q", key, cacheKeyPrefix())
	}
	if !strings.Contains(key, "/scan/root") {
		t.Fatalf("cache key %q does not carry the scan path", key)
	}

	// An entry written by a hypothetical previous schema must be invisible.
	putRaw(t, c, []byte("v1\x00/scan/root"), []byte("gob written by an older layout"))
	got, err := c.LoadScan("/scan/root", time.Hour)
	if err != nil || got != nil {
		t.Fatalf("LoadScan = (%v, %v); an entry from another schema version must be ignored", got, err)
	}
}

// TestForeignSchemaEntriesArePrunedOnOpen keeps the cache file from growing
// without bound across upgrades.
func TestForeignSchemaEntriesArePrunedOnOpen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cache.db")

	c, err := newCacheServiceAt(dbPath)
	if err != nil {
		t.Fatalf("newCacheServiceAt: %v", err)
	}
	putRaw(t, c, []byte("v1\x00/scan/root"), []byte("old"))
	if err := c.SaveScan("/scan/root", sampleTree()); err != nil {
		t.Fatalf("SaveScan: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := newCacheServiceAt(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	if hasKey(t, reopened, []byte("v1\x00/scan/root")) {
		t.Fatal("an entry from a previous schema version should have been pruned on open")
	}
	if !hasKey(t, reopened, cacheKey("/scan/root")) {
		t.Fatal("the current schema's entry must survive the prune")
	}
}

// TestLoadScanDegradesOnCorruptEntry pins the requirement that a bad cache never
// breaks a scan.
func TestLoadScanDegradesOnCorruptEntry(t *testing.T) {
	cases := map[string][]byte{
		"not gzip at all":     []byte("plain text, definitely not a gzip stream"),
		"truncated gzip":      gzipBytes(t, []byte("hello"))[:5],
		"gzip of nonsense":    gzipBytes(t, []byte("this is not a gob stream")),
		"gzip of zeroes":      gzipBytes(t, make([]byte, 1<<20)),
		"gzip of a wrong gob": gzipBytes(t, gobBytes(t, map[string]int{"unexpected": 1})),
	}

	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			c := newTestCache(t)
			putRaw(t, c, cacheKey("/scan/root"), payload)

			got, err := c.LoadScan("/scan/root", time.Hour)
			if err != nil {
				t.Fatalf("LoadScan returned an error (%v); a corrupt cache must degrade to \"no cache\"", err)
			}
			if got != nil {
				t.Fatalf("LoadScan returned %+v, want nil", got)
			}
			if hasKey(t, c, cacheKey("/scan/root")) {
				t.Fatal("a corrupt entry should be dropped so the next scan can repopulate it")
			}
		})
	}
}

// TestLoadScanRejectsAnOversizedDecodedPayload is the decompression-bomb guard.
// The cap is lowered rather than building a quarter-gigabyte fixture, but the
// payload is a genuine, well-formed snapshot, so the failure has to come from
// the decoded-size cap and not from the decoder rejecting nonsense.
func TestLoadScanRejectsAnOversizedDecodedPayload(t *testing.T) {
	c := newTestCache(t)

	children := make([]*TreeNode, 0, 200)
	for i := 0; i < 200; i++ {
		children = append(children, fileNode(fmt.Sprintf("file-%03d.bin", i), int64(i)))
	}
	big := dirNode("root", children...)

	if err := c.SaveScan("/scan/root", big); err != nil {
		t.Fatalf("SaveScan: %v", err)
	}

	// Sanity: at the real cap this entry loads fine.
	if got, err := c.LoadScan("/scan/root", time.Hour); err != nil || got == nil {
		t.Fatalf("precondition failed: LoadScan = (%v, %v), want a tree", got, err)
	}

	defer withDecodedCap(512)()

	got, err := c.LoadScan("/scan/root", time.Hour)
	if err != nil {
		t.Fatalf("LoadScan returned an error (%v); an oversized payload must degrade to \"no cache\"", err)
	}
	if got != nil {
		t.Fatal("a payload larger than the decoded-size cap must not produce a tree")
	}
}

func TestLoadScanIgnoresAnOversizedCompressedEntry(t *testing.T) {
	c := newTestCache(t)
	defer withCompressedCap(64)()

	putRaw(t, c, cacheKey("/scan/root"), make([]byte, 65))

	got, err := c.LoadScan("/scan/root", time.Hour)
	if err != nil || got != nil {
		t.Fatalf("LoadScan = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestSaveScanRefusesToStoreWhatItCouldNotReadBack(t *testing.T) {
	c := newTestCache(t)
	defer withCompressedCap(8)()

	if err := c.SaveScan("/scan/root", sampleTree()); err != nil {
		t.Fatalf("SaveScan: %v", err)
	}
	if hasKey(t, c, cacheKey("/scan/root")) {
		t.Fatal("an entry too large to be read back should not be stored at all")
	}
}

// withDecodedCap lowers the decompressed-payload cap and returns a restore func.
func withDecodedCap(n int64) func() {
	original := maxDecodedCacheBytes
	maxDecodedCacheBytes = n
	return func() { maxDecodedCacheBytes = original }
}

// withCompressedCap lowers the stored-blob cap and returns a restore func.
func withCompressedCap(n int64) func() {
	original := maxCachedEntryBytes
	maxCachedEntryBytes = n
	return func() { maxCachedEntryBytes = original }
}

func TestCappedReaderStopsAtTheLimit(t *testing.T) {
	src := bytes.NewReader(make([]byte, 100))
	r := &cappedReader{r: src, remaining: 10}

	n, err := io.Copy(io.Discard, r)
	if !errors.Is(err, errCacheTooLarge) {
		t.Fatalf("error = %v, want errCacheTooLarge", err)
	}
	if n != 10 {
		t.Fatalf("copied %d bytes, want 10", n)
	}
}

func TestNilCacheServiceIsInert(t *testing.T) {
	var c *CacheService
	if err := c.SaveScan("/x", sampleTree()); err != nil {
		t.Fatalf("SaveScan on a nil cache: %v", err)
	}
	got, err := c.LoadScan("/x", time.Hour)
	if err != nil || got != nil {
		t.Fatalf("LoadScan on a nil cache = (%v, %v), want (nil, nil)", got, err)
	}
	if err := c.DeleteScan("/x"); err != nil {
		t.Fatalf("DeleteScan on a nil cache: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close on a nil cache: %v", err)
	}
}

// TestScanServedFromCacheReportsCompleteProgress covers the cache-hit path of a
// scan end to end.
func TestScanServedFromCacheReportsCompleteProgress(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.bin"), 100)

	s, rec := newTestScanner(t, nil)
	s.cache = newTestCache(t)

	// First scan populates the cache.
	if err := s.StartScan(root, true); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	waitForScan(t, s)
	first := s.GetScanTree()

	// Delete the data. If the second scan walks the disk again it will report
	// zero bytes, so a correct size proves the result came from the cache.
	if err := os.Remove(filepath.Join(root, "a.bin")); err != nil {
		t.Fatal(err)
	}

	// Second scan is served from it.
	if err := s.StartScan(root, false); err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	waitForScan(t, s)

	tree := s.GetScanTree()
	if tree == nil || tree.Size != first.Size {
		t.Fatalf("cached tree = %+v, want the same size as %+v", tree, first)
	}
	progress := s.GetScanProgress()
	if !progress.IsComplete {
		t.Fatal("a cache hit must still report completion")
	}
	if progress.TotalSize != tree.Size {
		t.Fatalf("progress TotalSize = %d, want %d", progress.TotalSize, tree.Size)
	}
	if _, ok := rec.last("scan:complete"); !ok {
		t.Fatalf("a cache hit must emit scan:complete; got %v", rec.names())
	}
}

func gzipBytes(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gobBytes(t *testing.T, v interface{}) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
