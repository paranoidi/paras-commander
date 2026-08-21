package scan

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
)

const (
	hiddenExpandPerTick    = 4
	hiddenEnqueuePerTick   = 64
	hiddenFilesSpliceBatch = 512
	maxConcurrentWalks     = 4
	indexMediumThreshold   = 50_000
	indexLargeThreshold    = 500_000
)

func hiddenExpandPerTickForCount(indexed int) int {
	switch {
	case indexed >= indexLargeThreshold:
		return 1
	case indexed >= indexMediumThreshold:
		return 2
	default:
		return hiddenExpandPerTick
	}
}

func maxConcurrentWalksForCount(indexed int) int {
	switch {
	case indexed >= indexLargeThreshold:
		return 1
	case indexed >= indexMediumThreshold:
		return 2
	default:
		return maxConcurrentWalks
	}
}

type hiddenState struct {
	pendingDirs        []string
	pendingDirSet      map[string]struct{}
	pendingFilePaths   []string
	pendingFilePathSet map[string]struct{}
	expandedRoots      map[string]struct{}
	filesSpliceAt      int
	expandNext         int
	expandPending      []string
}

func newHiddenState() *hiddenState {
	return &hiddenState{
		pendingDirSet:      make(map[string]struct{}),
		pendingFilePathSet: make(map[string]struct{}),
	}
}

func (h *hiddenState) mergeSkipped(dirs []string, filePaths []string) {
	for _, d := range dirs {
		d = filepath.Clean(d)
		if d == "" {
			continue
		}
		if _, ok := h.pendingDirSet[d]; ok {
			continue
		}
		h.pendingDirSet[d] = struct{}{}
		h.pendingDirs = append(h.pendingDirs, d)
	}
	for _, p := range filePaths {
		p = filepath.Clean(p)
		if p == "" {
			continue
		}
		if _, ok := h.pendingFilePathSet[p]; ok {
			continue
		}
		h.pendingFilePathSet[p] = struct{}{}
		h.pendingFilePaths = append(h.pendingFilePaths, p)
	}
}

func (h *hiddenState) spliceFilesBatch(displayRoot string) []Entry {
	if h.filesSpliceAt >= len(h.pendingFilePaths) {
		return nil
	}
	end := h.filesSpliceAt + hiddenFilesSpliceBatch
	if end > len(h.pendingFilePaths) {
		end = len(h.pendingFilePaths)
	}
	paths := h.pendingFilePaths[h.filesSpliceAt:end]
	h.filesSpliceAt = end
	batch := make([]Entry, 0, len(paths))
	for _, p := range paths {
		if e, ok := entryFromHiddenFilePath(displayRoot, p); ok {
			batch = append(batch, e)
		}
	}
	return batch
}

func (h *hiddenState) enqueueDirs(limit int) {
	if limit <= 0 || h.expandNext >= len(h.pendingDirs) {
		return
	}
	if h.expandedRoots == nil {
		h.expandedRoots = make(map[string]struct{})
	}
	for limit > 0 && h.expandNext < len(h.pendingDirs) {
		dir := filepath.Clean(h.pendingDirs[h.expandNext])
		h.expandNext++
		if dir == "" {
			continue
		}
		if _, ok := h.expandedRoots[dir]; ok {
			continue
		}
		h.expandedRoots[dir] = struct{}{}
		h.expandPending = append(h.expandPending, dir)
		limit--
	}
}

func pathInSelectionScope(path string, roots []string) bool {
	path = filepath.Clean(path)
	for _, r := range roots {
		r = filepath.Clean(r)
		if path == r || panel.IsStrictPathDescendant(r, path) {
			return true
		}
	}
	return false
}

func filterEntriesToScope(entries []Entry, displayRoot string, roots []string) []Entry {
	if len(roots) == 0 {
		return entries
	}
	displayRoot = filepath.Clean(displayRoot)
	filtered := make([]Entry, 0, len(entries))
	for _, e := range entries {
		abs := filepath.Clean(filepath.Join(displayRoot, filepath.FromSlash(e.RelLine)))
		if pathInSelectionScope(abs, roots) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// entryPathHidden reports whether abs would be skipped with IncludeHidden off.
func entryPathHidden(displayRoot, abs string) bool {
	displayRoot = filepath.Clean(displayRoot)
	abs = filepath.Clean(abs)
	if abs == "" {
		return false
	}
	rel, err := filepath.Rel(displayRoot, abs)
	if err != nil {
		return strings.HasPrefix(filepath.Base(abs), ".")
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return strings.HasPrefix(filepath.Base(abs), ".")
	}
	for _, part := range strings.Split(rel, "/") {
		if part != "" && strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// wasSkipped reports whether e was previously recorded as skipped (by
// dot-name or gitignore) during a walk with IncludeHidden off — either
// directly, or as a descendant of a skipped directory.
func (h *hiddenState) wasSkipped(displayRoot string, e Entry) bool {
	abs := filepath.Clean(filepath.Join(displayRoot, filepath.FromSlash(e.RelLine)))
	for _, d := range h.pendingDirs {
		if abs == d || panel.IsStrictPathDescendant(d, abs) {
			return true
		}
	}
	_, ok := h.pendingFilePathSet[e.RelLine]
	return ok
}

// stripHiddenAndSkipped is stripHiddenEntriesByName plus wasSkipped, used
// when toggling Include Hidden back off so gitignore-revealed entries are
// hidden again along with dot-named ones.
func (h *hiddenState) stripHiddenAndSkipped(entries []Entry, displayRoot string) []Entry {
	filtered := make([]Entry, 0, len(entries))
	for _, e := range entries {
		abs := filepath.Clean(filepath.Join(displayRoot, filepath.FromSlash(e.RelLine)))
		if entryPathHidden(displayRoot, abs) || h.wasSkipped(displayRoot, e) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func stripHiddenEntriesByName(entries []Entry, displayRoot string) []Entry {
	if len(entries) == 0 {
		return nil
	}
	filtered := make([]Entry, 0, len(entries))
	for _, e := range entries {
		abs := filepath.Clean(filepath.Join(displayRoot, filepath.FromSlash(e.RelLine)))
		if entryPathHidden(displayRoot, abs) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func dirEntryForHiddenDir(displayRoot, dir string) Entry {
	dir = filepath.Clean(dir)
	rel := relLine(displayRoot, dir)
	return Entry{
		RelLine: rel,
		IsDir:   true,
		Type:    localfs.EntryDirectory,
	}
}
