package compare

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"strings"
)

// hashCacheEntry is the persisted full-file SHA-256 of one file, valid while
// the file's size and mtime still match.
type hashCacheEntry struct {
	Size, ModTime int64
	Hash          [32]byte
}

// DefaultHashCachePath returns the dedup hash cache file under the user cache
// dir, or "" (no cache) when that dir cannot be determined.
func DefaultHashCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "pc", "dedup-hashes.gob")
}

// loadHashCache reads the cache; a missing or corrupt file yields an empty map.
func loadHashCache(path string) map[string]hashCacheEntry {
	m := map[string]hashCacheEntry{}
	f, err := os.Open(path)
	if err != nil {
		return m
	}
	defer func() { _ = f.Close() }()
	var got map[string]hashCacheEntry
	if err := gob.NewDecoder(f).Decode(&got); err != nil || got == nil {
		return m
	}
	return got
}

// saveHashCache writes the cache via a temp file + rename. Errors are dropped:
// the cache is an optimization.
func saveHashCache(path string, m map[string]hashCacheEntry) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dedup-hashes-*")
	if err != nil {
		return
	}
	if err := gob.NewEncoder(tmp).Encode(m); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
	}
}

// mergeHashCache builds the cache to persist after a run: old entries outside
// root, old entries under root whose file was walked with unchanged size and
// mtime, plus every fresh full hash from this run.
func mergeHashCache(old map[string]hashCacheEntry, root string, walked map[string]FileRecord, fresh map[string]hashCacheEntry) map[string]hashCacheEntry {
	out := make(map[string]hashCacheEntry, len(old)+len(fresh))
	prefix := strings.TrimSuffix(root, "/") + "/"
	for p, e := range old {
		if !strings.HasPrefix(p, prefix) {
			out[p] = e
			continue
		}
		if f, ok := walked[p]; ok && f.Size == e.Size && f.ModTime == e.ModTime {
			out[p] = e
		}
	}
	for p, e := range fresh {
		out[p] = e
	}
	return out
}
