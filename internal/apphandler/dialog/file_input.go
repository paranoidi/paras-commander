package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// FileDialogRect returns the current on-screen file-dialog rect, used as a geometry
// signature to decide whether a per-keystroke overlay is valid. Returns the zero Rect
// when the dialog is not drawable. The Rect is a plain comparable int struct, so callers
// compare snapshots with ==; a change (e.g. mass-rename preview/hint rows resizing the
// dialog) forces a full render that correctly clears cells outside a now-smaller rect.
func (h *Handler) FileDialogRect() ui.Rect {
	w, ht := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(w, ht)
	r, ok := dialog.FileDialogRect(layout, h.model.FileDialog, ui.DialogListIconLeadingWidth(h.model.ShowFileIcons))
	if !ok {
		return ui.Rect{}
	}
	return r
}

// tryFileDialogPreKey handles the mass-rename/delete Alt-shortcuts and preview/list
// scrolling, plus the mkdir/rename Alt-mnemonics, that must run before general field
// routing in HandleFileDialogKey. Returns true when the key was fully handled.
func (h *Handler) tryFileDialogPreKey(event *tcell.EventKey, d *dialog.FileDialogState) bool {
	if d.Open && d.DialogType == dialog.FileDialogMassRename && event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if h.handleMassRenameAltShortcut(d, event.Rune()) {
			return true
		}
	}
	if d.Open && d.DialogType == dialog.FileDialogMassRename {
		if h.handleMassRenamePreviewScrollKey(d, event.Key()) {
			return true
		}
	}
	if d.Open && d.DialogType == dialog.FileDialogDelete {
		if h.handleDeleteListScrollKey(d, event.Key()) {
			return true
		}
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if h.tryMkdirActionAltShortcut(event.Rune()) {
			return true
		}
		if h.tryRenameFocusAltShortcut(event.Rune()) {
			return true
		}
	}
	return false
}

