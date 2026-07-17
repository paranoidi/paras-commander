package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// fileDialogRect returns the current on-screen file-dialog rect, used as a geometry
// signature to decide whether a per-keystroke overlay is valid. Returns the zero Rect
// when the dialog is not drawable. The Rect is a plain comparable int struct, so callers
// compare snapshots with ==; a change (e.g. mass-rename preview/hint rows resizing the
// dialog) forces a full render that correctly clears cells outside a now-smaller rect.
func (a *App) fileDialogRect() ui.Rect {
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	r, ok := dialog.FileDialogRect(layout, a.model.FileDialog, ui.DialogListIconLeadingWidth(a.model.ShowFileIcons))
	if !ok {
		return ui.Rect{}
	}
	return r
}

func (a *App) handleFileDialogKey(event *tcell.EventKey) bool {
	d := &a.model.FileDialog
	if d.Open && dialog.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase != dialog.RenamePhaseMain {
		return a.handleRenameToolKey(event)
	}
	if a.tryRenameDialogShortcut(event) {
		return false
	}
	if a.tryMkdirDialogShortcut(event) {
		return false
	}
	if d.Open && d.DialogType == dialog.FileDialogMassRename && event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if a.handleMassRenameAltShortcut(d, event.Rune()) {
			return false
		}
	}
	if d.Open && d.DialogType == dialog.FileDialogMassRename {
		if a.handleMassRenamePreviewScrollKey(d, event.Key()) {
			return false
		}
	}
	if d.Open && d.DialogType == dialog.FileDialogDelete {
		if a.handleDeleteListScrollKey(d, event.Key()) {
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
	if d.DialogType == dialog.FileDialogDelete {
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

	if f := a.focusedField(); f != nil && f.PathPicker && !f.PickerFocused &&
		event.Key() == tcell.KeyTab && f.CompletionSuffix != "" {
		if f.AcceptCompletion() {
			if after := a.fileDialogFieldAfterEdit(); after != nil {
				after()
			}
		}
		return false
	}

	onRadio := a.fileDialogOnRadio()
	onCheckbox := a.fileDialogOnMassRenameCaseCheckbox() || a.fileDialogOnRenameFocusCheckbox() || a.fileDialogOnMassRenameShowModifiedCheckbox()

	f := a.focusedField()
	skipEarlyFieldKey := f != nil && f.PathPicker && !f.PickerFocused && event.Key() == tcell.KeyRight
	if !onRadio && !onCheckbox && f != nil {
		if f.PathPicker && f.PickerFocused {
			if a.tryDialogInputRestore(event, f) {
				return false
			}
		} else if !skipEarlyFieldKey && a.handleFileDialogFieldKey(event, f, a.fileDialogFieldAfterEdit()) {
			return false
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeFileDialog()
		return false
	case tcell.KeyEnter:
		a.handleFileDialogEnter()
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
		a.handleFileDialogRune(event)
		return false
	}
	return false
}

// handleFileDialogEnter handles Enter on the file dialog: delete confirmation, path-picker
// open, mass-rename/run-for-each radio commit, checkbox toggles, mkdir radio commit, and
// button/default activation.
func (a *App) handleFileDialogEnter() {
	d := &a.model.FileDialog
	if d.DialogType == dialog.FileDialogDelete {
		if d.FocusedField == 0 {
			a.executeDelete()
		} else {
			a.closeFileDialog()
		}
		return
	}
	if f := a.focusedField(); f != nil && f.PathPicker && f.PickerFocused {
		a.openPathPickerForFileField(d.FocusedField)
		return
	}
	if a.fileDialogOnMassRenameRadio() {
		if d.FocusedField == 2 {
			a.launchMassRenameExternalEditor()
		} else {
			a.applyMassRenameModeFromFocus()
			d.FocusedField = massRenameFindFieldFocus
		}
		return
	}
	if a.fileDialogOnRunForEachPoolRadio() {
		a.selectFocusedRunForEachPoolRadio()
		return
	}
	if a.fileDialogOnMassRenameShowModifiedCheckbox() {
		d.MassRenameShowOnlyModified = !d.MassRenameShowOnlyModified
		a.recomputeMassRenamePreview()
		return
	}
	if a.fileDialogOnMassRenameCaseCheckbox() {
		d.MassRenameCaseFold = !d.MassRenameCaseFold
		a.recomputeMassRenamePreview()
		return
	}
	if a.fileDialogOnRenameFocusCheckbox() {
		d.RenameFocusAfter = !d.RenameFocusAfter
		return
	}
	if a.fileDialogOnMkdirRadio() {
		a.selectFocusedMkdirRadio()
	}
	if a.fileDialogOnButton() && d.DialogType != dialog.FileDialogDelete &&
		d.FocusedField == dialog.FileDialogCancelFocusIndex(*d) {
		a.closeFileDialog()
		return
	}
	a.executeFileDialog()
}

// handleFileDialogRune handles KeyRune on the file dialog: space-toggle for mass-rename and
// run-for-each radios, checkbox toggles, mkdir radio, and plain-input forwarding to the
// focused field.
func (a *App) handleFileDialogRune(event *tcell.EventKey) {
	d := &a.model.FileDialog
	if a.fileDialogOnMassRenameRadio() {
		if isPlainPrintableRune(event) && event.Rune() == ' ' {
			a.applyMassRenameModeFromFocus()
		}
		return
	}
	if a.fileDialogOnRunForEachPoolRadio() {
		if isPlainPrintableRune(event) && event.Rune() == ' ' {
			a.selectFocusedRunForEachPoolRadio()
		}
		return
	}
	if a.fileDialogOnMassRenameShowModifiedCheckbox() {
		if isPlainPrintableRune(event) && event.Rune() == ' ' {
			d.MassRenameShowOnlyModified = !d.MassRenameShowOnlyModified
			a.recomputeMassRenamePreview()
		}
		return
	}
	if a.fileDialogOnMassRenameCaseCheckbox() {
		if isPlainPrintableRune(event) && event.Rune() == ' ' {
			d.MassRenameCaseFold = !d.MassRenameCaseFold
			a.recomputeMassRenamePreview()
		}
		return
	}
	if a.fileDialogOnRenameFocusCheckbox() {
		if isPlainPrintableRune(event) && event.Rune() == ' ' {
			d.RenameFocusAfter = !d.RenameFocusAfter
		}
		return
	}
	if a.fileDialogOnMkdirRadio() {
		if isPlainPrintableRune(event) && event.Rune() == ' ' {
			a.selectFocusedMkdirRadio()
		}
		return
	}
	if isDialogInputRune(event) {
		if f := a.focusedField(); f != nil {
			a.handleFileDialogFieldKey(event, f, a.fileDialogFieldAfterEdit())
		}
	}
}

// handleMassRenameAltShortcut handles Alt-letter mode/option shortcuts on the mass-rename
// dialog (Simple/Regex/External mode switches, case-fold, show-only-modified). Returns
// true when the rune was handled.
func (a *App) handleMassRenameAltShortcut(d *dialog.FileDialogState, r rune) bool {
	switch r {
	case 's', 'S':
		d.MassRenameMode = dialog.MassRenameModeUISimple
		if d.FocusedField == 1 || d.FocusedField == 2 {
			d.FocusedField = 0
		}
		a.massRenameSyncFieldLabels()
		a.recomputeMassRenamePreview()
		return true
	case 'r', 'R':
		d.MassRenameMode = dialog.MassRenameModeUIRegex
		switch d.FocusedField {
		case 0, 2:
			d.FocusedField = 1
		case 6:
			d.FocusedField = 5 // Simple's show-modified (6) → Regex's show-modified (5)
		}
		a.massRenameSyncFieldLabels()
		a.recomputeMassRenamePreview()
		return true
	case 'e', 'E':
		d.MassRenameMode = dialog.MassRenameModeUIExternalEditor
		if d.FocusedField < 2 {
			d.FocusedField = 2
		}
		a.massRenameSyncFieldLabels()
		a.recomputeMassRenamePreview()
		return true
	case 'i', 'I':
		if d.MassRenameMode == dialog.MassRenameModeUISimple {
			d.MassRenameCaseFold = !d.MassRenameCaseFold
			a.recomputeMassRenamePreview()
		}
		return true
	case 'm', 'M':
		d.MassRenameShowOnlyModified = !d.MassRenameShowOnlyModified
		a.recomputeMassRenamePreview()
		return true
	}
	return false
}

// handleMassRenamePreviewScrollKey handles F4 (external editor) and PgUp/PgDn preview
// scrolling on the mass-rename dialog. Returns true when the key was handled.
func (a *App) handleMassRenamePreviewScrollKey(d *dialog.FileDialogState, key tcell.Key) bool {
	switch key {
	case tcell.KeyF4:
		a.launchMassRenameExternalEditor()
		return true
	case tcell.KeyPgUp:
		_, h := a.screen.Size()
		vp := dialog.MassRenamePreviewViewportRows(h, d.MassRenameMode)
		d.MassRenamePreviewScroll -= vp
		if d.MassRenamePreviewScroll < 0 {
			d.MassRenamePreviewScroll = 0
		}
		return true
	case tcell.KeyPgDn:
		_, h := a.screen.Size()
		vp := dialog.MassRenamePreviewViewportRows(h, d.MassRenameMode)
		dialog.MassRenameEnsurePreviewScroll(d, vp, len(d.MassRenamePreviewBefore))
		d.MassRenamePreviewScroll += vp
		dialog.MassRenameEnsurePreviewScroll(d, vp, len(d.MassRenamePreviewBefore))
		return true
	}
	return false
}

// handleDeleteListScrollKey handles PgUp/PgDn scrolling of the delete confirmation list.
// Returns true when the key was handled.
func (a *App) handleDeleteListScrollKey(d *dialog.FileDialogState, key tcell.Key) bool {
	switch key {
	case tcell.KeyPgUp:
		_, h := a.screen.Size()
		vp := dialog.DeleteDialogListViewportRows(h, *d)
		d.DeleteListScroll -= vp
		if d.DeleteListScroll < 0 {
			d.DeleteListScroll = 0
		}
		return true
	case tcell.KeyPgDn:
		_, h := a.screen.Size()
		vp := dialog.DeleteDialogListViewportRows(h, *d)
		dialog.DeleteEnsureListScroll(d, vp, len(d.DeleteEntries))
		d.DeleteListScroll += vp
		dialog.DeleteEnsureListScroll(d, vp, len(d.DeleteEntries))
		return true
	}
	return false
}

// tryRenameFocusAltShortcut toggles focus-after-rename via Alt+A on the main rename dialog.
func (a *App) tryRenameFocusAltShortcut(r rune) bool {
	d := &a.model.FileDialog
	if !dialog.FileDialogHasRenamePhase(d.DialogType) || d.RenamePhase != dialog.RenamePhaseMain {
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
	if d.DialogType != dialog.FileDialogMkdir || !d.MkdirShowActions {
		return false
	}
	action, radioIdx, ok := dialog.MkdirActionForAltShortcut(r)
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
	if a.model.FileDialog.DialogType != dialog.FileDialogMkdir {
		return
	}
	idx := a.fileDialogRadioIndex()
	if idx < 0 {
		return
	}
	switch idx {
	case 0:
		a.model.FileDialog.MkdirAction = dialog.MkdirActionCreate
	case 1:
		a.model.FileDialog.MkdirAction = dialog.MkdirActionCreateCopySelect
	case 2:
		a.model.FileDialog.MkdirAction = dialog.MkdirActionCreateMoveSelect
	}
}

func (a *App) focusedField() *dialog.FileDialogField {
	d := &a.model.FileDialog
	if dialog.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase != dialog.RenamePhaseMain {
		return nil
	}
	if d.DialogType == dialog.FileDialogMassRename {
		switch d.FocusedField {
		case 3:
			return &d.Fields[0]
		case 4:
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

// massRenameMoveFocusKey handles Tab/Backtab segment jumps and Down/Up visual-order
// transitions specific to the mass-rename dialog's non-linear focus layout (focus indices
// don't match visual order: Seg 0 = mode radios(0-2) + show-modified, Seg 1 = find+replace+
// case-fold (non-external), Seg 2 = buttons). Returns true when handled (focus updated);
// false when the dialog isn't mass-rename or the key isn't one of these special cases, so
// the caller falls through to the generic dialog.FileDialogFocusForm tail.
func (a *App) massRenameMoveFocusKey(event *tcell.EventKey) bool {
	d := &a.model.FileDialog
	if d.DialogType != dialog.FileDialogMassRename {
		return false
	}
	key := event.Key()
	externalMode := d.MassRenameMode == dialog.MassRenameModeUIExternalEditor
	showModifiedIdx := dialog.MassRenameShowModifiedFocusIdx(*d)
	okIdx := dialog.FileDialogOKFocusIndex(*d)
	onRadio := d.FocusedField >= 0 && d.FocusedField < 3
	onShowModified := d.FocusedField == showModifiedIdx
	onFindOrReplace := !externalMode && (d.FocusedField == massRenameFindFieldFocus || d.FocusedField == massRenameFindFieldFocus+1)
	onCaseFold := d.MassRenameMode == dialog.MassRenameModeUISimple && d.FocusedField == 5
	onSegment1 := onFindOrReplace || onCaseFold
	onButton := d.FocusedField >= okIdx
	if key == tcell.KeyTab || key == tcell.KeyBacktab {
		a.clearFileDialogPickerSubfocus()
		if key == tcell.KeyTab {
			switch {
			case onRadio || onShowModified:
				if externalMode {
					d.FocusedField = okIdx
				} else {
					d.FocusedField = massRenameFindFieldFocus
				}
			case onSegment1:
				d.FocusedField = okIdx
			case onButton:
				d.FocusedField = 0
				a.applyMassRenameModeFromFocus()
			}
		} else { // Backtab
			switch {
			case onRadio || onShowModified:
				d.FocusedField = okIdx
			case onSegment1:
				d.FocusedField = 0
				a.applyMassRenameModeFromFocus()
			case onButton:
				if externalMode {
					d.FocusedField = 0
					a.applyMassRenameModeFromFocus()
				} else {
					d.FocusedField = massRenameFindFieldFocus
				}
			}
		}
		return true
	}
	// Down/Up use visual order (show-modified is above the fields but has a higher focus index).
	notExternal := !externalMode
	if key == tcell.KeyDown && d.FocusedField == 2 && notExternal {
		a.clearFileDialogPickerSubfocus()
		d.FocusedField = showModifiedIdx
		return true
	}
	if key == tcell.KeyDown && onShowModified && notExternal {
		a.clearFileDialogPickerSubfocus()
		d.FocusedField = massRenameFindFieldFocus
		return true
	}
	if key == tcell.KeyUp && d.FocusedField == massRenameFindFieldFocus && notExternal {
		a.clearFileDialogPickerSubfocus()
		d.FocusedField = showModifiedIdx
		return true
	}
	if key == tcell.KeyUp && onShowModified && notExternal {
		a.clearFileDialogPickerSubfocus()
		d.FocusedField = 2
		a.applyMassRenameModeFromFocus()
		return true
	}
	return false
}

func (a *App) fileDialogMoveFocusKey(event *tcell.EventKey) bool {
	d := &a.model.FileDialog

	if a.massRenameMoveFocusKey(event) {
		return true
	}

	form := dialog.FileDialogFocusForm(*d)
	if nf, ok := form.MoveFocus(d.FocusedField, event.Key()); ok {
		a.clearFileDialogPickerSubfocus()
		d.FocusedField = nf
		if d.DialogType == dialog.FileDialogMassRename && (nf == 0 || nf == 1 || nf == 2) {
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
	if d.DialogType == dialog.FileDialogMkdir && d.MkdirShowActions {
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
	return d.DialogType == dialog.FileDialogMkdir && d.FocusedField >= base && d.FocusedField < base+extra
}

func (a *App) runForEachExtraFocusRows() int {
	d := &a.model.FileDialog
	if d.DialogType != dialog.FileDialogRunForEach {
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
	return d.DialogType == dialog.FileDialogRunForEach && d.FocusedField >= base && d.FocusedField < base+extra
}

// fileDialogOnMassRenameRadio returns true when focus is on one of the three mode radios.
func (a *App) fileDialogOnMassRenameRadio() bool {
	d := &a.model.FileDialog
	if d.DialogType != dialog.FileDialogMassRename {
		return false
	}
	return d.FocusedField >= 0 && d.FocusedField < 3
}

// fileDialogOnMassRenameCaseCheckbox returns true when focus is on the case-insensitive checkbox.
func (a *App) fileDialogOnMassRenameCaseCheckbox() bool {
	d := &a.model.FileDialog
	return d.DialogType == dialog.FileDialogMassRename &&
		d.MassRenameMode == dialog.MassRenameModeUISimple &&
		d.FocusedField == 5
}

// fileDialogOnMassRenameShowModifiedCheckbox returns true when focus is on the "Show only modified" checkbox.
func (a *App) fileDialogOnMassRenameShowModifiedCheckbox() bool {
	d := &a.model.FileDialog
	if d.DialogType != dialog.FileDialogMassRename {
		return false
	}
	return d.FocusedField == dialog.MassRenameShowModifiedFocusIdx(*d)
}

// fileDialogOnRenameFocusCheckbox returns true when focus is on the focus-after-rename checkbox.
func (a *App) fileDialogOnRenameFocusCheckbox() bool {
	d := &a.model.FileDialog
	return dialog.FileDialogHasRenamePhase(d.DialogType) &&
		d.RenamePhase == dialog.RenamePhaseMain &&
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
	if d.DialogType != dialog.FileDialogRunForEach {
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
	if d.DialogType == dialog.FileDialogDelete {
		return true // delete only has buttons
	}
	if dialog.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase != dialog.RenamePhaseMain {
		return d.FocusedField >= dialog.FileDialogOKFocusIndex(*d)
	}
	return d.FocusedField >= dialog.FileDialogOKFocusIndex(*d)
}

func (a *App) handleRenameToolKey(event *tcell.EventKey) bool {
	d := &a.model.FileDialog
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if a.tryStandardDialogActions(event, a.applyRenameToolAndReturnMain, a.closeRenameToolPhase, nil) {
			return false
		}
		switch d.RenamePhase {
		case dialog.RenamePhaseSanitize:
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
		case dialog.RenamePhaseSlugify:
			switch event.Rune() {
			case '.':
				d.RenameSlugifySep = dialog.RenameSlugifyDot
				d.FocusedField = 0
				return false
			case '_':
				d.RenameSlugifySep = dialog.RenameSlugifyUnderscore
				d.FocusedField = 1
				return false
			}
		case dialog.RenamePhaseEncoding:
			for i := 0; i < len(d.RenameEncodingCandidates); i++ {
				if event.Rune() == dialog.RenameEncodingCandidateShortcut(d.RenameEncodingCandidates[i].Label) {
					d.RenameEncodingSelected = i
					d.FocusedField = i
					return false
				}
			}
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeRenameToolPhase()
		return false
	case tcell.KeyEnter:
		okIdx := dialog.FileDialogOKFocusIndex(*d)
		cancelIdx := dialog.FileDialogCancelFocusIndex(*d)
		switch d.FocusedField {
		case okIdx:
			a.applyRenameToolAndReturnMain()
		case cancelIdx:
			a.closeRenameToolPhase()
		default:
			switch d.RenamePhase {
			case dialog.RenamePhaseSanitize:
				a.toggleRenameSanitizeAtFocus()
			case dialog.RenamePhaseSlugify:
				a.selectRenameSlugifyAtFocus()
			case dialog.RenamePhaseEncoding:
				a.selectRenameEncodingAtFocus()
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
			switch d.RenamePhase {
			case dialog.RenamePhaseSanitize:
				a.toggleRenameSanitizeAtFocus()
			case dialog.RenamePhaseSlugify:
				a.selectRenameSlugifyAtFocus()
			case dialog.RenamePhaseEncoding:
				a.selectRenameEncodingAtFocus()
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
		d.RenameSlugifySep = dialog.RenameSlugifyDot
	case 1:
		d.RenameSlugifySep = dialog.RenameSlugifyUnderscore
	}
}
