package panel

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/search"
)

// StripFilterHasMatches reports whether the active strip quick filter has at least one match.
func (s *State) StripFilterHasMatches() bool {
	return s.StripFilter.HasMatches()
}

// StripMatchRanges returns highlighted rune ranges for the strip row (basename match ranges).
func (s State) StripMatchRanges(index int) []search.Range {
	return s.StripFilter.Ranges(index)
}

// OpenStripFilter starts editing the selections-strip quick filter.
func (s *State) OpenStripFilter(stripViewportRows int) {
	s.StripFilter.Editing = true
	s.rebuildStripFilter()
	s.EnsureSelectionsStripCursorVisible(stripViewportRows)
}

// AcceptStripFilter exits editing while keeping the current filtered strip cursor.
func (s *State) AcceptStripFilter(stripViewportRows int) {
	s.StripFilter.Editing = false
	s.StripFilter.Active = s.StripFilter.Query != ""
	if !s.StripFilter.Active {
		s.StripFilter.clearResults()
	}
	s.EnsureSelectionsStripCursorVisible(stripViewportRows)
}

// CancelStripFilter exits editing and clears the strip filter query.
func (s *State) CancelStripFilter(stripViewportRows int) {
	s.StripFilter.Editing = false
	s.applyStripFilterQuery("", stripViewportRows)
}

// ClearStripFilter removes the query while preserving edit mode.
func (s *State) ClearStripFilter(stripViewportRows int) {
	editing := s.StripFilter.Editing
	s.applyStripFilterQuery("", stripViewportRows)
	s.StripFilter.Editing = editing
}

// AppendStripFilterRune appends a printable rune to the strip filter query.
func (s *State) AppendStripFilterRune(value rune, stripViewportRows int) {
	s.StripFilter.Editing = true
	s.applyStripFilterQuery(s.StripFilter.Query+string(value), stripViewportRows)
}

// BackspaceStripFilter removes the last rune from the strip filter query.
func (s *State) BackspaceStripFilter(stripViewportRows int) {
	runes := []rune(s.StripFilter.Query)
	if len(runes) == 0 {
		s.StripFilter.Editing = false
		return
	}
	s.StripFilter.Editing = true
	s.applyStripFilterQuery(string(runes[:len(runes)-1]), stripViewportRows)
}

// CycleStripFilterMatch moves the strip cursor through fuzzy matches (or plain Move when none).
func (s *State) CycleStripFilterMatch(delta int, stripViewportRows int) {
	cur, ok := s.StripFilter.Cycle(s.SelectionsStripCursor, delta)
	if !ok {
		s.MoveSelectionsStrip(delta, stripViewportRows)
		return
	}
	s.SelectionsStripCursor = cur
	s.EnsureSelectionsStripCursorVisible(stripViewportRows)
}

func (s *State) applyStripFilterQuery(query string, stripViewportRows int) {
	if cur, ok := s.StripFilter.Apply(query, s.stripFilterNames()); ok {
		s.SelectionsStripCursor = cur
	}
	s.EnsureSelectionsStripCursorVisible(stripViewportRows)
}

func (s *State) stripFilterNames() []string {
	paths := s.SelectionsStripPaths()
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = filepath.Base(p)
	}
	return names
}

func (s *State) rebuildStripFilter() {
	s.StripFilter.Rebuild(s.stripFilterNames())
}

// StripFilterActiveUI reports whether the strip quick filter is editing or has an active query.
func (s State) StripFilterActiveUI() bool {
	return s.StripFilter.Active || s.StripFilter.Editing
}

// MapStripBasenameRangesToDisplay offsets basename match ranges onto a display path that
// ends with that basename (common for Rel labels). Returns nil when the basename is not a suffix.
func MapStripBasenameRangesToDisplay(display, absPath string, baseRanges []search.Range) []search.Range {
	if len(baseRanges) == 0 {
		return nil
	}
	base := filepath.Base(absPath)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return nil
	}
	dispRunes := []rune(display)
	baseRunes := []rune(base)
	if len(baseRunes) == 0 || len(dispRunes) < len(baseRunes) {
		return nil
	}
	offset := len(dispRunes) - len(baseRunes)
	if string(dispRunes[offset:]) != base {
		// Truncated display — try last occurrence of basename as a substring.
		idx := strings.LastIndex(display, base)
		if idx < 0 {
			return nil
		}
		offset = utf8RuneCountPrefix(display, idx)
	}
	out := make([]search.Range, len(baseRanges))
	for i, r := range baseRanges {
		out[i] = search.Range{Start: r.Start + offset, End: r.End + offset}
	}
	return out
}

func utf8RuneCountPrefix(s string, byteIndex int) int {
	if byteIndex <= 0 {
		return 0
	}
	if byteIndex > len(s) {
		byteIndex = len(s)
	}
	return len([]rune(s[:byteIndex]))
}
