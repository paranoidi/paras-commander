package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// tryRenameDialogShortcut handles [dialog.rename] while the main
// rename dialog (name field) is active. Returns true when the event was consumed.
func (a *App) tryRenameDialogShortcut(ev *tcell.EventKey) bool {
	if a.keys.RenameDialog == nil {
		return false
	}
	d := &a.model.FileDialog
	if !d.Open || !dialog.FileDialogHasRenamePhase(d.DialogType) || d.RenamePhase != dialog.RenamePhaseMain {
		return false
	}
	id, ok := a.keys.RenameDialog.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionFileRenameOpenSanitize:
		d.RenamePhase = dialog.RenamePhaseSanitize
		d.RenameSanitizeDots = true
		d.RenameSanitizeUnderscores = true
		d.FocusedField = dialog.FileDialogOKFocusIndex(*d)
		return true
	case keymap.ActionFileRenameOpenSlugify:
		d.RenamePhase = dialog.RenamePhaseSlugify
		d.RenameSlugifySep = dialog.RenameSlugifyDot
		d.FocusedField = dialog.FileDialogOKFocusIndex(*d)
		return true
	case keymap.ActionFileRenameOpenEncoding:
		if len(d.RenameEncodingCandidates) == 0 {
			return false
		}
		d.RenamePhase = dialog.RenamePhaseEncoding
		if d.RenameEncodingSelected < 0 || d.RenameEncodingSelected >= len(d.RenameEncodingCandidates) {
			d.RenameEncodingSelected = 0
		}
		d.FocusedField = dialog.FileDialogOKFocusIndex(*d)
		return true
	default:
		return false
	}
}

func (a *App) renameDialogFooterEligible() bool {
	if a.keys.RenameDialog == nil {
		return false
	}
	d := &a.model.FileDialog
	if !d.Open || !dialog.FileDialogHasRenamePhase(d.DialogType) || d.RenamePhase != dialog.RenamePhaseMain {
		return false
	}
	return a.keys.RenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenSanitize) != "" ||
		a.keys.RenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenSlugify) != "" ||
		(len(d.RenameEncodingCandidates) > 0 && a.keys.RenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenEncoding) != "")
}

func (a *App) renameEncodingFooterEligible() bool {
	d := &a.model.FileDialog
	return d.Open && dialog.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase == dialog.RenamePhaseMain &&
		len(d.RenameEncodingCandidates) > 0 && a.keys.RenameDialog != nil &&
		a.keys.RenameDialog.MenuBindingLabel(keymap.ActionFileRenameOpenEncoding) != ""
}

func (a *App) closeRenameToolPhase() {
	d := &a.model.FileDialog
	d.RenamePhase = dialog.RenamePhaseMain
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
	case dialog.RenamePhaseSanitize:
		f.Value = dialog.ApplyRenameSanitize(f.Value, d.RenameSanitizeDots, d.RenameSanitizeUnderscores)
	case dialog.RenamePhaseSlugify:
		f.Value = dialog.ApplyRenameSlugify(f.Value, d.RenameSlugifySep)
	case dialog.RenamePhaseEncoding:
		idx := d.RenameEncodingSelected
		if idx < 0 || idx >= len(d.RenameEncodingCandidates) {
			a.closeRenameToolPhase()
			return
		}
		f.Value = d.RenameEncodingCandidates[idx].UTF8
	}
	f.Cursor = len([]rune(f.Value))
	f.PrefillPending = false
	d.RenamePhase = dialog.RenamePhaseMain
	d.FocusedField = 0
}

func (a *App) selectRenameEncodingAtFocus() {
	d := &a.model.FileDialog
	if d.FocusedField < 0 || d.FocusedField >= len(d.RenameEncodingCandidates) {
		return
	}
	d.RenameEncodingSelected = d.FocusedField
}
