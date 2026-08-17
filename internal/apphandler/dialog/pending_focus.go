package dialog

import "path/filepath"

// schedulePanelFocus defers select-and-center on name until the listing at listDir actually
// contains it: rename/mkdir/duplicate all trigger a directory reload immediately beforehand, and
// that reload is asynchronous (internal/panel's ScheduleAsyncLoad — a local path can be a slow
// network mount just as easily as an sftp:// one), so name may not exist in the panel's Entries
// yet at the point this is scheduled. applyPendingPanelFocus (called from App.reconcileAfterEvent
// after every event, including the reload's own completion) retries until it either succeeds or
// the panel has moved on to a different directory.
func (h *Handler) schedulePanelFocus(panelID int, listDir, name string) {
	h.schedulePanelFocusScroll(panelID, listDir, name, true)
}

// schedulePanelFocusScroll is schedulePanelFocus with an explicit centered choice — mkdir's
// plain-create case wants SelectVisibleEntryInViewport (respects the panel's scroll mode)
// instead of always forcing a center.
func (h *Handler) schedulePanelFocusScroll(panelID int, listDir, name string, centered bool) {
	h.pendingFocus = pendingPanelFocus{
		panelID:  panelID,
		listDir:  listDir,
		name:     name,
		centered: centered,
	}
}

// applyPendingPanelFocus retries the deferred select-and-center scheduled by schedulePanelFocus.
// A no-op once nothing is pending, once the target panel has navigated elsewhere, or once the
// entry is found and selected.
func (h *Handler) applyPendingPanelFocus() {
	f := h.pendingFocus
	if f.name == "" {
		return
	}
	p := h.host.PanelByID(f.panelID)
	if p == nil {
		h.pendingFocus = pendingPanelFocus{}
		return
	}
	if filepath.Clean(p.PathString()) != filepath.Clean(f.listDir) {
		h.pendingFocus = pendingPanelFocus{}
		return
	}
	vr := h.host.PanelViewportRows(f.panelID)
	var found bool
	if f.centered {
		found = p.SelectVisibleEntryCentered(f.name, vr)
	} else {
		found = p.SelectVisibleEntryInViewport(f.name, vr)
	}
	if found {
		h.pendingFocus = pendingPanelFocus{}
	}
}
