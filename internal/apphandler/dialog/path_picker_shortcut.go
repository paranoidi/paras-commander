package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// PathPickerHostFooterEligible reports whether the currently focused dialog row is a
// path-picker-capable field (file dialog path field, or the transfer/flatten destination
// text row), so the footer can advertise the bookmark, history, pinned and all-paths shortcuts.
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

// TryPathPickerHostShortcut opens a bookmarks, history, pinned, or combined path picker when
// the user presses bookmark.open, panel.history-dialog, panel.pin-dialog, or
// ui.input.path-picker-all while a path-picker host row is focused.
func (h *Handler) TryPathPickerHostShortcut(ev *tcell.EventKey) bool {
	var kind pathPickerListKind
	if id, ok := h.lookupPathPickerAction(ev); ok {
		switch id {
		case keymap.ActionBookmarkOpen:
			kind = pathPickerListBookmarks
		case keymap.ActionPanelHistoryDialog:
			kind = pathPickerListHistory
		case keymap.ActionPanelPinDialog:
			kind = pathPickerListPinned
		case keymap.ActionDialogInputPathPickerAll:
			kind = pathPickerListAll
		default:
			return false
		}
	} else {
		return false
	}
	if !h.PathPickerHostFooterEligible() {
		return false
	}
	switch {
	case h.model.FileDialog.Open:
		h.OpenPathPickerForFileField(h.model.FileDialog.FocusedField, kind)
	case h.model.TransferDialog.Open:
		h.OpenPathPickerForTransfer(kind)
	default:
		h.OpenPathPickerForFlatten(kind)
	}
	return true
}

// lookupPathPickerAction resolves ev against the dialog-input overlay first (the combined
// picker), then the global map.
func (h *Handler) lookupPathPickerAction(ev *tcell.EventKey) (string, bool) {
	if h.keysDialogInput != nil {
		if id, ok := h.keysDialogInput.Lookup(ev); ok && id == keymap.ActionDialogInputPathPickerAll {
			return id, true
		}
	}
	if h.keysGlobal == nil {
		return "", false
	}
	return h.keysGlobal.Lookup(ev)
}
