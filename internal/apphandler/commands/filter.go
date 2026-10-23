package commands

import (
	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Type-to-jump quick filter over the Commands list (same engine and keys as the panel and
// Dedup filters). Match indices are CommandsList indices; rows only append, so an open
// filter's indices stay valid and new rows join the ranking on the next keystroke.

// Filter returns a copy of the list's quick filter state.
func (h *Handler) Filter() quickfilter.Filter { return h.model.CommandsView.Filter }

// applyFilter sets the query and jumps the cursor to the best match.
func (h *Handler) applyFilter(query string) {
	h.mu.RLock()
	labels := make([]string, len(h.model.CommandsList))
	for i, e := range h.model.CommandsList {
		labels[i] = ui.CommandsFilterLabel(e, h.model.UserHomeDir)
	}
	h.mu.RUnlock()
	if cur, ok := h.model.CommandsView.Filter.Apply(query, labels, h.host.FilterOptions()); ok {
		h.setSelection(cur)
	}
}

// AppendFilterRune types r into the quick filter.
func (h *Handler) AppendFilterRune(r rune) {
	h.applyFilter(h.model.CommandsView.Filter.InsertRune(r))
}

// BackspaceFilter deletes the rune before the filter caret.
func (h *Handler) BackspaceFilter() {
	if next, changed := h.model.CommandsView.Filter.Backspace(); changed {
		h.applyFilter(next)
	}
}

// ClearFilter empties the query while the filter stays open.
func (h *Handler) ClearFilter() { h.applyFilter("") }

// CancelFilter closes the quick filter.
func (h *Handler) CancelFilter() { h.model.CommandsView.Filter = quickfilter.Filter{} }

// CycleFilterMatch steps the cursor through filter matches (positive = down the screen, toward
// older runs; in ranked mode, down the ranking), or moves plainly when nothing matches.
func (h *Handler) CycleFilterMatch(delta int) {
	cv := &h.model.CommandsView
	opts := h.host.FilterOptions()
	step := delta
	if !opts.CycleRanked {
		step = -delta // match order is storage order, the reverse of the screen
	}
	if cur, ok := cv.Filter.Cycle(cv.Selected, step, opts); ok {
		h.setSelection(cur)
		return
	}
	h.MoveSelection(delta)
}
