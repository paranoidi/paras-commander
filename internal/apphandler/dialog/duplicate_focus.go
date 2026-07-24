package dialog

import "path/filepath"

func (h *Handler) scheduleDuplicateFocus(panelID int, listDir, name string) {
	h.duplicateFocus = duplicateFocusPending{
		panelID: panelID,
		listDir: listDir,
		name:    name,
	}
}

func (h *Handler) applyDuplicateFocusPending() {
	f := h.duplicateFocus
	if f.name == "" {
		return
	}
	p := h.host.PanelByID(f.panelID)
	if p == nil {
		h.duplicateFocus = duplicateFocusPending{}
		return
	}
	if filepath.Clean(p.PathString()) != filepath.Clean(f.listDir) {
		h.duplicateFocus = duplicateFocusPending{}
		return
	}
	if p.SelectVisibleEntryCentered(f.name, h.host.PanelViewportRows(f.panelID)) {
		h.duplicateFocus = duplicateFocusPending{}
	}
}
