package dedup

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/selstrip"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// stripRows is the Selected strip's rows: the marked files, or their folded tree in tree mode.
func (h *Handler) stripRows() []selstrip.Row {
	return ui.DedupStripRows(h.model.DedupSnapshot, h.model.DedupView, h.model.SelectedStripTree())
}

// strip returns the Selected strip's state and rows while it has focus, else nil.
func (h *Handler) strip() (*selstrip.State, []selstrip.Row) {
	st := &h.model.DedupView
	if !st.FocusSelected {
		return nil, nil
	}
	return &st.Strip, h.stripRows()
}

// ToggleStripTree switches the session-wide Selected strip tree mode, keeping the cursor on the
// same row when it still exists.
func (h *Handler) ToggleStripTree() {
	st := &h.model.DedupView
	st.Strip.Remember(h.stripRows())
	h.model.SetSelectedStripTree(!h.model.SelectedStripTree())
	rows := h.stripRows()
	st.Strip.Restore(rows)
	st.Strip.Clamp(len(rows), 0)
}

// ToggleStripFocus focuses the Selected strip, or returns focus to the main tree when it
// already has it. It reports false when nothing is marked (no strip to focus).
func (h *Handler) ToggleStripFocus() bool {
	st := &h.model.DedupView
	if st.FocusSelected {
		h.focusPane(dedupPaneMain)
		return true
	}
	if st.MarkedCount == 0 {
		return false
	}
	h.focusPane(dedupPaneSelected)
	rows := h.stripRows()
	st.Strip.Restore(rows)
	st.Strip.Clamp(len(rows), 0)
	return true
}

// ReconcileStripFocus drops strip focus back to the main tree once nothing is marked, and
// remembers the cursor row for the next focus.
func (h *Handler) ReconcileStripFocus() {
	st := &h.model.DedupView
	if !st.FocusSelected {
		return
	}
	if st.MarkedCount == 0 {
		h.focusPane(dedupPaneMain)
		return
	}
	st.Strip.Remember(h.stripRows())
}

// stripRow is the strip row under the cursor.
func (h *Handler) stripRow() (selstrip.Row, bool) {
	rows := h.stripRows()
	cur := h.model.DedupView.Strip.Cursor
	if cur < 0 || cur >= len(rows) {
		return selstrip.Row{}, false
	}
	return rows[cur], true
}

// UnmarkStripRow clears the delete mark of the strip row under the cursor; a tree-mode folder
// row clears every mark under that folder.
func (h *Handler) UnmarkStripRow() {
	row, ok := h.stripRow()
	if !ok {
		return
	}
	if !row.Group {
		h.unmarkWhere(func(p string) bool { return p == row.Path })
		return
	}
	prefix := strings.TrimSuffix(row.Path, "/") + "/"
	h.unmarkWhere(func(p string) bool { return strings.HasPrefix(p, prefix) })
}

// unmarkWhere clears the delete mark of every marked file whose path matches.
func (h *Handler) unmarkWhere(match func(path string) bool) {
	st := &h.model.DedupView
	for _, g := range h.model.DedupSnapshot.Groups {
		for _, f := range g.Files {
			if p := f.Abs.String(); st.Marked[p] && match(p) {
				h.setMark(p, g.Size, false)
			}
		}
	}
	// Marks not in the snapshot any more: drop them with no reclaim bytes to adjust.
	for p := range st.Marked {
		if match(p) {
			h.setMark(p, 0, false)
		}
	}
}

// RevealStripRow selects the strip row's file (or, for a folder row, its directory row, else the
// first file under it) in the main tree, expanding the folders above it, and moves focus there.
func (h *Handler) RevealStripRow() {
	row, ok := h.stripRow()
	if !ok {
		return
	}
	st := &h.model.DedupView
	id, ancestors, found := ui.DedupRevealPath(h.model.DedupSnapshot, *st, row.Path)
	if !found && row.Group {
		rows := h.stripRows()
		for j := st.Strip.Cursor + 1; j < len(rows) && rows[j].Depth > row.Depth && !found; j++ {
			if !rows[j].Group {
				id, ancestors, found = ui.DedupRevealPath(h.model.DedupSnapshot, *st, rows[j].Path)
			}
		}
	}
	if !found {
		h.host.SetTransientMessage("File is hidden by the current filters", ui.MessageUrgencyInfo)
		return
	}
	h.focusPane(dedupPaneMain)
	for _, a := range ancestors {
		st.Main.SetCollapsed(a, false)
	}
	h.model.DedupList, st.IgnoredEmptyCount = ui.DedupRowsFromSnapshot(h.model.DedupSnapshot, *st)
	if i := ui.DedupRowIndexByID(h.model.DedupList, id); i >= 0 {
		st.Main.Selected = i
	}
	h.syncCopies()
	h.ensureSelectionVisible(0)
}
