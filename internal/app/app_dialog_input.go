package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// tryDialogInputRestore handles [dialog_input_action_keys] chords while a
// dialog input field is focused. Currently the only action is
// ui.input.restore-default, which restores the focused field's suggested
// default (Prefill) and re-arms PrefillPending so the next printable rune
// replaces from scratch.
//
// Returns true when the chord matched and the field state changed. A
// non-matching chord, a nil field, or a field with no Prefill all return false
// so the caller can fall through to its normal handling.
func (a *App) tryDialogInputRestore(ev *tcell.EventKey, f *ui.FileDialogField) bool {
	if a.keysDialogInput == nil || f == nil {
		return false
	}
	id, ok := a.keysDialogInput.Lookup(ev)
	if !ok || id != keymap.ActionDialogInputRestoreDefault {
		return false
	}
	return f.RestorePrefill()
}

// dialogInputRestoreFooterEligible reports whether the footer should show the
// preferred restore-default shortcut: a focused dialog text field whose
// Prefill is non-empty (same contexts where restore is meaningful).
func (a *App) dialogInputRestoreFooterEligible() bool {
	if a.keysDialogInput == nil {
		return false
	}
	if lbl := a.keysDialogInput.MenuBindingLabel(keymap.ActionDialogInputRestoreDefault); lbl == "" {
		return false
	}
	if a.model.FileDialog.Open {
		if a.fileDialogOnButton() || a.fileDialogOnRadio() {
			return false
		}
		f := a.focusedField()
		return f != nil && f.Prefill != ""
	}
	if a.model.TransferDialog.Open {
		d := &a.model.TransferDialog
		if d.FocusField != 0 {
			return false
		}
		switch d.Phase {
		case ui.TransferPhaseSelfCopyRename:
			return d.SelfCopyNewName.Prefill != ""
		case ui.TransferPhaseDestination:
			if d.DestSubFocus != ui.TransferDestSubFocusText {
				return false
			}
			return d.Destination.Prefill != ""
		default:
			return false
		}
	}
	return false
}

// tryDialogInputFieldActions handles [dialog_input_action_keys] for focused
// dialog text fields (restore default, word motion, backward kill word).
// Returns true when the chord matched a dialog-input action (even when the
// edit was a no-op), so the caller should not fall through to generic key handling.
func (a *App) tryDialogInputFieldActions(ev *tcell.EventKey, f *ui.FileDialogField) bool {
	if a.keysDialogInput == nil || f == nil {
		return false
	}
	id, ok := a.keysDialogInput.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionDialogInputRestoreDefault:
		return f.RestorePrefill()
	case keymap.ActionDialogInputKillWordBackward:
		f.KillWordBackward()
		return true
	case keymap.ActionDialogInputBackwardWord:
		f.MoveWordBackward()
		return true
	case keymap.ActionDialogInputForwardWord:
		f.MoveWordForward()
		return true
	default:
		return false
	}
}

// tryDialogInputPathPickerQuery applies [dialog_input_action_keys] to the path
// picker filter row when list focus is on the query (focus == 0).
func (a *App) tryDialogInputPathPickerQuery(ev *tcell.EventKey) bool {
	st := &a.model.PathPicker
	if !st.Open || st.Focus != 0 || a.keysDialogInput == nil {
		return false
	}
	id, ok := a.keysDialogInput.Lookup(ev)
	if !ok {
		return false
	}
	runes := []rune(st.Query)
	pos := st.QueryCursor
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}
	switch id {
	case keymap.ActionDialogInputKillWordBackward:
		newRunes, newPos := ui.KillWordBackward(runes, pos)
		st.Query = string(newRunes)
		st.QueryCursor = newPos
		a.ensurePathPickerQueryVisible()
		a.syncPathPickerRanks()
		a.armPathPickerValidateTimer()
		st.Selected = 0
		ui.EnsurePathPickerListScroll(st, a.pathPickerListRows())
		return true
	case keymap.ActionDialogInputBackwardWord:
		st.QueryCursor = ui.BackwardWordIndex(runes, pos)
		a.ensurePathPickerQueryVisible()
		return true
	case keymap.ActionDialogInputForwardWord:
		st.QueryCursor = ui.ForwardWordIndex(runes, pos)
		a.ensurePathPickerQueryVisible()
		return true
	case keymap.ActionDialogInputRestoreDefault:
		return false
	default:
		return false
	}
}
