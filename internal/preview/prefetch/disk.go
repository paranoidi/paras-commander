package prefetch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// diskCache stores video thumb PNGs under dir with a total size cap.
type diskCache struct {
	mu       sync.Mutex
	dir      string
	maxBytes int64
}

func newDiskCache(dir string, maxBytes int64) *diskCache {
	if maxBytes < 1 {
		maxBytes = 1
	}
	return &diskCache{dir: dir, maxBytes: maxBytes}
}

func videoDiskKey(path string, mtime, size int64, maxEdge, cols, rows int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%d\x00%d\x00%d\x00%d", path, mtime, size, maxEdge, cols, rows)))
	return hex.EncodeToString(sum[:]) + ".png"
}

func (d *diskCache) pathFor(key string) string {
	return filepath.Join(d.dir, key)
}

func (d *diskCache) get(key string) ([]byte, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	b, err := os.ReadFile(d.pathFor(key))
	if err != nil || len(b) == 0 {
		return nil, false
	}
	return b, true
}

func (d *diskCache) put(key string, png []byte) error {
	if len(png) == 0 {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		return err
	}
	tmp := d.pathFor(key) + ".tmp"
	if err := os.WriteFile(tmp, png, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, d.pathFor(key)); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	d.evictLocked()
	return nil
}

type diskFile struct {
	path string
	size int64
	mod  time.Time
}

func (d *diskCache) evictLocked() {
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return
	}
	var files []diskFile
	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".png" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		p := filepath.Join(d.dir, name)
		files = append(files, diskFile{path: p, size: info.Size(), mod: info.ModTime()})
		total += info.Size()
	}
	if total <= d.maxBytes {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].mod.Before(files[j].mod)
	})
	for _, f := range files {
		if total <= d.maxBytes {
			break
		}
		if err := os.Remove(f.path); err == nil {
			total -= f.size
		}
	}
}

// DefaultDir returns $XDG_CACHE_HOME/pc/video-thumbs (via os.UserCacheDir).
func DefaultDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pc", "video-thumbs"), nil
}
