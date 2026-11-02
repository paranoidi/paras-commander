package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// HandleFileDialogKey routes a key event to the open file dialog. Returns true when the event
// requests quitting the app (never for the file dialog itself; matches the App.handleKey
// (bool quit) convention shared across auxiliary key handlers).
func (h *Handler) HandleFileDialogKey(event *tcell.EventKey) bool {
	return h.handleFileDialogSpecKey(event)
}

// handleMassRenameEditorKey opens the external editor on F4 in the mass-rename dialog. Returns true
// when the key was handled.
func (h *Handler) handleMassRenameEditorKey(key tcell.Key) bool {
	if key != tcell.KeyF4 {
		return false
	}
	h.LaunchMassRenameExternalEditor()
	return true
}

// FocusedField returns the FileDialogField the file dialog currently has focus on, or nil when
// focus is on a button/radio/checkbox row or no dialog is open.
func (h *Handler) FocusedField() *dialog.FileDialogField {
	d := &h.model.FileDialog
	if i := d.FileDialogFocusedField(); i >= 0 {
		return &d.Fields[i]
	}
	return nil
}

// FileDialogOnButton returns true if current focus is on a button (not a field/radio).
func (h *Handler) FileDialogOnButton() bool {
	s := h.FileDialogScreen()
	id := dlg.Focused(s.Spec, *s.Focus)
	return id == dlg.OK || id == dlg.Cancel || id == dialog.MassRenameApply
}
