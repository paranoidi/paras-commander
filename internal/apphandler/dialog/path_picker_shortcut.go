package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// PathPickerHostFooterEligible reports whether the currently focused dialog row is a
// path-picker-capable field (file dialog path field, or the transfer/flatten destination
// text row), so the footer can advertise bookmark.open and panel.history-dialog shortcuts.
func (h *Handler) PathPickerHostFooterEligible() bool {
	if h.model.FileDialog.Open {
		if h.FileDialogOnButton() {
			return false
		}
		f := h.FocusedField()
		return f != nil && f.PathPicker
	}
	if h.model.TransferDialog.Open &&
		h.model.TransferDialog.Phase == dialog.TransferPhaseDestination &&
		h.model.TransferDialog.FocusField == 0 {
		return true
	}
	if h.model.FlattenDialog.Open && h.model.FlattenDialog.FocusField == 0 {
		return true
	}
	return false
}

// TryPathPickerHostShortcut opens a bookmarks-only or history-only path picker when the
// user presses bookmark.open or panel.history-dialog while a path-picker host row is focused.
func (h *Handler) TryPathPickerHostShortcut(ev *tcell.EventKey) bool {
	if h.keysGlobal == nil {
		return false
	}
	id, ok := h.keysGlobal.Lookup(ev)
	if !ok {
		return false
	}
	var bookmarks bool
	switch id {
	case keymap.ActionBookmarkOpen:
		bookmarks = true
	case keymap.ActionPanelHistoryDialog:
		bookmarks = false
	default:
		return false
	}
	if h.model.FileDialog.Open {
		if h.FileDialogOnButton() {
			return false
		}
		f := h.FocusedField()
		if f == nil || !f.PathPicker {
			return false
		}
		if bookmarks {
			h.OpenPathPickerForFileFieldBookmarks(h.model.FileDialog.FocusedField)
		} else {
			h.OpenPathPickerForFileFieldHistory(h.model.FileDialog.FocusedField)
		}
		return true
	}
	d := &h.model.TransferDialog
	if d.Open && d.Phase == dialog.TransferPhaseDestination && d.FocusField == 0 {
		if bookmarks {
			h.OpenPathPickerForTransferBookmarks()
		} else {
			h.OpenPathPickerForTransferHistory()
		}
		return true
	}
	if h.model.FlattenDialog.Open && h.model.FlattenDialog.FocusField == 0 {
		if bookmarks {
			h.OpenPathPickerForFlattenBookmarks()
		} else {
			h.OpenPathPickerForFlattenHistory()
		}
		return true
	}
	return false
}
