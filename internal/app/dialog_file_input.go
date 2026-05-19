package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) handleFileDialogKey(event *tcell.EventKey) bool {
	d := &a.model.FileDialog
	if d.Open && d.DialogType == ui.FileDialogRename && d.RenamePhase != ui.RenamePhaseMain {
		return a.handleRenameToolKey(event)
	}
	if a.tryRenameDialogShortcut(event) {
		return false
	}
	if d.Open && d.DialogType == ui.FileDialogMassRename && event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 's', 'S':
			d.MassRenameMode = ui.MassRenameModeUISimple
			d.FocusedField = 0
			a.massRenameSyncFieldLabels()
			a.recomputeMassRenamePreview()
			return false
		case 'r', 'R':
			d.MassRenameMode = ui.MassRenameModeUIRegex
			d.FocusedField = 1
			a.massRenameSyncFieldLabels()
			a.recomputeMassRenamePreview()
			return false
		case 'i', 'I':
			if d.MassRenameMode == ui.MassRenameModeUISimple {
				d.MassRenameCaseFold = !d.MassRenameCaseFold
				a.recomputeMassRenamePreview()
			}
			return false
		}
	}
	if d.Open && d.DialogType == ui.FileDialogMassRename {
		switch event.Key() {
		case tcell.KeyPgUp:
			_, h := a.screen.Size()
			vp := ui.MassRenamePreviewViewportRows(h)
			d.MassRenamePreviewScroll -= vp
			if d.MassRenamePreviewScroll < 0 {
				d.MassRenamePreviewScroll = 0
			}
			return false
		case tcell.KeyPgDn:
			_, h := a.screen.Size()
			vp := ui.MassRenamePreviewViewportRows(h)
			ui.MassRenameEnsurePreviewScroll(d, vp, len(d.MassRenamePreviewBefore))
			d.MassRenamePreviewScroll += vp
			ui.MassRenameEnsurePreviewScroll(d, vp, len(d.MassRenamePreviewBefore))
			return false
		}
	}
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
	onMkdirRadio := a.fileDialogOnMkdirRadio()
	onMassRenameRadio := a.fileDialogOnMassRenameRadio()

	f := a.focusedField()
	if !onRadio && f != nil {
		if f.PathPicker && f.PickerFocused {
			if a.tryDialogInputRestore(event, f) {
				return false
			}
		} else if a.tryDialogInputFieldActions(event, f) {
			if a.model.FileDialog.DialogType == ui.FileDialogMassRename {
				a.recomputeMassRenamePreview()
			}
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
		if onMassRenameRadio {
			a.applyMassRenameModeFromFocus()
			return false
		}
		if a.fileDialogOnMassRenameCaseCheckbox() {
			d := &a.model.FileDialog
			d.MassRenameCaseFold = !d.MassRenameCaseFold
			a.recomputeMassRenamePreview()
			return false
		}
		if onMkdirRadio {
			a.selectFocusedMkdirRadio()
		}
		if a.fileDialogOnButton() && d.DialogType != ui.FileDialogDelete &&
			d.FocusedField == ui.FileDialogCancelFocusIndex(*d) {
			a.closeFileDialog()
			return false
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
		} else if onRadio || a.fileDialogOnMassRenameCaseCheckbox() {
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
		} else if onRadio || a.fileDialogOnMassRenameCaseCheckbox() {
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
		if onRadio || a.fileDialogOnMassRenameCaseCheckbox() {
			return false
		}
		a.fileDialogMoveCursorStart()
		return false
	case tcell.KeyEnd:
		if onRadio || a.fileDialogOnMassRenameCaseCheckbox() {
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
		if onRadio || a.fileDialogOnMassRenameCaseCheckbox() {
			return false
		}
		a.fileDialogBackspace()
		return false
	case tcell.KeyDelete:
		if onRadio || a.fileDialogOnMassRenameCaseCheckbox() {
			return false
		}
		a.fileDialogDelete()
		return false
	case tcell.KeyCtrlL:
		if onRadio || a.fileDialogOnMassRenameCaseCheckbox() {
			return false
		}
		a.fileDialogClearField()
		return false
	case tcell.KeyRune:
		if onMassRenameRadio {
			if isPlainPrintableRune(event) && event.Rune() == ' ' {
				a.applyMassRenameModeFromFocus()
			}
			return false
		}
		if a.fileDialogOnMassRenameCaseCheckbox() {
			if isPlainPrintableRune(event) && event.Rune() == ' ' {
				d := &a.model.FileDialog
				d.MassRenameCaseFold = !d.MassRenameCaseFold
				a.recomputeMassRenamePreview()
			}
			return false
		}
		if onMkdirRadio {
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
	if a.model.FileDialog.DialogType != ui.FileDialogMkdir {
		return
	}
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
	d := &a.model.FileDialog
	if d.DialogType == ui.FileDialogRename && d.RenamePhase != ui.RenamePhaseMain {
		return nil
	}
	if d.DialogType == ui.FileDialogMassRename {
		switch d.FocusedField {
		case 2:
			return &d.Fields[0]
		case 3:
			return &d.Fields[1]
		default:
			return nil
		}
	}
	if d.FocusedField < 0 || d.FocusedField >= len(d.Fields) {
		return nil
	}
	return &d.Fields[d.FocusedField]
}

func (a *App) fileDialogInsertRune(r rune) {
	field := a.focusedField()
	if field == nil {
		return
	}
	field.InsertRune(r)
	if a.model.FileDialog.DialogType == ui.FileDialogMassRename {
		a.recomputeMassRenamePreview()
	}
}

func (a *App) fileDialogBackspace() {
	field := a.focusedField()
	if field == nil {
		return
	}
	field.Backspace()
	if a.model.FileDialog.DialogType == ui.FileDialogMassRename {
		a.recomputeMassRenamePreview()
	}
}

func (a *App) fileDialogDelete() {
	field := a.focusedField()
	if field == nil {
		return
	}
	field.Delete()
	if a.model.FileDialog.DialogType == ui.FileDialogMassRename {
		a.recomputeMassRenamePreview()
	}
}

func (a *App) fileDialogClearField() {
	field := a.focusedField()
	if field == nil {
		return
	}
	field.Clear()
	if a.model.FileDialog.DialogType == ui.FileDialogMassRename {
		a.recomputeMassRenamePreview()
	}
}

func (a *App) fileDialogMoveCursor(delta int) {
	field := a.focusedField()
	if field == nil {
		return
	}
	field.MoveCursor(delta)
	if a.model.FileDialog.DialogType == ui.FileDialogMassRename {
		a.recomputeMassRenamePreview()
	}
}

func (a *App) fileDialogMoveCursorStart() {
	field := a.focusedField()
	if field == nil {
		return
	}
	field.MoveCursorStart()
	if a.model.FileDialog.DialogType == ui.FileDialogMassRename {
		a.recomputeMassRenamePreview()
	}
}

func (a *App) fileDialogMoveCursorEnd() {
	field := a.focusedField()
	if field == nil {
		return
	}
	field.MoveCursorEnd()
	if a.model.FileDialog.DialogType == ui.FileDialogMassRename {
		a.recomputeMassRenamePreview()
	}
}

// fileDialogFocusCount returns total focusable items in file dialog.
func (a *App) fileDialogFocusCount() int {
	d := a.model.FileDialog
	if d.DialogType == ui.FileDialogDelete {
		return 2 // Yes, No
	}
	if d.DialogType == ui.FileDialogRename && d.RenamePhase != ui.RenamePhaseMain {
		return 4 // 2 options + OK + Cancel
	}
	if d.DialogType == ui.FileDialogMassRename {
		return ui.FileDialogCancelFocusIndex(d) + 1
	}
	return len(d.Fields) + a.mkdirExtraFocusRows() + 2 // fields + (optional) mkdir radios + OK + Cancel
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

// fileDialogOnRadio returns true when focus is on a mkdir post-action radio or a mass-rename mode radio.
func (a *App) fileDialogOnRadio() bool {
	return a.fileDialogOnMkdirRadio() || a.fileDialogOnMassRenameRadio()
}

// fileDialogOnMkdirRadio returns true when focus is on the mkdir-with-selections radio rows.
func (a *App) fileDialogOnMkdirRadio() bool {
	d := &a.model.FileDialog
	extra := a.mkdirExtraFocusRows()
	if extra == 0 {
		return false
	}
	base := len(d.Fields)
	return d.DialogType == ui.FileDialogMkdir && d.FocusedField >= base && d.FocusedField < base+extra
}

// fileDialogOnMassRenameRadio returns true when focus is on Simple / Regex mode radios.
func (a *App) fileDialogOnMassRenameRadio() bool {
	d := &a.model.FileDialog
	if d.DialogType != ui.FileDialogMassRename {
		return false
	}
	return d.FocusedField >= 0 && d.FocusedField < 2
}

// fileDialogOnMassRenameCaseCheckbox returns true when focus is on the case-insensitive checkbox.
func (a *App) fileDialogOnMassRenameCaseCheckbox() bool {
	d := &a.model.FileDialog
	return d.DialogType == ui.FileDialogMassRename &&
		d.MassRenameMode == ui.MassRenameModeUISimple &&
		d.FocusedField == 4
}

// fileDialogRadioIndex returns the 0-based radio index when focus is on a
// mkdir radio row, or -1 otherwise.
func (a *App) fileDialogRadioIndex() int {
	if !a.fileDialogOnMkdirRadio() {
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
	if d.DialogType == ui.FileDialogRename && d.RenamePhase != ui.RenamePhaseMain {
		return d.FocusedField >= 2
	}
	return d.FocusedField >= ui.FileDialogOKFocusIndex(*d)
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
	if d.DialogType == ui.FileDialogRename && d.RenamePhase != ui.RenamePhaseMain {
		okIdx := ui.FileDialogOKFocusIndex(*d)
		cancelIdx := ui.FileDialogCancelFocusIndex(*d)
		if d.FocusedField == okIdx && delta == 1 {
			d.FocusedField = cancelIdx
		} else if d.FocusedField == cancelIdx && delta == -1 {
			d.FocusedField = okIdx
		}
		return
	}
	// Fields + (optional radios) + OK + Cancel: move between OK/Cancel only
	okIdx := ui.FileDialogOKFocusIndex(*d)
	cancelIdx := ui.FileDialogCancelFocusIndex(*d)
	if d.FocusedField == okIdx && delta == 1 {
		d.FocusedField = cancelIdx
	} else if d.FocusedField == cancelIdx && delta == -1 {
		d.FocusedField = okIdx
	}
	// Otherwise stay
}

func (a *App) handleRenameToolKey(event *tcell.EventKey) bool {
	d := &a.model.FileDialog
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'o', 'O':
			a.applyRenameToolAndReturnMain()
			return false
		case 'c', 'C':
			a.closeRenameToolPhase()
			return false
		}
		if d.RenamePhase == ui.RenamePhaseSanitize {
			switch event.Rune() {
			case '.':
				d.RenameSanitizeDots = !d.RenameSanitizeDots
				d.FocusedField = 0
				return false
			case '_':
				d.RenameSanitizeUnderscores = !d.RenameSanitizeUnderscores
				d.FocusedField = 1
				return false
			}
		} else {
			switch event.Rune() {
			case '.':
				d.RenameSlugifySep = ui.RenameSlugifyDot
				d.FocusedField = 0
				return false
			case '_':
				d.RenameSlugifySep = ui.RenameSlugifyUnderscore
				d.FocusedField = 1
				return false
			}
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeRenameToolPhase()
		return false
	case tcell.KeyEnter:
		okIdx := ui.FileDialogOKFocusIndex(*d)
		cancelIdx := ui.FileDialogCancelFocusIndex(*d)
		switch d.FocusedField {
		case okIdx:
			a.applyRenameToolAndReturnMain()
		case cancelIdx:
			a.closeRenameToolPhase()
		default:
			if d.RenamePhase == ui.RenamePhaseSanitize {
				a.toggleRenameSanitizeAtFocus()
			} else {
				a.selectRenameSlugifyAtFocus()
			}
		}
		return false
	case tcell.KeyDown, tcell.KeyTab:
		a.renameToolFocusNext()
		return false
	case tcell.KeyUp, tcell.KeyBacktab:
		a.renameToolFocusPrev()
		return false
	case tcell.KeyLeft:
		if a.fileDialogOnButton() {
			a.fileDialogFocusButton(-1)
		}
		return false
	case tcell.KeyRight:
		if a.fileDialogOnButton() {
			a.fileDialogFocusButton(1)
		}
		return false
	case tcell.KeyRune:
		if isPlainPrintableRune(event) && event.Rune() == ' ' {
			if d.RenamePhase == ui.RenamePhaseSanitize {
				a.toggleRenameSanitizeAtFocus()
			} else {
				a.selectRenameSlugifyAtFocus()
			}
		}
		return false
	}
	return false
}

func (a *App) toggleRenameSanitizeAtFocus() {
	d := &a.model.FileDialog
	switch d.FocusedField {
	case 0:
		d.RenameSanitizeDots = !d.RenameSanitizeDots
	case 1:
		d.RenameSanitizeUnderscores = !d.RenameSanitizeUnderscores
	}
}

func (a *App) selectRenameSlugifyAtFocus() {
	d := &a.model.FileDialog
	switch d.FocusedField {
	case 0:
		d.RenameSlugifySep = ui.RenameSlugifyDot
	case 1:
		d.RenameSlugifySep = ui.RenameSlugifyUnderscore
	}
}

func (a *App) renameToolFocusNext() {
	d := &a.model.FileDialog
	count := 4
	if d.FocusedField+1 >= count {
		return
	}
	d.FocusedField++
}

func (a *App) renameToolFocusPrev() {
	d := &a.model.FileDialog
	if d.FocusedField <= 0 {
		return
	}
	d.FocusedField--
}