// HandleFileDialogKey routes a key event to the open file dialog. Returns true when the event
// requests quitting the app (never for the file dialog itself; matches the App.handleKey
// (bool quit) convention shared across auxiliary key handlers).
func (h *Handler) HandleFileDialogKey(event *tcell.EventKey) bool {
	d := &h.model.FileDialog
	if d.Open && dialog.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase != dialog.RenamePhaseMain {
		return h.handleRenameToolKey(event)
	}
	if h.tryRenameDialogShortcut(event) {
		return false
	}
	if h.tryMkdirDialogShortcut(event) {
		return false
	}
	if h.tryFileDialogPreKey(event, d) {
		return false
	}
	var altExtras []dialog.ExtraMnemonic
	if d.DialogType == dialog.FileDialogDelete {
		altExtras = []dialog.ExtraMnemonic{
			{Rune: 'y', Fn: h.ExecuteDelete},
			{Rune: 'n', Fn: h.CloseFileDialog},
		}
	}
	if dialog.TryStandardDialogActions(event, h.ExecuteFileDialog, h.CloseFileDialog, altExtras) {
		return false
	}

	if h.TryPathPickerHostShortcut(event) {
		return false
	}

	if f := h.FocusedField(); f != nil && f.PathPicker && !f.PickerFocused &&
		event.Key() == tcell.KeyTab && f.CompletionSuffix != "" {
		if f.AcceptCompletion() {
			if after := h.fileDialogFieldAfterEdit(); after != nil {
				after()
			}
		}
		return false
	}

	onRadio := h.fileDialogOnRadio()
	onCheckbox := h.fileDialogOnMassRenameCaseCheckbox() ||
		h.fileDialogOnMassRenameStripCheckbox() ||
		h.fileDialogOnRenameFocusCheckbox() ||
		h.fileDialogOnMassRenameShowModifiedCheckbox()

	f := h.FocusedField()
	skipEarlyFieldKey := f != nil && f.PathPicker && !f.PickerFocused && event.Key() == tcell.KeyRight
	if !onRadio && !onCheckbox && f != nil {
		if f.PathPicker && f.PickerFocused {
			if dialog.TryDialogInputRestore(event, f, h.keysDialogInput) {
				return false
			}
		} else if !skipEarlyFieldKey && dialog.HandleFileDialogFieldKey(event, f, h.keysDialogInput, h.fileDialogFieldAfterEdit()) {
			return false
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.CloseFileDialog()
		return false
	case tcell.KeyEnter:
		h.handleFileDialogEnter()
		return false
	case tcell.KeyDown:
		if h.fileDialogMoveFocusKey(event) {
			return false
		}
	case tcell.KeyUp:
		if h.fileDialogMoveFocusKey(event) {
			return false
		}
	case tcell.KeyLeft:
		// On button: move between buttons; on radio: no-op; on field: move cursor
		if h.FileDialogOnButton() {
			h.fileDialogMoveFocusKey(event)
		} else if onRadio || onCheckbox {
			return false
		} else if f := h.FocusedField(); f != nil && f.PathPicker && f.PickerFocused {
			f.PickerFocused = false
			runes := []rune(f.Value)
			f.Cursor = len(runes)
		} else if f := h.FocusedField(); f != nil {
			dialog.HandleFileDialogFieldKey(event, f, h.keysDialogInput, h.fileDialogFieldAfterEdit())
		}
		return false
	case tcell.KeyRight:
		if h.FileDialogOnButton() {
			h.fileDialogMoveFocusKey(event)
		} else if onRadio || onCheckbox {
			return false
		} else if f := h.FocusedField(); f != nil && f.PathPicker && !f.PickerFocused {
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
				dialog.HandleFileDialogFieldKey(event, f, h.keysDialogInput, h.fileDialogFieldAfterEdit())
			}
		} else if f := h.FocusedField(); f != nil {
			dialog.HandleFileDialogFieldKey(event, f, h.keysDialogInput, h.fileDialogFieldAfterEdit())
		}
		return false
	case tcell.KeyHome, tcell.KeyEnd, tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete, tcell.KeyCtrlL:
		return h.fileDialogPassFieldEditKey(event, onRadio, onCheckbox)
	case tcell.KeyTab:
		h.fileDialogMoveFocusKey(event)
		return false
	case tcell.KeyBacktab:
		h.fileDialogMoveFocusKey(event)
		return false
	case tcell.KeyRune:
		h.handleFileDialogRune(event)
		return false
	}
	return false
}

// fileDialogPassFieldEditKey routes Home/End/Backspace/Delete/Ctrl+L to the focused
// field's edit handler, unless focus is on a radio or checkbox row (no-op there).
// Always returns false: these keys are always fully consumed by the file dialog.
func (h *Handler) fileDialogPassFieldEditKey(event *tcell.EventKey, onRadio, onCheckbox bool) bool {
	if onRadio || onCheckbox {
		return false
	}
	if f := h.FocusedField(); f != nil {
		dialog.HandleFileDialogFieldKey(event, f, h.keysDialogInput, h.fileDialogFieldAfterEdit())
	}
	return false
}

