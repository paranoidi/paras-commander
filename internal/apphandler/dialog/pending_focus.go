package dialog

import "path/filepath"

// schedulePanelFocus defers select-and-center on name until the listing at listDir actually
// contains it. Used only by duplicate: AddTransferJob only enqueues the copy, so the duplicated
// file doesn't exist on disk (or in the panel's Entries) at the point this is scheduled, and it
// can't be tied to one specific reload the way rename/mkdir's RefreshBothPanelsWithFocus hook can
// — it lands later, off of the job's own terminal-event refresh (jobs.Handler's ApplyRefreshes).
// applyPendingPanelFocus (called from App.reconcileAfterEvent after every event, including that
// later refresh) retries until it either succeeds or the panel has moved on to a different directory.
func (h *Handler) schedulePanelFocus(panelID int, listDir, name string) {
	h.pendingFocus = pendingPanelFocus{
		panelID: panelID,
		listDir: listDir,
		name:    name,
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
	if p.SelectVisibleEntryCentered(f.name, vr) {
		h.pendingFocus = pendingPanelFocus{}
	}
}
