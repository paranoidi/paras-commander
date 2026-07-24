package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (a *App) pathPickerHostFooterEligible() bool {
	if a.model.FileDialog.Open {
		if a.dialogCtrl.FileDialogOnButton() {
			return false
		}
		f := a.dialogCtrl.FocusedField()
		return f != nil && f.PathPicker
	}
	if a.model.TransferDialog.Open &&
		a.model.TransferDialog.Phase == dialog.TransferPhaseDestination &&
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
	if a.keys.Global == nil {
		return false
	}
	id, ok := a.keys.Global.Lookup(ev)
	if !ok || id != keymap.ActionBookmarkOpen {
		return false
	}
	if a.model.FileDialog.Open {
		if a.dialogCtrl.FileDialogOnButton() {
			return false
		}
		f := a.dialogCtrl.FocusedField()
		if f == nil || !f.PathPicker {
			return false
		}
		a.openPathPickerForFileField(a.model.FileDialog.FocusedField)
		return true
	}
	d := &a.model.TransferDialog
	if d.Open && d.Phase == dialog.TransferPhaseDestination && d.FocusField == 0 {
		a.openPathPickerForTransfer()
		return true
	}
	if a.model.FlattenDialog.Open && a.model.FlattenDialog.FocusField == 0 {
		a.openPathPickerForFlatten()
		return true
	}
	return false
}
