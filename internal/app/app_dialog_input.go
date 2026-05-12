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
