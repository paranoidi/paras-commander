package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// handleFileDialogFieldKey applies standard text-editing keys to a FileDialogField.
// afterEdit runs after any mutation (e.g. mass-rename preview recompute, path completion sync).
// Returns true when the event was consumed.
func (a *App) handleFileDialogFieldKey(ev *tcell.EventKey, f *ui.FileDialogField, afterEdit func()) bool {
	if f == nil {
		return false
	}
	if a.tryDialogInputFieldActions(ev, f) {
		if afterEdit != nil {
			afterEdit()
		}
		return true
	}
	edited := false
	switch ev.Key() {
	case tcell.KeyLeft:
		f.MoveCursor(-1)
		edited = true
	case tcell.KeyRight:
		f.MoveCursor(1)
		edited = true
	case tcell.KeyHome:
		f.MoveCursorStart()
		edited = true
	case tcell.KeyEnd:
		f.MoveCursorEnd()
		edited = true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		f.Backspace()
		edited = true
	case tcell.KeyDelete:
		f.Delete()
		edited = true
	case tcell.KeyCtrlL:
		f.Clear()
		edited = true
	case tcell.KeyRune:
		if isDialogInputRune(ev) {
			f.InsertRune(ev.Rune())
			edited = true
		}
	}
	if edited {
		if afterEdit != nil {
			afterEdit()
		}
		return true
	}
	return false
}

func (a *App) fileDialogFieldAfterEdit() func() {
	switch a.model.FileDialog.DialogType {
	case ui.FileDialogMassRename:
		return a.recomputeMassRenamePreview
	case ui.FileDialogRunForEach:
		return a.recomputeRunForEachCommandValidation
	default:
		return nil
	}
}
