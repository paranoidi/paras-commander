package gitstatus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Cache stores git status snapshots keyed by listing directory with mtime-based invalidation.
type Cache struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	fingerprint string
	snapshot    *snapshot
}

// NewCache returns an empty git status cache.
func NewCache() *Cache {
	return &Cache{entries: make(map[string]*cacheEntry)}
}

// ListingPaths describes one row in a panel listing for MapForListing.
type ListingPaths struct {
	AbsPath string
	IsDir   bool
}

// StatusesForListing returns per-path Git cells for listDir inside a work tree.
// workRoot must be the directory containing .git (from gitignore.WorkTreeRoot).
func (c *Cache) StatusesForListing(ctx context.Context, workRoot, listDir string, paths []ListingPaths) (map[string]Cell, error) {
	if c == nil {
		return nil, nil
	}
	listDir, err := filepath.Abs(listDir)
	if err != nil {
		return nil, err
	}
	listDir = filepath.Clean(listDir)
	workRoot = filepath.Clean(workRoot)
	if workRoot == "" {
		return nil, nil
	}

	fp, err := statusFingerprint(workRoot, listDir)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	ent, ok := c.entries[listDir]
	if ok && ent.fingerprint == fp && ent.snapshot != nil {
		sn := ent.snapshot
		c.mu.Unlock()
		return mapFromSnapshot(sn, paths), nil
	}
	c.mu.Unlock()

	sn, err := querySnapshot(ctx, workRoot)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[listDir] = &cacheEntry{fingerprint: fp, snapshot: sn}
	c.mu.Unlock()

	return mapFromSnapshot(sn, paths), nil
}

func mapFromSnapshot(sn *snapshot, paths []ListingPaths) map[string]Cell {
	return MapForListing(sn, paths)
}

func statusFingerprint(workRoot, listDir string) (string, error) {
	var b strings.Builder
	appendFileMtime(&b, filepath.Join(workRoot, ".git", "index"))
	appendFileMtime(&b, filepath.Join(workRoot, ".git", "HEAD"))
	for _, dir := range gitignoreDirsFromRootTo(workRoot, listDir) {
		appendFileMtime(&b, filepath.Join(dir, ".gitignore"))
	}
	appendFileMtime(&b, filepath.Join(workRoot, ".git", "info", "exclude"))
	return b.String(), nil
}

func gitignoreDirsFromRootTo(workRoot, target string) []string {
	// Duplicate dirsFromRootTo logic without exporting from gitignore.
	workRoot = filepath.Clean(workRoot)
	target = filepath.Clean(target)
	if workRoot == "" || target == "" {
		return nil
	}
	if target == workRoot {
		return []string{workRoot}
	}
	rel, err := filepath.Rel(workRoot, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	out := []string{workRoot}
	cur := workRoot
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		out = append(out, cur)
	}
	return out
}

func appendFileMtime(b *strings.Builder, path string) {
	st, err := os.Stat(path)
	if err != nil {
		b.WriteString(path)
		b.WriteString(":missing;")
		return
	}
	b.WriteString(path)
	b.WriteString(":")
	b.WriteString(st.ModTime().UTC().Format("20060102150405.999999999"))
	b.WriteString(";")
}