// handleFileDialogEnter handles Enter on the file dialog: delete confirmation, path-picker
// open, mass-rename/run-for-each radio commit, checkbox toggles, mkdir radio commit, and
// button/default activation.
func (h *Handler) handleFileDialogEnter() {
	d := &h.model.FileDialog
	if d.DialogType == dialog.FileDialogDelete {
		if d.FocusedField == 0 {
			h.ExecuteDelete()
		} else {
			h.CloseFileDialog()
		}
		return
	}
	if f := h.FocusedField(); f != nil && f.PathPicker && f.PickerFocused {
		h.OpenPathPickerForFileField(d.FocusedField)
		return
	}
	if h.fileDialogOnMassRenameRadio() {
		if d.FocusedField == 2 {
			h.LaunchMassRenameExternalEditor()
		} else {
			h.ApplyMassRenameModeFromFocus()
			d.FocusedField = dialog.MassRenameFindFieldFocus
		}
		return
	}
	if h.fileDialogOnRunForEachPoolRadio() {
		h.selectFocusedRunForEachPoolRadio()
		return
	}
	if h.fileDialogOnMassRenameShowModifiedCheckbox() {
		d.MassRenameShowOnlyModified = !d.MassRenameShowOnlyModified
		h.RecomputeMassRenamePreview()
		return
	}
	if h.fileDialogOnMassRenameStripCheckbox() {
		d.MassRenameStripSpaces = !d.MassRenameStripSpaces
		h.RecomputeMassRenamePreview()
		return
	}
	if h.fileDialogOnMassRenameCaseCheckbox() {
		d.MassRenameCaseFold = !d.MassRenameCaseFold
		h.RecomputeMassRenamePreview()
		return
	}
	if h.fileDialogOnRenameFocusCheckbox() {
		d.RenameFocusAfter = !d.RenameFocusAfter
		return
	}
	if h.fileDialogOnMkdirRadio() {
		h.selectFocusedMkdirRadio()
	}
	if h.FileDialogOnButton() && d.DialogType != dialog.FileDialogDelete &&
		d.FocusedField == dialog.FileDialogCancelFocusIndex(*d) {
		h.CloseFileDialog()
		return
	}
	h.ExecuteFileDialog()
}

// handleFileDialogRune handles KeyRune on the file dialog: space-toggle for mass-rename and
// run-for-each radios, checkbox toggles, mkdir radio, and plain-input forwarding to the
// focused field.
func (h *Handler) handleFileDialogRune(event *tcell.EventKey) {
	d := &h.model.FileDialog
	if h.fileDialogOnMassRenameRadio() {
		if keymap.IsPlainPrintableRune(event) && event.Rune() == ' ' {
			h.ApplyMassRenameModeFromFocus()
		}
		return
	}
	if h.fileDialogOnRunForEachPoolRadio() {
		if keymap.IsPlainPrintableRune(event) && event.Rune() == ' ' {
			h.selectFocusedRunForEachPoolRadio()
		}
		return
	}
	if h.fileDialogOnMassRenameShowModifiedCheckbox() {
		if keymap.IsPlainPrintableRune(event) && event.Rune() == ' ' {
			d.MassRenameShowOnlyModified = !d.MassRenameShowOnlyModified
			h.RecomputeMassRenamePreview()
		}
		return
	}
	if h.fileDialogOnMassRenameStripCheckbox() {
		if keymap.IsPlainPrintableRune(event) && event.Rune() == ' ' {
			d.MassRenameStripSpaces = !d.MassRenameStripSpaces
			h.RecomputeMassRenamePreview()
		}
		return
	}
	if h.fileDialogOnMassRenameCaseCheckbox() {
		if keymap.IsPlainPrintableRune(event) && event.Rune() == ' ' {
			d.MassRenameCaseFold = !d.MassRenameCaseFold
			h.RecomputeMassRenamePreview()
		}
		return
	}
	if h.fileDialogOnRenameFocusCheckbox() {
		if keymap.IsPlainPrintableRune(event) && event.Rune() == ' ' {
			d.RenameFocusAfter = !d.RenameFocusAfter
		}
		return
	}
	if h.fileDialogOnMkdirRadio() {
		if keymap.IsPlainPrintableRune(event) && event.Rune() == ' ' {
			h.selectFocusedMkdirRadio()
		}
		return
	}
	if scrollquery.IsDialogInputRune(event) {
		if f := h.FocusedField(); f != nil {
			dialog.HandleFileDialogFieldKey(event, f, h.keysDialogInput, h.fileDialogFieldAfterEdit())
		}
	}
}

