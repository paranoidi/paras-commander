package panel

import "github.com/paranoidi/paras-commander/internal/localfs"

// EntryFilter narrows which entries are visible in a panel's file list/tree
// view. Subsystems plug in by constructing one and calling SetEntryFilter.
type EntryFilter struct {
	ID    string // stable id, e.g. "git-staged"
	Label string // bottom-border indicator text, e.g. "Filter: staged"
	Match func(entry localfs.Entry, s *State) bool
	// Applicable reports whether the filter still makes sense for the current listing (e.g. a
	// git-status filter outside a git work tree). Nil means always applicable. Checked whenever
	// the filter is set and after every listing load; when it returns false the filter clears
	// itself instead of leaving the panel showing zero entries.
	Applicable func(s *State) bool
}

// SetEntryFilter installs f (nil clears filtering) and rebuilds the visible-entry index.
func (s *State) SetEntryFilter(f *EntryFilter) {
	if f != nil && f.Applicable != nil && !f.Applicable(s) {
		f = nil
	}
	s.ActiveEntryFilter = f
	s.rebuildFilter()
	s.clampCursor()
}

// ClearEntryFilter removes any active entry filter.
func (s *State) ClearEntryFilter() {
	s.SetEntryFilter(nil)
}

// RefreshEntryFilter re-evaluates the active filter, e.g. after async git status data merges in.
func (s *State) RefreshEntryFilter() {
	if s.ActiveEntryFilter == nil {
		return
	}
	s.rebuildFilter()
	s.clampCursor()
}

// NoteTreeChildGitStatusApplied decrements the in-flight tree-child git-status counter and
// reports whether it just reached zero. Tree-mode directory expansion (notably Ctrl+Alt+Right's
// expand-all-shallow) can dispatch one git-status fetch per newly-loaded directory, all
// completing independently over a short span; calling the full RefreshEntryFilter (which
// recomputes filteredIdx and, in tree mode, filteredTreeShape — both O(visible tree size)) on
// every single one of those arrivals, plus the render each triggers, is what made expand-all
// visibly flicker and slow down while a filter was active. Callers should call
// RefreshEntryFilter only when this returns true, coalescing N arrivals into one refresh.
func (s *State) NoteTreeChildGitStatusApplied() bool {
	if s.gitStatusChildPending > 0 {
		s.gitStatusChildPending--
	}
	return s.gitStatusChildPending == 0
}

// ClearFilterIfInapplicable clears ActiveEntryFilter when its Applicable check fails for the
// current listing (e.g. leaving a git work tree with a git-status filter active) — otherwise the
// panel would render as empty instead of the filter clearing itself.
func (s *State) ClearFilterIfInapplicable() {
	f := s.ActiveEntryFilter
	if f == nil || f.Applicable == nil || f.Applicable(s) {
		return
	}
	s.ActiveEntryFilter = nil
	s.filteredIdx = nil
}

// translateVisibleIndex maps a display-space index (0..VisibleEntryCount()-1) to the raw backing
// index (s.Entries in flat mode, s.treeRows in tree mode), honoring the active entry filter. ok
// is false when index is out of range. VisibleEntry and TreeRowShape both go through this so a
// filtered display position never gets used to index the raw backing slice directly.
func (s State) translateVisibleIndex(index int) (int, bool) {
	if s.ActiveEntryFilter == nil {
		return index, index >= 0
	}
	if index < 0 || index >= len(s.filteredIdx) {
		return 0, false
	}
	return s.filteredIdx[index], true
}

// rawIndexForCursor returns the raw backing index (s.Entries in flat mode, s.treeRows in tree
// mode) for the cursor's current display position. Tree-mode cursor operations (expand/collapse/
// sibling-jump) need this instead of using s.Cursor directly against s.treeRows, since s.Cursor is
// a filtered display position once a filter is active.
func (s State) rawIndexForCursor() (int, bool) {
	return s.translateVisibleIndex(s.Cursor)
}

// cursorForRawIndex returns the display-position cursor value for backing index rawIdx, honoring
// the active entry filter. ok is false when rawIdx is currently filtered out (not visible).
func (s State) cursorForRawIndex(rawIdx int) (int, bool) {
	if s.ActiveEntryFilter == nil {
		return rawIdx, true
	}
	for i, v := range s.filteredIdx {
		if v == rawIdx {
			return i, true
		}
	}
	return 0, false
}

// rebuildEntryFilter recomputes filteredIdx from the current unfiltered entry space.
// Must run before rebuildFilter's own logic, which ranks through VisibleEntryCount/VisibleEntry
// and therefore needs filteredIdx already up to date.
func (s *State) rebuildEntryFilter() {
	if s.ActiveEntryFilter == nil {
		s.filteredIdx = nil
		s.filteredTreeShape = nil
		return
	}
	if s.ListLayout == ListLayoutTree {
		s.filteredIdx = make([]int, 0, len(s.treeRows))
		for i := range s.treeRows {
			if s.ActiveEntryFilter.Match(s.treeRows[i].Value.Entry, s) {
				s.filteredIdx = append(s.filteredIdx, i)
			}
		}
		s.recomputeFilteredTreeConnectors()
		return
	}
	s.filteredIdx = make([]int, 0, len(s.Entries))
	for i := range s.Entries {
		if s.ActiveEntryFilter.Match(s.Entries[i], s) {
			s.filteredIdx = append(s.filteredIdx, i)
		}
	}
	s.filteredTreeShape = nil
}
