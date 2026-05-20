package gitignore

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Cache stores matchers keyed by listing directory with mtime-based invalidation.
type Cache struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	fingerprint string
	matcher     *Matcher
}

// NewCache returns an empty matcher cache.
func NewCache() *Cache {
	return &Cache{entries: make(map[string]*cacheEntry)}
}

// MatcherForDir returns a matcher for listing directory listDir, or nil when not in a Git work tree.
func (c *Cache) MatcherForDir(listDir string) (*Matcher, error) {
	if c == nil {
		return nil, nil
	}
	listDir, err := filepath.Abs(listDir)
	if err != nil {
		return nil, err
	}
	listDir = filepath.Clean(listDir)

	workRoot := WorkTreeRoot(listDir)
	if workRoot == "" {
		return nil, nil
	}

	fp, err := ignoreFingerprint(workRoot, listDir)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if ent, ok := c.entries[listDir]; ok && ent.fingerprint == fp {
		return ent.matcher, nil
	}
	m, err := newMatcher(workRoot, listDir)
	if err != nil {
		return nil, err
	}
	c.entries[listDir] = &cacheEntry{fingerprint: fp, matcher: m}
	return m, nil
}

func ignoreFingerprint(workRoot, listDir string) (string, error) {
	var b strings.Builder
	for _, dir := range dirsFromRootTo(workRoot, listDir) {
		appendFileMtime(&b, filepath.Join(dir, ".gitignore"))
	}
	appendFileMtime(&b, filepath.Join(workRoot, ".git", "info", "exclude"))
	return b.String(), nil
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
