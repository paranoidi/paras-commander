package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// tryRenameDialogShortcut handles [dialog.rename] while the main
// rename dialog (name field) is active. Returns true when the event was consumed.
func (a *App) tryRenameDialogShortcut(ev *tcell.EventKey) bool {
	if a.keysRenameDialog == nil {
		return false
	}
	d := &a.model.FileDialog
	if !d.Open || !ui.FileDialogHasRenamePhase(d.DialogType) || d.RenamePhase != ui.RenamePhaseMain {
		return false
	}
	id, ok := a.keysRenameDialog.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionFileRenameOpenSanitize:
		d.RenamePhase = ui.RenamePhaseSanitize
		d.RenameSanitizeDots = true
		d.RenameSanitizeUnderscores = true
		d.FocusedField = ui.FileDialogOKFocusIndex(*d)
		return true
	case keymap.ActionFileRenameOpenSlugify:
		d.RenamePhase = ui.RenamePhaseSlugify
		d.RenameSlugifySep = ui.RenameSlugifyDot
		d.FocusedField = ui.FileDialogOKFocusIndex(*d)
		return true
	default:
		return false
	}
}

func (a *App) renameDialogFooterEligible() bool {
	if a.keysRenameDialog == nil {
		return false
	}
	d := &a.model.FileDialog
	if !d.Open || !ui.FileDialogHasRenamePhase(d.DialogType) || d.RenamePhase != ui.RenamePhaseMain {
		return false
	}
	return a.keysRenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenSanitize) != "" ||
		a.keysRenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenSlugify) != ""
}

func (a *App) closeRenameToolPhase() {
	d := &a.model.FileDialog
	d.RenamePhase = ui.RenamePhaseMain
	d.FocusedField = 0
}

func (a *App) applyRenameToolAndReturnMain() {
	d := &a.model.FileDialog
	if len(d.Fields) < 1 {
		a.closeRenameToolPhase()
		return
	}
	f := &d.Fields[0]
	switch d.RenamePhase {
	case ui.RenamePhaseSanitize:
		f.Value = ui.ApplyRenameSanitize(f.Value, d.RenameSanitizeDots, d.RenameSanitizeUnderscores)
	case ui.RenamePhaseSlugify:
		f.Value = ui.ApplyRenameSlugify(f.Value, d.RenameSlugifySep)
	}
	f.Cursor = len([]rune(f.Value))
	f.PrefillPending = false
	d.RenamePhase = ui.RenamePhaseMain
	d.FocusedField = 0
}
