package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) handleFileDialogKey(event *tcell.EventKey) bool {
	d := &a.model.FileDialog
	if d.Open && ui.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase != ui.RenamePhaseMain {
		return a.handleRenameToolKey(event)
	}
	if a.tryRenameDialogShortcut(event) {
		return false
	}
	if d.Open && d.DialogType == ui.FileDialogMassRename && event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 's', 'S':
			d.MassRenameMode = ui.MassRenameModeUISimple
			if d.FocusedField == 1 {
				d.FocusedField = 0
			}
			a.massRenameSyncFieldLabels()
			a.recomputeMassRenamePreview()
			return false
		case 'r', 'R':
			d.MassRenameMode = ui.MassRenameModeUIRegex
			switch d.FocusedField {
			case 0:
				d.FocusedField = 1
			case 4:
				d.FocusedField = 3
			}
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
	if d.Open && d.DialogType == ui.FileDialogDelete {
		switch event.Key() {
		case tcell.KeyPgUp:
			_, h := a.screen.Size()
			vp := ui.DeleteDialogListViewportRows(h, *d)
			d.DeleteListScroll -= vp
			if d.DeleteListScroll < 0 {
				d.DeleteListScroll = 0
			}
			return false
		case tcell.KeyPgDn:
			_, h := a.screen.Size()
			vp := ui.DeleteDialogListViewportRows(h, *d)
			ui.DeleteEnsureListScroll(d, vp, len(d.DeleteEntries))
			d.DeleteListScroll += vp
			ui.DeleteEnsureListScroll(d, vp, len(d.DeleteEntries))
			return false
		}
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if a.tryMkdirActionAltShortcut(event.Rune()) {
			return false
		}
		if a.tryRenameFocusAltShortcut(event.Rune()) {
			return false
		}
	}
	var altExtras []dialogExtraMnemonic
	if d.DialogType == ui.FileDialogDelete {
		altExtras = []dialogExtraMnemonic{
			{'y', a.executeDelete},
			{'n', a.closeFileDialog},
		}
	}
	if a.tryStandardDialogActions(event, a.executeFileDialog, a.closeFileDialog, altExtras) {
		return false
	}

	if a.tryPathPickerHostShortcut(event) {
		return false
	}

	onRadio := a.fileDialogOnRadio()
	onMkdirRadio := a.fileDialogOnMkdirRadio()
	onRunForEachRadio := a.fileDialogOnRunForEachPoolRadio()
	onMassRenameRadio := a.fileDialogOnMassRenameRadio()
	onCheckbox := a.fileDialogOnMassRenameCaseCheckbox() || a.fileDialogOnRenameFocusCheckbox()

	f := a.focusedField()
	if !onRadio && !onCheckbox && f != nil {
		if f.PathPicker && f.PickerFocused {
			if a.tryDialogInputRestore(event, f) {
				return false
			}
		} else if a.handleFileDialogFieldKey(event, f, a.fileDialogFieldAfterEdit()) {
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
		if onRunForEachRadio {
			a.selectFocusedRunForEachPoolRadio()
			return false
		}
		if a.fileDialogOnMassRenameCaseCheckbox() {
			d := &a.model.FileDialog
			d.MassRenameCaseFold = !d.MassRenameCaseFold
			a.recomputeMassRenamePreview()
			return false
		}
		if a.fileDialogOnRenameFocusCheckbox() {
			d.RenameFocusAfter = !d.RenameFocusAfter
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
		if a.fileDialogMoveFocusKey(event) {
			return false
		}
	case tcell.KeyUp:
		if a.fileDialogMoveFocusKey(event) {
			return false
		}
	case tcell.KeyLeft:
		// On button: move between buttons; on radio: no-op; on field: move cursor
		if a.fileDialogOnButton() {
			a.fileDialogMoveFocusKey(event)
		} else if onRadio || onCheckbox {
			return false
		} else if f := a.focusedField(); f != nil && f.PathPicker && f.PickerFocused {
			f.PickerFocused = false
			runes := []rune(f.Value)
			f.Cursor = len(runes)
		} else if f := a.focusedField(); f != nil {
			a.handleFileDialogFieldKey(event, f, a.fileDialogFieldAfterEdit())
		}
		return false
	case tcell.KeyRight:
		if a.fileDialogOnButton() {
			a.fileDialogMoveFocusKey(event)
		} else if onRadio || onCheckbox {
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
				a.handleFileDialogFieldKey(event, f, a.fileDialogFieldAfterEdit())
			}
		} else if f := a.focusedField(); f != nil {
			a.handleFileDialogFieldKey(event, f, a.fileDialogFieldAfterEdit())
		}
		return false
	case tcell.KeyHome:
		if onRadio || onCheckbox {
			return false
		}
		if f := a.focusedField(); f != nil {
			a.handleFileDialogFieldKey(event, f, a.fileDialogFieldAfterEdit())
		}
		return false
	case tcell.KeyEnd:
		if onRadio || onCheckbox {
			return false
		}
		if f := a.focusedField(); f != nil {
			a.handleFileDialogFieldKey(event, f, a.fileDialogFieldAfterEdit())
		}
		return false
	case tcell.KeyTab:
		a.fileDialogMoveFocusKey(event)
		return false
	case tcell.KeyBacktab:
		a.fileDialogMoveFocusKey(event)
		return false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if onRadio || onCheckbox {
			return false
		}
		if f := a.focusedField(); f != nil {
			a.handleFileDialogFieldKey(event, f, a.fileDialogFieldAfterEdit())
		}
		return false
	case tcell.KeyDelete:
		if onRadio || onCheckbox {
			return false
		}
		if f := a.focusedField(); f != nil {
			a.handleFileDialogFieldKey(event, f, a.fileDialogFieldAfterEdit())
		}
		return false
	case tcell.KeyCtrlL:
		if onRadio || onCheckbox {
			return false
		}
		if f := a.focusedField(); f != nil {
			a.handleFileDialogFieldKey(event, f, a.fileDialogFieldAfterEdit())
		}
		return false
	case tcell.KeyRune:
		if onMassRenameRadio {
			if isPlainPrintableRune(event) && event.Rune() == ' ' {
				a.applyMassRenameModeFromFocus()
			}
			return false
		}
		if onRunForEachRadio {
			if isPlainPrintableRune(event) && event.Rune() == ' ' {
				a.selectFocusedRunForEachPoolRadio()
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
		if a.fileDialogOnRenameFocusCheckbox() {
			if isPlainPrintableRune(event) && event.Rune() == ' ' {
				d.RenameFocusAfter = !d.RenameFocusAfter
			}
			return false
		}
		if onMkdirRadio {
			if isPlainPrintableRune(event) && event.Rune() == ' ' {
				a.selectFocusedMkdirRadio()
			}
			return false
		}
		if isDialogInputRune(event) {
			if f := a.focusedField(); f != nil {
				a.handleFileDialogFieldKey(event, f, a.fileDialogFieldAfterEdit())
			}
		}
		return false
	}
	return false
}

// tryRenameFocusAltShortcut toggles focus-after-rename via Alt+A on the main rename dialog.
func (a *App) tryRenameFocusAltShortcut(r rune) bool {
	d := &a.model.FileDialog
	if !ui.FileDialogHasRenamePhase(d.DialogType) || d.RenamePhase != ui.RenamePhaseMain {
		return false
	}
	if r != 'a' && r != 'A' {
		return false
	}
	d.RenameFocusAfter = !d.RenameFocusAfter
	return true
}

// tryMkdirActionAltShortcut selects a mkdir post-action via Alt+mnemonic. Works while
// the directory-name field is focused. Alt+C selects copy (not Cancel) in this mode.
func (a *App) tryMkdirActionAltShortcut(r rune) bool {
	d := &a.model.FileDialog
	if d.DialogType != ui.FileDialogMkdir || !d.MkdirShowActions {
		return false
	}
	action, radioIdx, ok := ui.MkdirActionForAltShortcut(r)
	if !ok {
		return false
	}
	d.MkdirAction = action
	d.FocusedField = len(d.Fields) + radioIdx
	return true
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
	if ui.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase != ui.RenamePhaseMain {
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

func (a *App) clearFileDialogPickerSubfocus() {
	for i := range a.model.FileDialog.Fields {
		a.model.FileDialog.Fields[i].PickerFocused = false
	}
}

func (a *App) fileDialogMoveFocusKey(event *tcell.EventKey) bool {
	d := &a.model.FileDialog
	form := ui.FileDialogFocusForm(*d)
	if nf, ok := form.MoveFocus(d.FocusedField, event.Key()); ok {
		a.clearFileDialogPickerSubfocus()
		d.FocusedField = nf
		if d.DialogType == ui.FileDialogMassRename && (nf == 0 || nf == 1) {
			a.applyMassRenameModeFromFocus()
		}
		return true
	}
	return false
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
	return a.fileDialogOnMkdirRadio() || a.fileDialogOnMassRenameRadio() || a.fileDialogOnRunForEachPoolRadio()
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

func (a *App) runForEachExtraFocusRows() int {
	d := &a.model.FileDialog
	if d.DialogType != ui.FileDialogRunForEach {
		return 0
	}
	if len(d.RunForEachPools) == 0 {
		return 0
	}
	// "No pool" + one per configured pool.
	return 1 + len(d.RunForEachPools)
}

// fileDialogOnRunForEachPoolRadio returns true when focus is on the run-for-each pool selector rows.
func (a *App) fileDialogOnRunForEachPoolRadio() bool {
	d := &a.model.FileDialog
	extra := a.runForEachExtraFocusRows()
	if extra == 0 {
		return false
	}
	base := len(d.Fields)
	return d.DialogType == ui.FileDialogRunForEach && d.FocusedField >= base && d.FocusedField < base+extra
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

// fileDialogOnRenameFocusCheckbox returns true when focus is on the focus-after-rename checkbox.
func (a *App) fileDialogOnRenameFocusCheckbox() bool {
	d := &a.model.FileDialog
	return ui.FileDialogHasRenamePhase(d.DialogType) &&
		d.RenamePhase == ui.RenamePhaseMain &&
		len(d.Fields) > 0 &&
		d.FocusedField == len(d.Fields)
}

// fileDialogRadioIndex returns the 0-based radio index when focus is on a
// mkdir radio row, or -1 otherwise.
func (a *App) fileDialogRadioIndex() int {
	if !a.fileDialogOnMkdirRadio() {
		return -1
	}
	return a.model.FileDialog.FocusedField - len(a.model.FileDialog.Fields)
}

func (a *App) runForEachPoolRadioIndex() int {
	if !a.fileDialogOnRunForEachPoolRadio() {
		return -1
	}
	return a.model.FileDialog.FocusedField - len(a.model.FileDialog.Fields)
}

func (a *App) selectFocusedRunForEachPoolRadio() {
	d := &a.model.FileDialog
	if d.DialogType != ui.FileDialogRunForEach {
		return
	}
	idx := a.runForEachPoolRadioIndex()
	if idx < 0 {
		return
	}
	if idx == 0 {
		d.RunForEachPool = ""
		return
	}
	poolIdx := idx - 1
	if poolIdx >= 0 && poolIdx < len(d.RunForEachPools) {
		d.RunForEachPool = d.RunForEachPools[poolIdx]
	}
}

// fileDialogOnButton returns true if current focus is on a button (not a field/radio).
func (a *App) fileDialogOnButton() bool {
	d := &a.model.FileDialog
	if d.DialogType == ui.FileDialogDelete {
		return true // delete only has buttons
	}
	if ui.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase != ui.RenamePhaseMain {
		return d.FocusedField >= 2
	}
	return d.FocusedField >= ui.FileDialogOKFocusIndex(*d)
}

func (a *App) handleRenameToolKey(event *tcell.EventKey) bool {
	d := &a.model.FileDialog
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if a.tryStandardDialogActions(event, a.applyRenameToolAndReturnMain, a.closeRenameToolPhase, nil) {
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
		a.fileDialogMoveFocusKey(event)
		return false
	case tcell.KeyUp, tcell.KeyBacktab:
		a.fileDialogMoveFocusKey(event)
		return false
	case tcell.KeyLeft, tcell.KeyRight:
		if a.fileDialogOnButton() {
			a.fileDialogMoveFocusKey(event)
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
