package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) pathPickerHostFooterEligible() bool {
	if a.model.FileDialog.Open {
		if a.fileDialogOnButton() {
			return false
		}
		f := a.focusedField()
		return f != nil && f.PathPicker
	}
	if a.model.TransferDialog.Open &&
		a.model.TransferDialog.Phase == ui.TransferPhaseDestination &&
		a.model.TransferDialog.FocusField == 0 {
		return true
	}
	if a.model.FlattenDialog.Open && a.model.FlattenDialog.FocusField == 0 {
		return true
	}
	return false
}

// tryPathPickerHostShortcut opens the fuzzy path/history picker when the user presses
// a chord bound to bookmark.open (Open bookmarks) while a path-picker host row is focused.
func (a *App) tryPathPickerHostShortcut(ev *tcell.EventKey) bool {
	if a.keys == nil {
		return false
	}
	id, ok := a.keys.Lookup(ev)
	if !ok || id != keymap.ActionBookmarkOpen {
		return false
	}
	if a.model.FileDialog.Open {
		if a.fileDialogOnButton() {
			return false
		}
		f := a.focusedField()
		if f == nil || !f.PathPicker {
			return false
		}
		a.openPathPickerForFileField(a.model.FileDialog.FocusedField)
		return true
	}
	d := &a.model.TransferDialog
	if d.Open && d.Phase == ui.TransferPhaseDestination && d.FocusField == 0 {
		a.openPathPickerForTransfer()
		return true
	}
	if a.model.FlattenDialog.Open && a.model.FlattenDialog.FocusField == 0 {
		a.openPathPickerForFlatten()
		return true
	}
	return false
}
