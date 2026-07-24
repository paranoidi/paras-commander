package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// PathPickerHostFooterEligible reports whether the currently focused dialog row is a
// path-picker-capable field (file dialog path field, or the transfer/flatten destination
// text row), so the footer can advertise the bookmark.open shortcut for opening the fuzzy
// path/history picker.
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

// TryPathPickerHostShortcut opens the fuzzy path/history picker when the user presses
// a chord bound to bookmark.open (Open bookmarks) while a path-picker host row is focused.
func (h *Handler) TryPathPickerHostShortcut(ev *tcell.EventKey) bool {
	if h.keysGlobal == nil {
		return false
	}
	id, ok := h.keysGlobal.Lookup(ev)
	if !ok || id != keymap.ActionBookmarkOpen {
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
		h.OpenPathPickerForFileField(h.model.FileDialog.FocusedField)
		return true
	}
	d := &h.model.TransferDialog
	if d.Open && d.Phase == dialog.TransferPhaseDestination && d.FocusField == 0 {
		h.OpenPathPickerForTransfer()
		return true
	}
	if h.model.FlattenDialog.Open && h.model.FlattenDialog.FocusField == 0 {
		h.OpenPathPickerForFlatten()
		return true
	}
	return false
}
