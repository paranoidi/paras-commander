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

// StashSaveFromSelection copies the current selection into the stash and clears live selection.
// Returns false when there is nothing to stash.
func (s *State) StashSaveFromSelection() bool {
	if len(s.SelectedPaths) == 0 {
		return false
	}
	paths := make([]string, 0, len(s.SelectedPaths))
	for p := range s.SelectedPaths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	s.SelectionStashPaths = paths
	if len(s.SelectionsStripOrder) == 0 {
		s.SelectionStashStripOrder = nil
	} else {
		s.SelectionStashStripOrder = append([]string(nil), s.SelectionsStripOrder...)
	}
	s.ClearSelection()
	return true
}

// StashClear removes any stashed selection on this panel.
func (s *State) StashClear() {
	s.SelectionStashPaths = nil
	s.SelectionStashStripOrder = nil
}

// ApplySelectionSnapshot replaces the live selection with paths and strip order.
// Paths that no longer exist on disk are omitted (stash restore).
func (s *State) ApplySelectionSnapshot(paths, stripOrder []string) {
	s.SelectedPaths = nil
	s.SelectionsStripOrder = nil
	paths = filterExistingSelectionPaths(paths)
	if len(paths) == 0 {
		s.normalizeSelectionsStripCursor()
		return
	}
	s.SelectedPaths = make(map[string]bool, len(paths))
	for _, p := range paths {
		s.SelectedPaths[p] = true
	}
	filteredStrip := filterSelectionsStripOrder(stripOrder, s.SelectedPaths)
	if len(filteredStrip) != len(stripOrder) {
		s.SelectionsStripOrder = nil
	} else {
		s.SelectionsStripOrder = filteredStrip
	}
	s.normalizeSelectionsStripCursor()
}

// MergeSelectionSnapshot unions stash paths and strip order into the live selection.
// Paths that no longer exist on disk are omitted (stash restore).
func (s *State) MergeSelectionSnapshot(paths, stripOrder []string) {
	paths = filterExistingSelectionPaths(paths)
	if len(paths) == 0 {
		return
	}
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool, len(paths))
	}
	for _, p := range paths {
		s.SelectedPaths[p] = true
	}
	seen := make(map[string]bool, len(s.SelectionsStripOrder)+len(stripOrder))
	merged := make([]string, 0, len(s.SelectionsStripOrder)+len(stripOrder))
	for _, p := range s.SelectionsStripOrder {
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
		s.SelectionsStripOrder = nil
	} else {
		s.SelectionsStripOrder = merged
	}
	s.normalizeSelectionsStripCursor()
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

func filterSelectionsStripOrder(stripOrder []string, selected map[string]bool) []string {
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
