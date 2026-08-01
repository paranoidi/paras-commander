package scan

import (
	"path/filepath"
	"sync"
)

// Index holds the find corpus with dedup by RelLine. Only the scan coordinator writes.
type Index struct {
	mu       sync.RWMutex
	entries  []Entry
	byRel    map[string]int // RelLine -> index in entries
	revision int
}

func newIndex() *Index {
	return &Index{byRel: make(map[string]int)}
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
// Returns entries actually added (deduped by RelLine).
func (idx *Index) Append(displayRoot string, batch []Entry) []Entry {
	if len(batch) == 0 {
		return nil
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	added := make([]Entry, 0, len(batch))
	for _, e := range batch {
		rel := e.RelLine
		if rel == "" {
			continue
		}
		if _, dup := idx.byRel[rel]; dup {
			continue
		}
		entryIdx := len(idx.entries)
		idx.entries = append(idx.entries, e)
		idx.byRel[rel] = entryIdx
		added = append(added, e)
	}
	if len(added) > 0 {
		idx.revision++
	}
	return added
}

// ReplaceEntries swaps the full corpus (selection narrow, hidden strip).
func (idx *Index) ReplaceEntries(displayRoot string, batch []Entry) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.entries = idx.entries[:0]
	idx.byRel = make(map[string]int, len(batch))
	for _, e := range batch {
		rel := e.RelLine
		if rel == "" {
			continue
		}
		if _, dup := idx.byRel[rel]; dup {
			continue
		}
		entryIdx := len(idx.entries)
		idx.entries = append(idx.entries, e)
		idx.byRel[rel] = entryIdx
	}
	idx.revision++
}

func (idx *Index) RunMatch(req MatchRequest, shouldCancel func() bool) MatchOutput {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return runMatchInPlace(idx.entries, req, shouldCancel)
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

// EntryMetaForAbs finds one entry by absolute path under displayRoot.
func (idx *Index) EntryMetaForAbs(displayRoot, absPath string) (Entry, bool) {
	displayRoot = filepath.Clean(displayRoot)
	absPath = filepath.Clean(absPath)
	rel, err := filepath.Rel(displayRoot, absPath)
	if err != nil {
		return Entry{}, false
	}
	rel = filepath.ToSlash(rel)
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	i, ok := idx.byRel[rel]
	if !ok {
		return Entry{}, false
	}
	return idx.entries[i], true
}
