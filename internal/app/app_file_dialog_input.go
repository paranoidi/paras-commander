package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) handleFileDialogKey(event *tcell.EventKey) bool {
	// Alt+O = OK, Alt+C = Cancel
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'o', 'O':
			a.executeFileDialog()
			return false
		case 'c', 'C':
			a.closeFileDialog()
			return false
		case 'y', 'Y':
			if a.model.FileDialog.DialogType == ui.FileDialogDelete {
				a.executeDelete()
				return false
			}
		case 'n', 'N':
			if a.model.FileDialog.DialogType == ui.FileDialogDelete {
				a.closeFileDialog()
				return false
			}
		}
	}

	if a.tryPathPickerHostShortcut(event) {
		return false
	}

	onRadio := a.fileDialogOnRadio()

	f := a.focusedField()
	if !onRadio && f != nil {
		if f.PathPicker && f.PickerFocused {
			if a.tryDialogInputRestore(event, f) {
				return false
			}
		} else if a.tryDialogInputFieldActions(event, f) {
			return false
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeFileDialog()
		return false
	case tcell.KeyEnter:
		if a.model.FileDialog.DialogType == ui.FileDialogDelete {
			if a.model.FileDialog.FocusedField == 0 {
				a.executeDelete()
			} else {
				a.closeFileDialog()
			}
			return false
		}
		if f := a.focusedField(); f != nil && f.PathPicker && f.PickerFocused {
			a.openPathPickerForFileField(a.model.FileDialog.FocusedField)
			return false
		}
		if onRadio {
			a.selectFocusedMkdirRadio()
		}
		a.executeFileDialog()
		return false
	case tcell.KeyDown:
		a.fileDialogFocusNext()
		return false
	case tcell.KeyUp:
		a.fileDialogFocusPrev()
		return false
	case tcell.KeyLeft:
		// On button: move between buttons; on radio: no-op; on field: move cursor
		if a.fileDialogOnButton() {
			a.fileDialogFocusButton(-1)
		} else if onRadio {
			return false
		} else if f := a.focusedField(); f != nil && f.PathPicker && f.PickerFocused {
			f.PickerFocused = false
			runes := []rune(f.Value)
			f.Cursor = len(runes)
		} else {
			a.fileDialogMoveCursor(-1)
		}
		return false
	case tcell.KeyRight:
		if a.fileDialogOnButton() {
			a.fileDialogFocusButton(1)
		} else if onRadio {
			return false
		} else if f := a.focusedField(); f != nil && f.PathPicker && !f.PickerFocused {
			runes := []rune(f.Value)
			c := f.Cursor
			if c < 0 {
				c = 0
			}
			if c > len(runes) {
				c = len(runes)
			}
			if f.Prefill != "" && f.PrefillPending && f.Value == f.Prefill && c >= len(runes) {
				f.CommitPrefill()
				return false
			}
			if c >= len(runes) {
				f.PickerFocused = true
			} else {
				a.fileDialogMoveCursor(1)
			}
		} else {
			a.fileDialogMoveCursor(1)
		}
		return false
	case tcell.KeyHome:
		if onRadio {
			return false
		}
		a.fileDialogMoveCursorStart()
		return false
	case tcell.KeyEnd:
		if onRadio {
			return false
		}
		a.fileDialogMoveCursorEnd()
		return false
	case tcell.KeyTab:
		a.fileDialogFocusNext()
		return false
	case tcell.KeyBacktab:
		a.fileDialogFocusPrev()
		return false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if onRadio {
			return false
		}
		a.fileDialogBackspace()
		return false
	case tcell.KeyDelete:
		if onRadio {
			return false
		}
		a.fileDialogDelete()
		return false
	case tcell.KeyCtrlL:
		if onRadio {
			return false
		}
		a.fileDialogClearField()
		return false
	case tcell.KeyRune:
		if onRadio {
			if isPlainPrintableRune(event) && event.Rune() == ' ' {
				a.selectFocusedMkdirRadio()
			}
			return false
		}
		if isPlainPrintableRune(event) {
			a.fileDialogInsertRune(event.Rune())
		}
		return false
	}
	return false
}

// selectFocusedMkdirRadio commits the currently focused mkdir radio row as the
// active MkdirAction. No-op when focus is not on a radio row.
func (a *App) selectFocusedMkdirRadio() {
	idx := a.fileDialogRadioIndex()
	if idx < 0 {
		return
	}
	switch idx {
	case 0:
		a.model.FileDialog.MkdirAction = ui.MkdirActionCreate
	case 1:
		a.model.FileDialog.MkdirAction = ui.MkdirActionCreateCopySelect
	case 2:
		a.model.FileDialog.MkdirAction = ui.MkdirActionCreateMoveSelect
	}
}