// handleMassRenameAltShortcut handles Alt-letter mode/option shortcuts on the mass-rename
// dialog (Simple/Regex/External mode switches, case-fold, strip, show-only-modified).
// Returns true when the rune was handled.
func (h *Handler) handleMassRenameAltShortcut(d *dialog.FileDialogState, r rune) bool {
	switch r {
	case 's', 'S':
		prev := d.MassRenameMode
		d.MassRenameMode = dialog.MassRenameModeUISimple
		if d.FocusedField == 1 || d.FocusedField == 2 {
			d.FocusedField = 0
		}
		h.MassRenameClampFocusAfterModeChange(prev)
		h.MassRenameSyncFieldLabels()
		h.RecomputeMassRenamePreview()
		return true
	case 'r', 'R':
		prev := d.MassRenameMode
		d.MassRenameMode = dialog.MassRenameModeUIRegex
		if d.FocusedField == 0 || d.FocusedField == 2 {
			d.FocusedField = 1
		}
		h.MassRenameClampFocusAfterModeChange(prev)
		h.MassRenameSyncFieldLabels()
		h.RecomputeMassRenamePreview()
		return true
	case 'e', 'E':
		prev := d.MassRenameMode
		d.MassRenameMode = dialog.MassRenameModeUIExternalEditor
		if d.FocusedField < 2 {
			d.FocusedField = 2
		}
		h.MassRenameClampFocusAfterModeChange(prev)
		h.MassRenameSyncFieldLabels()
		h.RecomputeMassRenamePreview()
		return true
	case 'i', 'I':
		if d.MassRenameMode != dialog.MassRenameModeUIExternalEditor {
			d.MassRenameCaseFold = !d.MassRenameCaseFold
			h.RecomputeMassRenamePreview()
		}
		return true
	case 't', 'T':
		d.MassRenameStripSpaces = !d.MassRenameStripSpaces
		h.RecomputeMassRenamePreview()
		return true
	case 'm', 'M':
		d.MassRenameShowOnlyModified = !d.MassRenameShowOnlyModified
		h.RecomputeMassRenamePreview()
		return true
	}
	return false
}

// handleMassRenamePreviewScrollKey handles F4 (external editor) and PgUp/PgDn preview
// scrolling on the mass-rename dialog. Returns true when the key was handled.
func (h *Handler) handleMassRenamePreviewScrollKey(d *dialog.FileDialogState, key tcell.Key) bool {
	switch key {
	case tcell.KeyF4:
		h.LaunchMassRenameExternalEditor()
		return true
	case tcell.KeyPgUp:
		_, ht := h.screen.Size()
		vp := dialog.MassRenamePreviewViewportRows(ht, d.MassRenameMode)
		d.MassRenamePreviewScroll -= vp
		if d.MassRenamePreviewScroll < 0 {
			d.MassRenamePreviewScroll = 0
		}
		return true
	case tcell.KeyPgDn:
		_, ht := h.screen.Size()
		vp := dialog.MassRenamePreviewViewportRows(ht, d.MassRenameMode)
		dialog.MassRenameEnsurePreviewScroll(d, vp, len(d.MassRenamePreviewBefore))
		d.MassRenamePreviewScroll += vp
		dialog.MassRenameEnsurePreviewScroll(d, vp, len(d.MassRenamePreviewBefore))
		return true
	}
	return false
}

// handleDeleteListScrollKey handles PgUp/PgDn scrolling of the delete confirmation list.
// Returns true when the key was handled.
func (h *Handler) handleDeleteListScrollKey(d *dialog.FileDialogState, key tcell.Key) bool {
	switch key {
	case tcell.KeyPgUp:
		_, ht := h.screen.Size()
		vp := dialog.DeleteDialogListViewportRows(ht, *d)
		d.DeleteListScroll -= vp
		if d.DeleteListScroll < 0 {
			d.DeleteListScroll = 0
		}
		return true
	case tcell.KeyPgDn:
		_, ht := h.screen.Size()
		vp := dialog.DeleteDialogListViewportRows(ht, *d)
		dialog.DeleteEnsureListScroll(d, vp, len(d.DeleteEntries))
		d.DeleteListScroll += vp
		dialog.DeleteEnsureListScroll(d, vp, len(d.DeleteEntries))
		return true
	}
	return false
}

