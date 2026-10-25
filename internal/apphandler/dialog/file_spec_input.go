package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// fileDialogSpec is the widget spec of the open FileDialogUsesSpec dialog, built for the current
// screen.
func (h *Handler) fileDialogSpec() dlg.Spec {
	w, ht := h.screen.Size()
	return dialog.FileDialogView(h.model.FileDialog, dialog.FileDialogViewContext{
		Layout:   h.host.LayoutForTerminalSize(w, ht),
		IconLead: ui.DialogListIconLeadingWidth(h.model.UseNerdfontIcons),
	})
}

// focusedFieldIndex is the index into Fields of the focused text input, or -1.
func (h *Handler) focusedFieldIndex() int {
	d := &h.model.FileDialog
	if dialog.FileDialogUsesSpec(d.DialogType) {
		return d.FileDialogFocusedField()
	}
	if d.FocusedField >= 0 && d.FocusedField < len(d.Fields) {
		return d.FocusedField
	}
	return -1
}

// applyFileDialogEvent acts on an activation from the widget spec: the state folds options in
// (Apply); Enter in a text input or OK runs the dialog (on a rename tool screen: applies the tool
// and returns to the name); Cancel closes (on a tool screen: returns to the name). Enter on a
// mkdir post-action radio selects it and runs the dialog.
func (h *Handler) applyFileDialogEvent(ev dlg.Event, key *tcell.EventKey) {
	d := &h.model.FileDialog
	d.Apply(ev)
	tool := dialog.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase != dialog.RenamePhaseMain
	_, field := dialog.FileFieldIndex(ev.ID)
	switch {
	case ev.ID == dlg.OK && tool:
		h.applyRenameToolAndReturnMain()
	case ev.ID == dlg.Cancel && tool:
		h.closeRenameToolPhase()
	case ev.ID == dlg.OK, field:
		h.ExecuteFileDialog()
	case ev.ID == dlg.Cancel:
		h.CloseFileDialog()
	default:
		if _, radio := dialog.MkdirActionFromID(ev.ID); radio && key.Key() == tcell.KeyEnter {
			h.ExecuteFileDialog()
		}
	}
}

// handleDeleteListScrollKey scrolls the delete confirmation's name list by a page on PgUp / PgDn.
func (h *Handler) handleDeleteListScrollKey(key tcell.Key) bool {
	d := &h.model.FileDialog
	_, ht := h.screen.Size()
	vp := dialog.DeleteDialogListViewportRows(ht, *d)
	switch key {
	case tcell.KeyPgUp:
		d.DeleteListScroll = max(d.DeleteListScroll-vp, 0)
	case tcell.KeyPgDn:
		dialog.DeleteEnsureListScroll(d, vp, len(d.DeleteEntries))
		d.DeleteListScroll += vp
		dialog.DeleteEnsureListScroll(d, vp, len(d.DeleteEntries))
	default:
		return false
	}
	return true
}

// handleFileDialogSpecKey routes a key for the open FileDialogUsesSpec dialog (rename, duplicate,
// mkdir, delete, chmod, chown, symlink, hardlink and the rename tool screens). Order: the
// dialog's own chords, list paging, Alt mnemonics (they win over everything, also in an input),
// the path-picker shortcuts and the completion dropdown, then the widget engine; what it declines
// is text editing of the focused input. Always returns false (the file dialog never quits).
func (h *Handler) handleFileDialogSpecKey(event *tcell.EventKey) bool {
	d := &h.model.FileDialog
	spec := h.fileDialogSpec()
	apply := func() bool {
		ev, ok := dlg.HandleKey(spec, &d.Focus, event)
		if ok {
			h.applyFileDialogEvent(ev, event)
		}
		return ok
	}
	if dialog.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase != dialog.RenamePhaseMain {
		apply()
		return false
	}
	if h.tryRenameDialogShortcut(event) || h.tryMkdirDialogShortcut(event) {
		return false
	}
	if d.DialogType == dialog.FileDialogDelete && h.handleDeleteListScrollKey(event.Key()) {
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
