package panel

import (
	"sort"

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
// A non-empty stash is merged with the selection (stash strip order first). Returns false when
// there is nothing to stash.
func (s *State) StashSaveFromSelection() bool {
	if len(s.SelectedPaths) == 0 {
		return false
	}
	seen := make(map[string]bool, len(s.SelectionStashPaths)+len(s.SelectedPaths))
	paths := make([]string, 0, len(s.SelectionStashPaths)+len(s.SelectedPaths))
	for _, p := range s.SelectionStashPaths {
		seen[p] = true
		paths = append(paths, p)
	}
	for p := range s.SelectedPaths {
		if !seen[p] {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	hadStash := len(s.SelectionStashPaths) > 0
	s.SelectionStashPaths = paths
	var strip []string
	stripSeen := make(map[string]bool, len(s.SelectionStashStripOrder)+len(s.SelectedStripOrder))
	for _, p := range append(append([]string(nil), s.SelectionStashStripOrder...), s.SelectedStripOrder...) {
		if !stripSeen[p] {
			stripSeen[p] = true
			strip = append(strip, p)
		}
	}
	s.SelectionStashStripOrder = strip
	if !hadStash || s.SelectionStashStripCursorPath == "" {
		s.SelectionStashStripCursorPath = s.StripCursorPath
	}
	s.ClearSelection()
	return true
}

// StashClear removes any stashed selection on this panel.
func (s *State) StashClear() {
	s.SelectionStashPaths = nil
	s.SelectionStashStripOrder = nil
	s.SelectionStashStripCursorPath = ""
}

// ApplySelectionSnapshot replaces the live selection with paths and strip order.
// Paths that no longer exist on disk are omitted (stash restore).
func (s *State) ApplySelectionSnapshot(paths, stripOrder []string, cursorPath string) {
	s.clearSelectionState()
	s.SelectedStripOrder = nil
	s.StripCursorPath = cursorPath
	paths = filterExistingSelectionPaths(paths)
	if len(paths) == 0 {
		s.invalidateSelectionDerivedFull()
		s.normalizeSelectedStripCursor()
		return
	}
	s.SelectedPaths = make(map[string]bool, len(paths))
	for _, p := range paths {
		s.SelectedPaths[p] = true
	}
	s.rebuildSelectedDirPaths()
	s.recomputeSelectionListedBytes()
	filteredStrip := filterSelectedStripOrder(stripOrder, s.SelectedPaths)
	if len(filteredStrip) != len(stripOrder) {
		s.SelectedStripOrder = nil
	} else {
		s.SelectedStripOrder = filteredStrip
	}
	s.invalidateSelectionDerivedFull()
	s.normalizeSelectedStripCursor()
}

// MergeSelectionSnapshot unions stash paths and strip order into the live selection.
// Paths that no longer exist on disk are omitted (stash restore).
func (s *State) MergeSelectionSnapshot(paths, stripOrder []string, cursorPath string) {
	paths = filterExistingSelectionPaths(paths)
	if len(paths) == 0 {
		return
	}
	if s.StripCursorPath == "" {
		s.StripCursorPath = cursorPath
	}
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool, len(paths))
	}
	for _, p := range paths {
		s.SelectedPaths[p] = true
	}
	seen := make(map[string]bool, len(s.SelectedStripOrder)+len(stripOrder))
	merged := make([]string, 0, len(s.SelectedStripOrder)+len(stripOrder))
	for _, p := range s.SelectedStripOrder {
		if seen[p] || s.SelectedPaths == nil || !s.SelectedPaths[p] {
			continue
		}
		merged = append(merged, p)
		seen[p] = true
	}
	for _, p := range stripOrder {
		if seen[p] || s.SelectedPaths == nil || !s.SelectedPaths[p] {
			continue
		}
		merged = append(merged, p)
		seen[p] = true
	}
	if len(merged) == 0 {
		s.SelectedStripOrder = nil
	} else {
		s.SelectedStripOrder = merged
	}
	s.rebuildSelectedDirPaths()
	s.recomputeSelectionListedBytes()
	s.invalidateSelectionDerivedFull()
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

func filterSelectedStripOrder(stripOrder []string, selected map[string]bool) []string {
	if len(stripOrder) == 0 || len(selected) == 0 {
		return nil
	}
	out := make([]string, 0, len(stripOrder))
	for _, p := range stripOrder {
		p = cleanPathString(p)
		if p != "" && selected[p] {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
