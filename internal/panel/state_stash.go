package panel

import (
	"maps"
	"slices"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// StashEmpty reports whether this panel has no stashed selection.
func (s State) StashEmpty() bool {
	return len(s.SelectionStashPaths) == 0
}

// StashPathCount returns the number of paths in the stash.
func (s State) StashPathCount() int {
	return len(s.SelectionStashPaths)
}

// SelectedPathCount returns the number of live selected paths.
func (s State) SelectedPathCount() int {
	return len(s.SelectedPaths)
}

// StashSaveFromSelection moves the current selection into the stash and clears live selection.
// A non-empty stash is merged with the selection. Returns false when
// there is nothing to stash.
func (s *State) StashSaveFromSelection() bool {
	if len(s.SelectedPaths) == 0 {
		return false
	}
	hadStash := len(s.SelectionStashPaths) > 0
	paths := slices.AppendSeq(slices.Clone(s.SelectionStashPaths), maps.Keys(s.SelectedPaths))
	slices.Sort(paths)
	s.SelectionStashPaths = slices.Compact(paths)
	if !hadStash || s.SelectionStashStripCursorPath == "" {
		s.SelectionStashStripCursorPath = s.Strip.CursorPath
	}
	s.ClearSelection()
	return true
}

// StashClear removes any stashed selection on this panel.
func (s *State) StashClear() {
	s.SelectionStashPaths = nil
	s.SelectionStashStripCursorPath = ""
}

// ApplySelectionSnapshot replaces the live selection with paths.
// Paths that no longer exist on disk are omitted (stash restore).
func (s *State) ApplySelectionSnapshot(paths []string, cursorPath string) {
	s.clearSelectionState()
	s.Strip.CursorPath = cursorPath
	paths = filterExistingSelectionPaths(paths)
	if len(paths) == 0 {
		s.invalidateSelectionDerived()
		s.normalizeSelectedStripCursor()
		return
	}
	s.SelectedPaths = make(map[string]bool, len(paths))
	for _, p := range paths {
		s.SelectedPaths[p] = true
	}
	s.rebuildSelectedDirPaths()
	s.recomputeSelectionListedBytes()
	s.invalidateSelectionDerived()
	s.normalizeSelectedStripCursor()
}

// MergeSelectionSnapshot unions stash paths into the live selection.
// Paths that no longer exist on disk are omitted (stash restore).
func (s *State) MergeSelectionSnapshot(paths []string, cursorPath string) {
	paths = filterExistingSelectionPaths(paths)
	if len(paths) == 0 {
		return
	}
	if s.Strip.CursorPath == "" {
		s.Strip.CursorPath = cursorPath
	}
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool, len(paths))
	}
	for _, p := range paths {
		s.SelectedPaths[p] = true
	}
	s.rebuildSelectedDirPaths()
	s.recomputeSelectionListedBytes()
	s.invalidateSelectionDerived()
	s.normalizeSelectedStripCursor()
}

func filterExistingSelectionPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		p = cleanPathString(p)
		if p == "" {
			continue
		}
		if _, err := localfs.EntryFromPath(p); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}
