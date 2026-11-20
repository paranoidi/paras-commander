// Package selstrip holds the cursor, scroll and quick-filter state of the Selected strip,
// shared by the file panel and the find-duplicates view. The caller owns the path list.
package selstrip

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/search"
)

// State is the strip's view state; rows come from the caller (n rows / paths).
type State struct {
	Cursor int
	Scroll int
	// CursorPath is the last focused row path; restores the cursor on re-focus.
	CursorPath string
	// Filter is the strip quick filter (basename fuzzy match).
	Filter quickfilter.Filter
}

// Names returns the basename of each path (the strip filter's match text).
func Names(paths []string) []string {
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = filepath.Base(p)
	}
	return names
}

// Clamp keeps Cursor inside n rows and Scroll inside the viewport (vr <= 0 skips viewport math).
func (s *State) Clamp(n, vr int) {
	if n == 0 {
		s.Cursor = 0
		s.Scroll = 0
		return
	}
	if s.Cursor < 0 {
		s.Cursor = 0
	}
	if s.Cursor >= n {
		s.Cursor = n - 1
	}
	if vr <= 0 {
		if s.Scroll > s.Cursor {
			s.Scroll = s.Cursor
		}
		return
	}
	maxOffset := n - vr
	if maxOffset < 0 {
		maxOffset = 0
		s.Scroll = 0
	}
	if s.Scroll > maxOffset {
		s.Scroll = maxOffset
	}
	if s.Scroll < 0 {
		s.Scroll = 0
	}
	if s.Cursor < s.Scroll {
		s.Scroll = s.Cursor
	}
	if s.Cursor >= s.Scroll+vr {
		s.Scroll = s.Cursor - vr + 1
	}
}

// Move shifts the cursor by delta and keeps it visible.
func (s *State) Move(delta, n, vr int) {
	if n == 0 {
		s.Cursor = 0
		s.Scroll = 0
		return
	}
	s.Cursor += delta
	s.Clamp(n, vr)
}

// Top moves the cursor to the first row.
func (s *State) Top(n, vr int) {
	if n == 0 {
		return
	}
	s.Cursor = 0
	s.Scroll = 0
	s.Clamp(n, vr)
}

// Bottom moves the cursor to the last row.
func (s *State) Bottom(n, vr int) {
	if n == 0 {
		return
	}
	s.Cursor = n - 1
	s.Clamp(n, vr)
}

// Remember records the path under the cursor.
func (s *State) Remember(paths []string) {
	if s.Cursor >= 0 && s.Cursor < len(paths) {
		s.CursorPath = paths[s.Cursor]
	}
}

// Restore moves the cursor back to CursorPath when it is still listed.
func (s *State) Restore(paths []string) {
	if s.CursorPath == "" {
		return
	}
	for i, p := range paths {
		if p == s.CursorPath {
			s.Cursor = i
			return
		}
	}
}

// OpenFilter starts editing the quick filter.
func (s *State) OpenFilter(names []string, opts quickfilter.Options, vr int) {
	s.Filter.Editing = true
	s.Filter.Rebuild(names, opts)
	s.Clamp(len(names), vr)
}

// AcceptFilter exits editing while keeping the current filtered cursor.
func (s *State) AcceptFilter(n, vr int) {
	s.Filter.Editing = false
	s.Filter.Active = s.Filter.Query != ""
	if !s.Filter.Active {
		s.Filter.ClearResults()
	}
	s.Clamp(n, vr)
}

// CancelFilter exits editing and clears the query.
func (s *State) CancelFilter(names []string, opts quickfilter.Options, vr int) {
	s.Filter.Editing = false
	s.applyQuery("", names, opts, vr)
}

// ClearFilter removes the query while preserving edit mode.
func (s *State) ClearFilter(names []string, opts quickfilter.Options, vr int) {
	editing := s.Filter.Editing
	s.applyQuery("", names, opts, vr)
	s.Filter.Editing = editing
}

// AppendFilterRune appends a printable rune to the query.
func (s *State) AppendFilterRune(r rune, names []string, opts quickfilter.Options, vr int) {
	s.applyQuery(s.Filter.InsertRune(r), names, opts, vr)
}

// BackspaceFilter removes the last rune from the query.
func (s *State) BackspaceFilter(names []string, opts quickfilter.Options, vr int) {
	if next, changed := s.Filter.Backspace(); changed {
		s.applyQuery(next, names, opts, vr)
	}
}

// CycleFilterMatch moves the cursor through fuzzy matches (or plain Move when none).
func (s *State) CycleFilterMatch(delta, n, vr int, opts quickfilter.Options) {
	cur, ok := s.Filter.Cycle(s.Cursor, delta, opts)
	if !ok {
		s.Move(delta, n, vr)
		return
	}
	s.Cursor = cur
	s.Clamp(n, vr)
}

func (s *State) applyQuery(query string, names []string, opts quickfilter.Options, vr int) {
	if cur, ok := s.Filter.Apply(query, names, opts); ok {
		s.Cursor = cur
	}
	s.Clamp(len(names), vr)
}

// MapBasenameRangesToDisplay offsets basename match ranges onto a display path that
// ends with that basename (common for Rel labels). Returns nil when the basename is not a suffix.
func MapBasenameRangesToDisplay(display, absPath string, baseRanges []search.Range) []search.Range {
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
		offset = len([]rune(display[:idx]))
	}
	out := make([]search.Range, len(baseRanges))
	for i, r := range baseRanges {
		out[i] = search.Range{Start: r.Start + offset, End: r.End + offset}
	}
	return out
}
