package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// FileDialogRect returns the current on-screen file-dialog rect, used as a geometry
// signature to decide whether a per-keystroke overlay is valid. Returns the zero Rect
// when the dialog is not drawable. The Rect is a plain comparable int struct, so callers
// compare snapshots with ==; a change (e.g. mass-rename preview/hint rows resizing the
// dialog) forces a full render that correctly clears cells outside a now-smaller rect.
func (h *Handler) FileDialogRect() ui.Rect {
	w, ht := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(w, ht)
	r, ok := dialog.FileDialogRect(layout, h.model.FileDialog, ui.DialogListIconLeadingWidth(h.model.UseNerdfontIcons))
	if !ok {
		return ui.Rect{}
	}
	return r
}

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
