package scan

import (
	"path/filepath"
	"sync"
)

// Index holds the find corpus with dedup by RelLine. Only the scan coordinator writes.
type Index struct {
	mu       sync.RWMutex
	entries  []Entry
	byRel    map[string]struct{}
	revision int
}

func newIndex() *Index {
	return &Index{byRel: make(map[string]struct{})}
}

func (idx *Index) Revision() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.revision
}

func (idx *Index) Len() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}

// Append ingests a batch; displayRoot rewrites RelLine for multi-root scopes.
func (idx *Index) Append(displayRoot string, batch []Entry) int {
	if len(batch) == 0 {
		return 0
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	added := 0
	for _, e := range batch {
		p := filepath.Clean(e.Path)
		if p == "" {
			continue
		}
		rel := relLine(displayRoot, p)
		if rel == "" {
			continue
		}
		if _, dup := idx.byRel[rel]; dup {
			continue
		}
		idx.byRel[rel] = struct{}{}
		e.Path = p
		e.RelLine = rel
		idx.entries = append(idx.entries, e)
		added++
	}
	if added > 0 {
		idx.revision++
	}
	return added
}

// ReplaceEntries swaps the full corpus (selection narrow, hidden strip).
func (idx *Index) ReplaceEntries(displayRoot string, batch []Entry) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.entries = idx.entries[:0]
	idx.byRel = make(map[string]struct{}, len(batch))
	for _, e := range batch {
		p := filepath.Clean(e.Path)
		if p == "" {
			continue
		}
		rel := relLine(displayRoot, p)
		if rel == "" {
			continue
		}
		e.Path = p
		e.RelLine = rel
		idx.byRel[rel] = struct{}{}
		idx.entries = append(idx.entries, e)
	}
	idx.revision++
}

// Snapshot returns a copy of all entries for UI sync at index finish.
func (idx *Index) Snapshot() []Entry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if len(idx.entries) == 0 {
		return nil
	}
	out := make([]Entry, len(idx.entries))
	copy(out, idx.entries)
	return out
}

// View calls fn with entry slices under read lock.
func (idx *Index) View(fn func(entries []Entry, revision int)) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	fn(idx.entries, idx.revision)
}

// EntryAt returns one entry by corpus index.
func (idx *Index) EntryAt(i int) (Entry, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if i < 0 || i >= len(idx.entries) {
		return Entry{}, false
	}
	return idx.entries[i], true
}

// LinesAndDirs returns parallel slices for match workers.
func (idx *Index) LinesAndDirs() (lines []string, isDirs []bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	n := len(idx.entries)
	lines = make([]string, n)
	isDirs = make([]bool, n)
	for i, e := range idx.entries {
		lines[i] = e.RelLine
		isDirs[i] = e.IsDir
	}
	return lines, isDirs
}

// PathIndex rebuilds absolute-path lookup maps after indexing completes.
func (idx *Index) PathIndex(displayRoot string) (isDir map[string]bool, sizes map[string]int64) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if len(idx.entries) == 0 {
		return nil, nil
	}
	isDir = make(map[string]bool, len(idx.entries))
	sizes = make(map[string]int64)
	for _, e := range idx.entries {
		abs := filepath.Clean(filepath.Join(displayRoot, filepath.FromSlash(e.RelLine)))
		if abs == "" {
			continue
		}
		isDir[abs] = e.IsDir
		if !e.IsDir && e.Size > 0 {
			sizes[abs] = e.Size
		}
	}
	if len(sizes) == 0 {
		sizes = nil
	}
	return isDir, sizes
}
