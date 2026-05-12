package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) pathPickerHostFooterEligible() bool {
	if a.keysPathPickerHost == nil {
		return false
	}
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
	return false
}

// tryPathPickerHostShortcut handles [path_picker_host_action_keys] (default F9 → open path picker)
// while copy/move or symlink/hardlink dialogs host a path row.
func (a *App) tryPathPickerHostShortcut(ev *tcell.EventKey) bool {
	if a.keysPathPickerHost == nil {
		return false
	}
	id, ok := a.keysPathPickerHost.Lookup(ev)
	if !ok || id != keymap.ActionUIOpenPathPicker {
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
	return false
}
