package panel

import "github.com/paranoidi/paras-commander/internal/selstrip"

// OpenStripFilter starts editing the selected-strip quick filter.
func (s *State) OpenStripFilter(stripViewportRows int) {
	s.Strip.OpenFilter(s.stripNames(), s.filterOptions(), stripViewportRows)
}

// AcceptStripFilter exits editing while keeping the current filtered strip cursor.
func (s *State) AcceptStripFilter(stripViewportRows int) {
	s.Strip.AcceptFilter(s.SelectedStripCount(), stripViewportRows)
}

// CancelStripFilter exits editing and clears the strip filter query.
func (s *State) CancelStripFilter(stripViewportRows int) {
	s.Strip.CancelFilter(s.stripNames(), s.filterOptions(), stripViewportRows)
}

// ClearStripFilter removes the query while preserving edit mode.
func (s *State) ClearStripFilter(stripViewportRows int) {
	s.Strip.ClearFilter(s.stripNames(), s.filterOptions(), stripViewportRows)
}

// AppendStripFilterRune appends a printable rune to the strip filter query.
func (s *State) AppendStripFilterRune(value rune, stripViewportRows int) {
	s.Strip.AppendFilterRune(value, s.stripNames(), s.filterOptions(), stripViewportRows)
}

// BackspaceStripFilter removes the last rune from the strip filter query.
func (s *State) BackspaceStripFilter(stripViewportRows int) {
	s.Strip.BackspaceFilter(s.stripNames(), s.filterOptions(), stripViewportRows)
}

// CycleStripFilterMatch moves the strip cursor through fuzzy matches (or plain Move when none).
func (s *State) CycleStripFilterMatch(delta int, stripViewportRows int) {
	s.Strip.CycleFilterMatch(delta, s.SelectedStripCount(), stripViewportRows, s.filterOptions())
}

// stripNames is the strip filter's match text, one basename per row.
func (s *State) stripNames() []string { return selstrip.RowNames(s.SelectedStripRows()) }
