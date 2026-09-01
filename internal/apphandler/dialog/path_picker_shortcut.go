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

// PathPickerPinnedFooterEligible reports whether the Pinned path-picker shortcut should be
// advertised in the footer. Unlike Bookmarks/History (all three path-picker hosts), Pinned is
// scoped to the copy/move (transfer) dialog's destination field only.
func (h *Handler) PathPickerPinnedFooterEligible() bool {
	d := &h.model.TransferDialog
	return d.Open && d.Phase == dialog.TransferPhaseDestination && d.FocusField == 0
}

// TryPathPickerHostShortcut opens a bookmarks-only, history-only, or (transfer destination
// field only) pinned-directories-only path picker when the user presses bookmark.open,
// panel.history-dialog, or panel.pin-dialog while a path-picker host row is focused.
func (h *Handler) TryPathPickerHostShortcut(ev *tcell.EventKey) bool {
	if h.keysGlobal == nil {
		return false
	}
	id, ok := h.keysGlobal.Lookup(ev)
	if !ok {
		return false
	}
	if id == keymap.ActionPanelPinDialog {
		if h.PathPickerPinnedFooterEligible() {
			h.OpenPathPickerForTransferPinned()
			return true
		}
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