func (a *App) focusedField() *ui.FileDialogField {
	if a.model.FileDialog.FocusedField < 0 || a.model.FileDialog.FocusedField >= len(a.model.FileDialog.Fields) {
		return nil
	}
	return &a.model.FileDialog.Fields[a.model.FileDialog.FocusedField]
}

func (a *App) fileDialogInsertRune(r rune) {
	field := a.focusedField()
	field.InsertRune(r)
}

func (a *App) fileDialogBackspace() {
	field := a.focusedField()
	field.Backspace()
}

func (a *App) fileDialogDelete() {
	field := a.focusedField()
	field.Delete()
}

func (a *App) fileDialogClearField() {
	field := a.focusedField()
	field.Clear()
}

func (a *App) fileDialogMoveCursor(delta int) {
	field := a.focusedField()
	field.MoveCursor(delta)
}

func (a *App) fileDialogMoveCursorStart() {
	field := a.focusedField()
	field.MoveCursorStart()
}

func (a *App) fileDialogMoveCursorEnd() {
	field := a.focusedField()
	field.MoveCursorEnd()
}

// fileDialogFocusCount returns total focusable items in file dialog.
func (a *App) fileDialogFocusCount() int {
	if a.model.FileDialog.DialogType == ui.FileDialogDelete {
		return 2 // Yes, No
	}
	return len(a.model.FileDialog.Fields) + a.mkdirExtraFocusRows() + 2 // fields + (optional) mkdir radios + OK + Cancel
}

// mkdirExtraFocusRows returns the number of extra focus rows contributed by the
// mkdir-with-selections radio section, or 0 when not applicable.
func (a *App) mkdirExtraFocusRows() int {
	d := &a.model.FileDialog
	if d.DialogType == ui.FileDialogMkdir && d.MkdirShowActions {
		return 3
	}
	return 0
}

// fileDialogOnRadio returns true when focus sits on the mkdir post-action radio
// section (between text fields and the OK button).
func (a *App) fileDialogOnRadio() bool {
	d := &a.model.FileDialog
	extra := a.mkdirExtraFocusRows()
	if extra == 0 {
		return false
	}
	base := len(d.Fields)
	return d.FocusedField >= base && d.FocusedField < base+extra
}

// fileDialogRadioIndex returns the 0-based radio index when focus is on a
// mkdir radio row, or -1 otherwise.
func (a *App) fileDialogRadioIndex() int {
	if !a.fileDialogOnRadio() {
		return -1
	}
	return a.model.FileDialog.FocusedField - len(a.model.FileDialog.Fields)
}

// fileDialogOnButton returns true if current focus is on a button (not a field/radio).
func (a *App) fileDialogOnButton() bool {
	d := &a.model.FileDialog
	if d.DialogType == ui.FileDialogDelete {
		return true // delete only has buttons
	}
	return d.FocusedField >= len(d.Fields)+a.mkdirExtraFocusRows()
}

// fileDialogFocusNext moves focus to next item. Down on last button = no-op.
func (a *App) fileDialogFocusNext() {
	for i := range a.model.FileDialog.Fields {
		a.model.FileDialog.Fields[i].PickerFocused = false
	}
	count := a.fileDialogFocusCount()
	if count <= 1 {
		return
	}
	next := a.model.FileDialog.FocusedField + 1
	if next >= count {
		return // no wrap from last button
	}
	a.model.FileDialog.FocusedField = next
}

// fileDialogFocusPrev moves focus to previous item. Up on first item = no-op.
func (a *App) fileDialogFocusPrev() {
	for i := range a.model.FileDialog.Fields {
		a.model.FileDialog.Fields[i].PickerFocused = false
	}
	if a.model.FileDialog.FocusedField <= 0 {
		return // no wrap from first field
	}
	a.model.FileDialog.FocusedField--
}

// fileDialogFocusButton moves focus between buttons only (Left/Right).
func (a *App) fileDialogFocusButton(delta int) {
	d := &a.model.FileDialog
	if d.DialogType == ui.FileDialogDelete {
		// Yes(0), No(1): move between them, no wrap
		next := d.FocusedField + delta
		if next < 0 || next >= 2 {
			return
		}
		d.FocusedField = next
		return
	}
	// Fields + (optional radios) + OK + Cancel: move between OK/Cancel only
	okIdx := len(d.Fields) + a.mkdirExtraFocusRows()
	cancelIdx := okIdx + 1
	if d.FocusedField == okIdx && delta == 1 {
		d.FocusedField = cancelIdx
	} else if d.FocusedField == cancelIdx && delta == -1 {
		d.FocusedField = okIdx
	}
	// Otherwise stay
}
