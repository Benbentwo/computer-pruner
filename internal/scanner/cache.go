package scanner

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"go.etcd.io/bbolt"

	"github.com/Benbentwo/computer-pruner/internal/preferences"
)

// cacheSchemaVersion identifies the on-disk gob layout of a cached scan.
//
// Bump it whenever TreeNode or ScanSnapshot changes shape. The version is part
// of every key, so entries written by an older build are simply never looked
// up — a stale gob layout can therefore never be fed to a decoder that no longer
// matches it. Version 1 was the unversioned layout keyed by bare path.
const cacheSchemaVersion = 2

// Size caps for a single cache entry. They are variables only so tests can
// lower them; treat them as constants everywhere else.
var (
	// maxCachedEntryBytes caps the compressed blob we are willing to store or
	// read back.
	maxCachedEntryBytes int64 = 64 << 20 // 64 MiB

	// maxDecodedCacheBytes caps the decompressed payload. The cache file is
	// 0600 in the user's own configuration directory, so this is defence in
	// depth rather than a live threat model — but a gzip stream expands
	// arbitrarily, and refusing to decode 200 GiB of zeroes costs nothing.
	maxDecodedCacheBytes int64 = 256 << 20 // 256 MiB
)

var (
	cacheBucket = []byte("scans")

	// errCacheTooLarge is returned by cappedReader once the decompressed
	// payload exceeds maxDecodedCacheBytes.
	errCacheTooLarge = errors.New("cached scan exceeds the maximum decoded size")
)

// ScanSnapshot wraps the tree with a timestamp.
type ScanSnapshot struct {
	Timestamp time.Time
	Root      *TreeNode
}

// CacheService persists finished scans so reopening a previously scanned volume
// is instant. It is entirely optional: every failure degrades to "no cache".
type CacheService struct {
	db *bbolt.DB
}

// NewCacheService opens (creating if needed) the scan cache inside the shared
// configuration directory — the same directory the preferences file lives in.
func NewCacheService() (*CacheService, error) {
	configDir, err := preferences.EnsureConfigDir()
	if err != nil {
		return nil, err
	}
	return newCacheServiceAt(filepath.Join(configDir, "cache.db"))
}

// newCacheServiceAt opens the cache at an explicit path. Tests use it to get an
// isolated database in a temporary directory.
func newCacheServiceAt(dbPath string) (*CacheService, error) {
	// A timeout keeps a second instance of the app from blocking forever on the
	// bolt file lock; it just runs without a cache instead.
	db, err := bbolt.Open(dbPath, 0o600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}

	if err := db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(cacheBucket)
		if err != nil {
			return err
		}
		return dropForeignSchemas(bucket)
	}); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &CacheService{db: db}, nil
}

// dropForeignSchemas deletes entries written by a different schema version, so
// the cache file cannot grow without bound across upgrades.
func dropForeignSchemas(bucket *bbolt.Bucket) error {
	prefix := []byte(cacheKeyPrefix())
	cursor := bucket.Cursor()
	for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
		if bytes.HasPrefix(k, prefix) {
			continue
		}
		if err := cursor.Delete(); err != nil {
			return err
		}
	}
	return nil
}

func cacheKeyPrefix() string {
	return fmt.Sprintf("v%d\x00", cacheSchemaVersion)
}

// cacheKey namespaces a scan root by schema version.
func cacheKey(path string) []byte {
	return []byte(cacheKeyPrefix() + strings.TrimSpace(path))
}

// Close releases the underlying database.
func (c *CacheService) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

// SaveScan stores a finished tree for path.
func (c *CacheService) SaveScan(path string, tree *TreeNode) error {
	if c == nil || c.db == nil {
		return nil
	}
	if tree == nil {
		return nil
	}

	snapshot := ScanSnapshot{
		Timestamp: time.Now(),
		Root:      tree,
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if err := gob.NewEncoder(zw).Encode(snapshot); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}

	if int64(buf.Len()) > maxCachedEntryBytes {
		// Storing something we would refuse to read back is pointless; drop any
		// previous entry so a stale, smaller tree is not served instead.
		return c.DeleteScan(path)
	}

	return c.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(cacheBucket)
		if bucket == nil {
			return errors.New("scan cache bucket is missing")
		}
		return bucket.Put(cacheKey(path), buf.Bytes())
	})
}

// LoadScan returns the cached tree for path, or nil when there is nothing
// usable to return.
//
// "Nothing usable" is reported as (nil, nil), never as an error: a missing,
// expired, oversized, truncated or otherwise undecodable entry must degrade to
// "no cache" and let the scan proceed. The error return is reserved for
// problems with the database itself.
func (c *CacheService) LoadScan(path string, maxAge time.Duration) (*TreeNode, error) {
	if c == nil || c.db == nil {
		return nil, nil
	}

	var compressed []byte
	err := c.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(cacheBucket)
		if bucket == nil {
			return nil
		}
		data := bucket.Get(cacheKey(path))
		if data == nil {
			return nil
		}
		if int64(len(data)) > maxCachedEntryBytes {
			// Refuse to even decompress an implausibly large entry.
			return nil
		}
		// Copy: the slice is only valid for the life of the transaction.
		compressed = make([]byte, len(data))
		copy(compressed, data)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(compressed) == 0 {
		return nil, nil
	}

	snapshot, decodeErr := decodeSnapshot(compressed)
	if decodeErr != nil || snapshot.Root == nil {
		// Corrupt, oversized or written by an incompatible encoder. Drop it so
		// the next scan re-populates the entry, and report "no cache".
		_ = c.DeleteScan(path)
		return nil, nil
	}

	if time.Since(snapshot.Timestamp) > maxAge {
		return nil, nil // Expired.
	}

	return snapshot.Root, nil
}

// decodeSnapshot decompresses and gob-decodes a cache entry under a decoded-size
// cap, converting any panic from the decoder into an ordinary error.
//
// gob decoding is not hardened against hostile input, and a struct layout that
// no longer matches the encoded stream has historically been able to panic
// rather than return an error. The cache file is owner-only, so this is belt and
// braces — but the cost of a recover is one deferred call per scan and the
// alternative is the whole application dying on a corrupt cache.
func decodeSnapshot(compressed []byte) (snapshot ScanSnapshot, err error) {
	defer func() {
		if r := recover(); r != nil {
			snapshot = ScanSnapshot{}
			err = fmt.Errorf("panic while decoding cached scan: %v", r)
		}
	}()

	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return ScanSnapshot{}, err
	}
	defer func() { _ = zr.Close() }()

	reader := &cappedReader{r: zr, remaining: maxDecodedCacheBytes}
	if err := gob.NewDecoder(reader).Decode(&snapshot); err != nil {
		return ScanSnapshot{}, err
	}
	return snapshot, nil
}

// DeleteScan removes the cached tree for path.
func (c *CacheService) DeleteScan(path string) error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(cacheBucket)
		if bucket == nil {
			return nil
		}
		return bucket.Delete(cacheKey(path))
	})
}

// cappedReader fails with errCacheTooLarge instead of returning more than
// `remaining` bytes. Unlike io.LimitReader it does not quietly present the cap
// as a clean EOF, which a decoder could mistake for a valid short stream.
type cappedReader struct {
	r         io.Reader
	remaining int64
}

func (c *cappedReader) Read(p []byte) (int, error) {
	if c.remaining <= 0 {
		return 0, errCacheTooLarge
	}
	if int64(len(p)) > c.remaining {
		p = p[:c.remaining]
	}
	n, err := c.r.Read(p)
	c.remaining -= int64(n)
	return n, err
}
