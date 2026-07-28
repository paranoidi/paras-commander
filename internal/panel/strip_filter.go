package panel

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/paranoidi/paras-commander/internal/search"
)

// StripFilterHasMatches reports whether the active strip quick filter has at least one match.
func (s *State) StripFilterHasMatches() bool {
	return len(s.StripFilter.results) > 0
}

// StripMatchRanges returns highlighted rune ranges for the strip row (basename match ranges).
func (s State) StripMatchRanges(index int) []search.Range {
	if !s.StripFilter.Active {
		return nil
	}
	for _, result := range s.StripFilter.results {
		if result.Index == index {
			return result.Ranges
		}
	}
	return nil
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
		s.StripFilter.results = nil
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
	if len(s.StripFilter.results) == 0 {
		s.MoveSelectionsStrip(delta, stripViewportRows)
		return
	}
	order := s.stripFilterResultsCycleOrder()
	n := len(order)
	cur := -1
	for i := range order {
		if order[i].Index == s.SelectionsStripCursor {
			cur = i
			break
		}
	}
	if cur < 0 {
		if delta > 0 {
			cur = nextFilterMatchIndex(order, s.SelectionsStripCursor)
		} else {
			cur = previousFilterMatchIndex(order, s.SelectionsStripCursor)
		}
	} else {
		cur = (cur + delta) % n
		if cur < 0 {
			cur += n
		}
	}
	s.SelectionsStripCursor = order[cur].Index
	s.EnsureSelectionsStripCursorVisible(stripViewportRows)
}

func (s *State) stripFilterResultsCycleOrder() []filterResult {
	if s.StripFilter.cycleMatchesRanked() {
		return s.StripFilter.results
	}
	out := make([]filterResult, len(s.StripFilter.results))
	copy(out, s.StripFilter.results)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Index < out[j].Index
	})
	return out
}

func (s *State) applyStripFilterQuery(query string, stripViewportRows int) {
	s.StripFilter.Query = query
	s.StripFilter.Active = query != ""
	s.rebuildStripFilter()
	if len(s.StripFilter.results) > 0 {
		s.SelectionsStripCursor = primaryFilterMatchIndex(query, s.StripFilter.results)
	}
	s.EnsureSelectionsStripCursorVisible(stripViewportRows)
}

func (s *State) rebuildStripFilter() {
	s.StripFilter.results = nil
	if s.StripFilter.Query == "" {
		s.StripFilter.Active = false
		return
	}
	query := search.Parse(s.StripFilter.Query)
	if query.Empty() {
		s.StripFilter.Active = false
		return
	}
	paths := s.SelectionsStripPaths()
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = filepath.Base(p)
	}
	ranked := query.Rank(names, search.Options{CaseInsensitive: s.StripFilter.CaseInsensitive})
	s.StripFilter.results = make([]filterResult, 0, len(ranked))
	for _, result := range ranked {
		s.StripFilter.results = append(s.StripFilter.results, filterResult{
			Index:  result.Index,
			Score:  result.Result.Score,
			Ranges: result.Result.Ranges,
		})
	}
	s.StripFilter.Active = true
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
