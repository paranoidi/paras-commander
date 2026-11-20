package dedup

import (
	"github.com/paranoidi/paras-commander/internal/selstrip"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// strip returns the Selected strip's state and rows while it has focus, else nil.
func (h *Handler) strip() (*selstrip.State, []string) {
	st := &h.model.DedupView
	if !st.FocusSelected {
		return nil, nil
	}
	return &st.Strip, ui.DedupMarkedPaths(*st)
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
	paths := ui.DedupMarkedPaths(*st)
	st.Strip.Restore(paths)
	st.Strip.Clamp(len(paths), 0)
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
	st.Strip.Remember(ui.DedupMarkedPaths(*st))
}

// stripPath is the marked path under the strip cursor.
func (h *Handler) stripPath() (string, bool) {
	st := &h.model.DedupView
	paths := ui.DedupMarkedPaths(*st)
	if st.Strip.Cursor < 0 || st.Strip.Cursor >= len(paths) {
		return "", false
	}
	return paths[st.Strip.Cursor], true
}

// UnmarkStripRow clears the delete mark of the strip row under the cursor.
func (h *Handler) UnmarkStripRow() {
	path, ok := h.stripPath()
	if !ok {
		return
	}
	for _, g := range h.model.DedupSnapshot.Groups {
		for _, f := range g.Files {
			if f.Abs.String() == path {
				h.setMark(path, g.Size, false)
				return
			}
		}
	}
	// Not in the snapshot any more: drop the stale mark with no reclaim bytes to adjust.
	h.setMark(path, 0, false)
}

// RevealStripRow selects the strip row's file in the main tree (expanding the folders above
// it) and moves focus there.
func (h *Handler) RevealStripRow() {
	path, ok := h.stripPath()
	if !ok {
		return
	}
	st := &h.model.DedupView
	id, ancestors, found := ui.DedupRevealPath(h.model.DedupSnapshot, *st, path)
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
