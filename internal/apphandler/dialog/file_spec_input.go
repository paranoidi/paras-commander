package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// FileDialogSpec is the widget spec of the open file dialog, built for the current
// screen.
func (h *Handler) FileDialogSpec() dlg.Spec {
	w, ht := h.screen.Size()
	return dialog.FileDialogView(h.model.FileDialog, dialog.FileDialogViewContext{
		Layout:   h.host.LayoutForTerminalSize(w, ht),
		IconLead: ui.DialogListIconLeadingWidth(h.model.UseNerdfontIcons),
		Styles:   h.host.Styles(),
	})
}

// focusedFieldIndex is the index into Fields of the focused text input, or -1.
func (h *Handler) focusedFieldIndex() int { return h.model.FileDialog.FileDialogFocusedField() }

// applyFileDialogEvent acts on an activation from the widget spec: the state folds options in
// (Apply); Enter in a text input or OK runs the dialog (on a rename tool screen: applies the tool
// and returns to the name; on the mass rename save prompt: saves the pattern); Cancel closes the
// innermost layer. Enter on a mkdir post-action radio selects it and runs the dialog. The mass
// rename radios, checkboxes and Apply button are handled by applyMassRenameEvent.
func (h *Handler) applyFileDialogEvent(ev dlg.Event) {
	d := &h.model.FileDialog
	d.Apply(ev)
	switch d.DialogType {
	case dialog.FileDialogRunForEach:
		if ev.ID == dialog.RunForEachInDirs {
			h.commands.RecomputeRunForEachValidation()
		}
	case dialog.FileDialogMassRename:
		if h.applyMassRenameEvent(ev) {
			return
		}
	}
	_, field := dialog.FileFieldIndex(ev.ID)
	switch {
	case ev.ID == dlg.OK, field:
		h.FileDialogOK()
	case ev.ID == dlg.Cancel:
		h.cancelFileDialogLayer()
	default:
		if _, radio := dialog.MkdirActionFromID(ev.ID); radio && ev.Enter {
			h.ExecuteFileDialog()
		}
	}
}

// ApplyFileDialogEvent acts on a widget event of the open file dialog from a click: on a picker
// screen through the picker's handling, a click on a text input moves its cursor, anything else
// through applyFileDialogEvent like the keys.
func (h *Handler) ApplyFileDialogEvent(ev dlg.Event) {
	if b, ok := h.openPickerBinding(); ok {
		applyPickerEvent(ev, b)
		return
	}
	if ev.Kind != dlg.EventCaret {
		h.applyFileDialogEvent(ev)
		return
	}
	d := &h.model.FileDialog
	if i, ok := dialog.FileFieldIndex(ev.ID); ok && i < len(d.Fields) {
		if f := &d.Fields[i]; f.PathPicker {
			h.setPathFieldCursor(f, ev.Index)
		} else {
			f.SetCursor(ev.Index)
		}
	}
}

// cancelFileDialogLayer is the Cancel button: it leaves the innermost sub-phase (a rename tool,
// the mass rename save prompt), else closes the dialog.
func (h *Handler) cancelFileDialogLayer() {
	if _, cancel := h.fileDialogSubPhase(); cancel != nil {
		cancel()
		return
	}
	h.CloseFileDialog()
}

// handleFileDialogSpecKey routes a key for the open file dialog (rename, duplicate, mkdir, delete,
// chmod, chown, symlink, hardlink, add bookmark, SFTP password, extract, run-for-each, mass rename
// and their sub-screens). Order: the sub-screen pickers, the dialog's own chords,
// Alt mnemonics (they win over everything, also in an input), the path-picker shortcuts and the
// completion dropdown, then the widget engine; what it declines is text editing of the focused
// input. Always returns false (the file dialog never quits).
func (h *Handler) handleFileDialogSpecKey(event *tcell.EventKey) bool {
	d := &h.model.FileDialog
	switch {
	case d.DialogType == dialog.FileDialogRunForEach && d.RunForEachHistoryOpen:
		h.handlePickerKey(event, h.runForEachHistoryPickerBinding())
		return false
	case d.DialogType == dialog.FileDialogMassRename && massRenamePickerPhaseOpen(d.MassRenamePhase):
		return h.handleMassRenamePickerKey(event)
	}
	var spec *dlg.Spec // built on first use: most chords return before the engine runs
	apply := func() bool {
		if spec == nil {
			s := h.FileDialogSpec()
			spec = &s
		}
		ev, ok := dlg.HandleKey(*spec, &d.Focus, event)
		if ok {
			h.applyFileDialogEvent(ev)
		}
		return ok
	}
	if dialog.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase != dialog.RenamePhaseMain {
		apply()
		return false
	}
	if h.tryFileDialogChord(event) {
		return false
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) && apply() {
		return false
	}
	if h.TryPathPickerHostShortcut(event) {
		return false
	}
	f := h.FocusedField()
	if f != nil && f.PathPicker && h.tryPathFieldCompletionKey(event, f, h.TransferDestinationTextWidth(), nil) {
		return false
	}
	if apply() {
		return false
	}
	if f != nil {
		dialog.HandleFileDialogFieldKey(event, f, h.keysDialogInput, h.fileDialogFieldAfterEdit())
	}
	return false
}

// tryFileDialogChord handles the open dialog's own chords that run before the widget engine: the
// rename and mkdir shortcuts, the extract destination shortcuts, the run-for-each history key and
// the mass rename pattern and editor keys. Returns true when the key was consumed.
func (h *Handler) tryFileDialogChord(event *tcell.EventKey) bool {
	d := &h.model.FileDialog
	switch d.DialogType {
	case dialog.FileDialogRunForEach:
		return h.tryRunForEachDialogShortcut(event)
	case dialog.FileDialogExtract:
		return h.tryExtractDialogDestinationShortcut(event)
	case dialog.FileDialogMassRename:
		switch d.MassRenamePhase {
		case dialog.MassRenamePhaseMain:
			return h.tryMassRenameDialogShortcut(event) || h.handleMassRenameEditorKey(event.Key())
		case dialog.MassRenamePhaseSavePrompt:
			return h.tryMassRenameSavePromptShortcut(event)
		}
		return false
	}
	return h.tryRenameDialogShortcut(event) || h.tryMkdirDialogShortcut(event)
}