// tryRenameFocusAltShortcut toggles focus-after-rename via Alt+A on the main rename dialog.
func (h *Handler) tryRenameFocusAltShortcut(r rune) bool {
	d := &h.model.FileDialog
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
func (h *Handler) tryMkdirActionAltShortcut(r rune) bool {
	d := &h.model.FileDialog
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
func (h *Handler) selectFocusedMkdirRadio() {
	if h.model.FileDialog.DialogType != dialog.FileDialogMkdir {
		return
	}
	idx := h.fileDialogRadioIndex()
	if idx < 0 {
		return
	}
	switch idx {
	case 0:
		h.model.FileDialog.MkdirAction = dialog.MkdirActionCreate
	case 1:
		h.model.FileDialog.MkdirAction = dialog.MkdirActionCreateCopySelect
	case 2:
		h.model.FileDialog.MkdirAction = dialog.MkdirActionCreateMoveSelect
	}
}

// FocusedField returns the FileDialogField the file dialog currently has focus on, or nil when
// focus is on a button/radio/checkbox row or no dialog is open.
func (h *Handler) FocusedField() *dialog.FileDialogField {
	d := &h.model.FileDialog
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

func (h *Handler) clearFileDialogPickerSubfocus() {
	for i := range h.model.FileDialog.Fields {
		h.model.FileDialog.Fields[i].PickerFocused = false
	}
}

// massRenameMoveFocusKey handles Tab/Backtab segment jumps, Left/Right on the options
// checkbox row, and Down/Up visual-order transitions (focus indices don't match visual
// order: Seg 0 = mode radios(0-2) + options row, Seg 1 = find+replace, Seg 2 = buttons).
// Returns true when handled; false so the caller can fall through to FileDialogFocusForm.
func (h *Handler) massRenameMoveFocusKey(event *tcell.EventKey) bool {
	d := &h.model.FileDialog
	if d.DialogType != dialog.FileDialogMassRename {
		return false
	}
	key := event.Key()
	externalMode := d.MassRenameMode == dialog.MassRenameModeUIExternalEditor
	showModifiedIdx := dialog.MassRenameShowModifiedFocusIdx(*d)
	stripIdx := dialog.MassRenameStripFocusIdx(*d)
	caseIdx := dialog.MassRenameCaseFocusIdx(*d)
	okIdx := dialog.FileDialogOKFocusIndex(*d)
	onRadio := d.FocusedField >= 0 && d.FocusedField < 3
	onOptionsRow := d.FocusedField == showModifiedIdx || d.FocusedField == stripIdx || d.FocusedField == caseIdx
	onFindOrReplace := !externalMode && (d.FocusedField == dialog.MassRenameFindFieldFocus || d.FocusedField == dialog.MassRenameFindFieldFocus+1)
	onButton := d.FocusedField >= okIdx

	if key == tcell.KeyRight {
		switch d.FocusedField {
		case showModifiedIdx:
			h.clearFileDialogPickerSubfocus()
			d.FocusedField = stripIdx
			return true
		case stripIdx:
			if caseIdx >= 0 {
				h.clearFileDialogPickerSubfocus()
				d.FocusedField = caseIdx
				return true
			}
		}
	}
	if key == tcell.KeyLeft {
		switch d.FocusedField {
		case stripIdx:
			h.clearFileDialogPickerSubfocus()
			d.FocusedField = showModifiedIdx
			return true
		case caseIdx:
			h.clearFileDialogPickerSubfocus()
			d.FocusedField = stripIdx
			return true
		}
	}

	if key == tcell.KeyTab || key == tcell.KeyBacktab {
		h.clearFileDialogPickerSubfocus()
		if key == tcell.KeyTab {
			switch {
			case onRadio || onOptionsRow:
				if externalMode {
					d.FocusedField = okIdx
				} else {
					d.FocusedField = dialog.MassRenameFindFieldFocus
				}
			case onFindOrReplace:
				d.FocusedField = okIdx
			case onButton:
				// Land on the radio for the current mode (do not force Simple).
				d.FocusedField = dialog.MassRenameModeRadioFocus(d.MassRenameMode)
			}
		} else { // Backtab
			switch {
			case onRadio || onOptionsRow:
				d.FocusedField = okIdx
			case onFindOrReplace:
				d.FocusedField = dialog.MassRenameModeRadioFocus(d.MassRenameMode)
			case onButton:
				if externalMode {
					d.FocusedField = dialog.MassRenameModeRadioFocus(d.MassRenameMode)
				} else {
					d.FocusedField = dialog.MassRenameFindFieldFocus
				}
			}
		}
		return true
	}
	// Down/Up use visual order (options row is above the fields but has higher focus indices).
	if key == tcell.KeyDown && d.FocusedField == 2 {
		h.clearFileDialogPickerSubfocus()
		d.FocusedField = showModifiedIdx
		return true
	}
	if key == tcell.KeyDown && onOptionsRow {
		h.clearFileDialogPickerSubfocus()
		if externalMode {
			d.FocusedField = okIdx
		} else {
			d.FocusedField = dialog.MassRenameFindFieldFocus
		}
		return true
	}
	if key == tcell.KeyUp && d.FocusedField == dialog.MassRenameFindFieldFocus && !externalMode {
		h.clearFileDialogPickerSubfocus()
		d.FocusedField = showModifiedIdx
		return true
	}
	if key == tcell.KeyDown && d.FocusedField == dialog.MassRenameFindFieldFocus+1 && !externalMode {
		// Replace → OK (skip options-row indices that sit above the fields visually).
		h.clearFileDialogPickerSubfocus()
		d.FocusedField = okIdx
		return true
	}
	if key == tcell.KeyUp && onOptionsRow {
		h.clearFileDialogPickerSubfocus()
		d.FocusedField = 2
		h.ApplyMassRenameModeFromFocus()
		return true
	}
	if key == tcell.KeyUp && onButton {
		h.clearFileDialogPickerSubfocus()
		if externalMode {
			d.FocusedField = showModifiedIdx
		} else {
			d.FocusedField = dialog.MassRenameFindFieldFocus + 1 // Replace
		}
		return true
	}
	return false
}

func (h *Handler) fileDialogMoveFocusKey(event *tcell.EventKey) bool {
	d := &h.model.FileDialog

	if h.massRenameMoveFocusKey(event) {
		return true
	}

	form := dialog.FileDialogFocusForm(*d)
	if nf, ok := form.MoveFocus(d.FocusedField, event.Key()); ok {
		h.clearFileDialogPickerSubfocus()
		d.FocusedField = nf
		if d.DialogType == dialog.FileDialogMassRename && (nf == 0 || nf == 1 || nf == 2) {
			h.ApplyMassRenameModeFromFocus()
		}
		return true
	}
	return false
}

// mkdirExtraFocusRows returns the number of extra focus rows contributed by the
// mkdir-with-selections radio section, or 0 when not applicable.
func (h *Handler) mkdirExtraFocusRows() int {
	d := &h.model.FileDialog
	if d.DialogType == dialog.FileDialogMkdir && d.MkdirShowActions {
		return 3
	}
	return 0
}

// fileDialogOnRadio returns true when focus is on a mkdir post-action radio, a mass-rename mode
// radio, or a run-for-each pool radio.
func (h *Handler) fileDialogOnRadio() bool {
	return h.fileDialogOnMkdirRadio() || h.fileDialogOnMassRenameRadio() || h.fileDialogOnRunForEachPoolRadio()
}

// FileDialogOnRadio reports whether focus is currently on any file-dialog radio row (mkdir
// post-action, mass-rename mode, or run-for-each pool).
func (h *Handler) FileDialogOnRadio() bool {
	return h.fileDialogOnRadio()
}

// fileDialogOnMkdirRadio returns true when focus is on the mkdir-with-selections radio rows.
func (h *Handler) fileDialogOnMkdirRadio() bool {
	d := &h.model.FileDialog
	extra := h.mkdirExtraFocusRows()
	if extra == 0 {
		return false
	}
	base := len(d.Fields)
	return d.DialogType == dialog.FileDialogMkdir && d.FocusedField >= base && d.FocusedField < base+extra
}

func (h *Handler) runForEachExtraFocusRows() int {
	d := &h.model.FileDialog
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
func (h *Handler) fileDialogOnRunForEachPoolRadio() bool {
	d := &h.model.FileDialog
	extra := h.runForEachExtraFocusRows()
	if extra == 0 {
		return false
	}
	base := len(d.Fields)
	return d.DialogType == dialog.FileDialogRunForEach && d.FocusedField >= base && d.FocusedField < base+extra
}

// fileDialogOnMassRenameRadio returns true when focus is on one of the three mode radios.
func (h *Handler) fileDialogOnMassRenameRadio() bool {
	d := &h.model.FileDialog
	if d.DialogType != dialog.FileDialogMassRename {
		return false
	}
	return d.FocusedField >= 0 && d.FocusedField < 3
}

// fileDialogOnMassRenameCaseCheckbox returns true when focus is on the case-insensitive checkbox.
func (h *Handler) fileDialogOnMassRenameCaseCheckbox() bool {
	d := &h.model.FileDialog
	if d.DialogType != dialog.FileDialogMassRename {
		return false
	}
	idx := dialog.MassRenameCaseFocusIdx(*d)
	return idx >= 0 && d.FocusedField == idx
}

// fileDialogOnMassRenameStripCheckbox returns true when focus is on the "Strip spaces" checkbox.
func (h *Handler) fileDialogOnMassRenameStripCheckbox() bool {
	d := &h.model.FileDialog
	if d.DialogType != dialog.FileDialogMassRename {
		return false
	}
	return d.FocusedField == dialog.MassRenameStripFocusIdx(*d)
}

// fileDialogOnMassRenameShowModifiedCheckbox returns true when focus is on the "Show only modified" checkbox.
func (h *Handler) fileDialogOnMassRenameShowModifiedCheckbox() bool {
	d := &h.model.FileDialog
	if d.DialogType != dialog.FileDialogMassRename {
		return false
	}
	return d.FocusedField == dialog.MassRenameShowModifiedFocusIdx(*d)
}

// fileDialogOnRenameFocusCheckbox returns true when focus is on the focus-after-rename checkbox.
func (h *Handler) fileDialogOnRenameFocusCheckbox() bool {
	d := &h.model.FileDialog
	return dialog.FileDialogHasRenamePhase(d.DialogType) &&
		d.RenamePhase == dialog.RenamePhaseMain &&
		len(d.Fields) > 0 &&
		d.FocusedField == len(d.Fields)
}

// fileDialogRadioIndex returns the 0-based radio index when focus is on a
// mkdir radio row, or -1 otherwise.
func (h *Handler) fileDialogRadioIndex() int {
	if !h.fileDialogOnMkdirRadio() {
		return -1
	}
	return h.model.FileDialog.FocusedField - len(h.model.FileDialog.Fields)
}

func (h *Handler) runForEachPoolRadioIndex() int {
	if !h.fileDialogOnRunForEachPoolRadio() {
		return -1
	}
	return h.model.FileDialog.FocusedField - len(h.model.FileDialog.Fields)
}

func (h *Handler) selectFocusedRunForEachPoolRadio() {
	d := &h.model.FileDialog
	if d.DialogType != dialog.FileDialogRunForEach {
		return
	}
	idx := h.runForEachPoolRadioIndex()
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

// FileDialogOnButton returns true if current focus is on a button (not a field/radio).
func (h *Handler) FileDialogOnButton() bool {
	d := &h.model.FileDialog
	if d.DialogType == dialog.FileDialogDelete {
		return true // delete only has buttons
	}
	if dialog.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase != dialog.RenamePhaseMain {
		return d.FocusedField >= dialog.FileDialogOKFocusIndex(*d)
	}
	return d.FocusedField >= dialog.FileDialogOKFocusIndex(*d)
}

// tryRenameToolAltShortcut handles the per-phase Alt-letter mnemonics on the rename-tool
// dialogs (Sanitize: '.'/'_' toggle dots/underscores; Slugify: '.'/'_' pick separator;
// Encoding: per-candidate shortcut letter). Returns true when the rune was handled.
func (h *Handler) tryRenameToolAltShortcut(event *tcell.EventKey, d *dialog.FileDialogState) bool {
	switch d.RenamePhase {
	case dialog.RenamePhaseSanitize:
		switch event.Rune() {
		case '.':
			d.RenameSanitizeDots = !d.RenameSanitizeDots
			d.FocusedField = 0
			return true
		case '_':
			d.RenameSanitizeUnderscores = !d.RenameSanitizeUnderscores
			d.FocusedField = 1
			return true
		}
	case dialog.RenamePhaseSlugify:
		switch event.Rune() {
		case '.':
			d.RenameSlugifySep = dialog.RenameSlugifyDot
			d.FocusedField = 0
			return true
		case '_':
			d.RenameSlugifySep = dialog.RenameSlugifyUnderscore
			d.FocusedField = 1
			return true
		}
	case dialog.RenamePhaseEncoding:
		for i := 0; i < len(d.RenameEncodingCandidates); i++ {
			if event.Rune() == dialog.RenameEncodingCandidateShortcut(d.RenameEncodingCandidates[i].Label) {
				d.RenameEncodingSelected = i
				d.FocusedField = i
				return true
			}
		}
	}
	return false
}

// applyRenameToolAtFocus applies the rename-tool action for the current focused row
// (sanitize toggle / slugify pick / encoding pick), dispatched by d.RenamePhase. Shared
// by Enter (on non-button focus) and Space.
func (h *Handler) applyRenameToolAtFocus() {
	d := &h.model.FileDialog
	switch d.RenamePhase {
	case dialog.RenamePhaseSanitize:
		h.toggleRenameSanitizeAtFocus()
	case dialog.RenamePhaseSlugify:
		h.selectRenameSlugifyAtFocus()
	case dialog.RenamePhaseEncoding:
		h.selectRenameEncodingAtFocus()
	}
}

func (h *Handler) handleRenameToolKey(event *tcell.EventKey) bool {
	d := &h.model.FileDialog
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if dialog.TryStandardDialogActions(event, h.applyRenameToolAndReturnMain, h.closeRenameToolPhase, nil) {
			return false
		}
		if h.tryRenameToolAltShortcut(event, d) {
			return false
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.closeRenameToolPhase()
		return false
	case tcell.KeyEnter:
		okIdx := dialog.FileDialogOKFocusIndex(*d)
		cancelIdx := dialog.FileDialogCancelFocusIndex(*d)
		switch d.FocusedField {
		case okIdx:
			h.applyRenameToolAndReturnMain()
		case cancelIdx:
			h.closeRenameToolPhase()
		default:
			h.applyRenameToolAtFocus()
		}
		return false
	case tcell.KeyDown, tcell.KeyTab:
		h.fileDialogMoveFocusKey(event)
		return false
	case tcell.KeyUp, tcell.KeyBacktab:
		h.fileDialogMoveFocusKey(event)
		return false
	case tcell.KeyLeft, tcell.KeyRight:
		if h.FileDialogOnButton() {
			h.fileDialogMoveFocusKey(event)
		}
		return false
	case tcell.KeyRune:
		if keymap.IsPlainPrintableRune(event) && event.Rune() == ' ' {
			h.applyRenameToolAtFocus()
		}
		return false
	}
	return false
}

func (h *Handler) toggleRenameSanitizeAtFocus() {
	d := &h.model.FileDialog
	switch d.FocusedField {
	case 0:
		d.RenameSanitizeDots = !d.RenameSanitizeDots
	case 1:
		d.RenameSanitizeUnderscores = !d.RenameSanitizeUnderscores
	}
}

func (h *Handler) selectRenameSlugifyAtFocus() {
	d := &h.model.FileDialog
	switch d.FocusedField {
	case 0:
		d.RenameSlugifySep = dialog.RenameSlugifyDot
	case 1:
		d.RenameSlugifySep = dialog.RenameSlugifyUnderscore
	}
}
