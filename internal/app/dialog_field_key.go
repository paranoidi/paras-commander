package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// handleFileDialogFieldKey applies standard text-editing keys to a FileDialogField.
// afterEdit runs after any mutation (e.g. mass-rename preview recompute, path completion sync).
// Returns true when the event was consumed.
func (a *App) handleFileDialogFieldKey(ev *tcell.EventKey, f *dialog.FileDialogField, afterEdit func()) bool {
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
		if scrollquery.IsDialogInputRune(ev) {
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
	var extra func()
	switch a.model.FileDialog.DialogType {
	case dialog.FileDialogMassRename:
		extra = a.recomputeMassRenamePreview
	case dialog.FileDialogRunForEach:
		extra = a.recomputeRunForEachCommandValidation
	}
	f := a.focusedField()
	if f == nil || !f.PathPicker {
		return extra
	}
	textWidth := a.transferDestinationTextWidth()
	return func() {
		if extra != nil {
			extra()
		}
		a.syncPathFieldCompletion(f, textWidth)
	}
}

func (a *App) syncFocusedFileDialogPathFieldCompletion() {
	f := a.focusedField()
	if f == nil || !f.PathPicker {
		return
	}
	a.syncPathFieldCompletion(f, a.transferDestinationTextWidth())
}
