package dialog

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (h *Handler) filePickerOpen() bool {
	st := &h.model.PathPicker
	return st.Open && st.Purpose == dialog.PathPickerPurposeNavigateFile
}

// FilePickerFooterEligible reports whether the file-list picker is open with a row under the
// cursor, so the footer can advertise its actions.
func (h *Handler) FilePickerFooterEligible() bool {
	_, ok := h.pathPickerSelectedItem()
	return ok && h.filePickerOpen()
}

// TryFilePickerShortcut handles the Find-style actions of the file-list picker
// (PathPickerPurposeNavigateFile): the [dialog.find] view, edit and open-in-panel chords and the
// global select and pin toggles. Returns true when the event was consumed.
func (h *Handler) TryFilePickerShortcut(ev *tcell.EventKey) bool {
	if !h.filePickerOpen() {
		return false
	}
	if id, ok := h.keysFindDialog.Lookup(ev); ok {
		switch id {
		case keymap.ActionFindView:
			h.viewFilePickerCursor()
			return true
		case keymap.ActionFindEdit:
			h.editFilePickerCursor()
			return true
		case keymap.ActionOpenInPrimary:
			h.openFilePickerCursorIn(h.model.FirstSlotPanel())
			return true
		case keymap.ActionOpenInSecondary:
			h.openFilePickerCursorIn(h.model.SecondSlotPanel())
			return true
		}
	}
	id, ok := h.keysGlobal.Lookup(ev)
	switch {
	case ok && id == keymap.ActionPanelPinToggle:
		h.pinFilePickerCursor()
		return true
	// Plain space is filter text here, even though it is globally bound to select-toggle.
	case ok && id == keymap.ActionPanelSelectToggle && !keymap.IsPlainPrintableRune(ev):
		h.toggleFilePickerMark()
		return true
	}
	return false
}

// toggleFilePickerMark marks or unmarks the row under the cursor and moves to the next row.
func (h *Handler) toggleFilePickerMark() {
	st := &h.model.PathPicker
	item, ok := h.pathPickerSelectedItem()
	if !ok || !st.FilterFocused() {
		return
	}
	if st.Marked[item.Path] {
		delete(st.Marked, item.Path)
	} else {
		st.Marked[item.Path] = true
	}
	if st.Selected < len(st.Ranked)-1 {
		st.Selected++
		dialog.EnsurePathPickerListScroll(st, h.PathPickerListRows())
	}
}

// applyFilePickerMarks adds the marked files to the active panel's selection and closes the picker.
func (h *Handler) applyFilePickerMarks() {
	st := &h.model.PathPicker
	p := h.host.ActivePanel()
	var paths []string
	for file := range st.Marked {
		loc, err := pathloc.ForPanel(file, st.IndexPanel)
		if err != nil || p.SelectedPaths[loc.String()] {
			continue
		}
		paths = append(paths, loc.String())
	}
	conflicts := p.BulkAddSelections(paths, func(string) bool { return false })
	h.ClosePathPicker()
	host.ReportSelectionAdded(h.host, len(paths), conflicts)
}

// filePickerCursorFile is the host path under the cursor for an action that needs the real file;
// a row the picker knows is gone says so ("View: file is missing").
func (h *Handler) filePickerCursorFile(action string) (string, bool) {
	item, ok := h.pathPickerSelectedItem()
	if !ok {
		return "", false
	}
	if item.PathMissing {
		h.host.SetTransientMessage(action+": file is missing", ui.MessageUrgencyWarn)
		return "", false
	}
	return item.Path, true
}

// viewFilePickerCursor opens the file under the cursor in the fullscreen viewer; closing it brings
// the picker back as it was (ReopenFilePickerAfterPreviewClose).
func (h *Handler) viewFilePickerCursor() {
	file, ok := h.filePickerCursorFile("View")
	if !ok {
		return
	}
	st := &h.model.PathPicker
	st.Open, h.filePickerHiddenForView = false, true
	if err := h.preview.OpenFullscreenFilePreviewAt(file, false); err != nil {
		st.Open, h.filePickerHiddenForView = true, false
		h.host.SetTransientMessage("View: "+err.Error(), ui.MessageUrgencyWarn)
	}
}

// ReopenFilePickerAfterPreviewClose shows the file-list picker again, as it was, when the
// just-closed fullscreen viewer was opened from it (find.view); no-op otherwise.
func (h *Handler) ReopenFilePickerAfterPreviewClose() {
	if !h.filePickerHiddenForView {
		return
	}
	h.filePickerHiddenForView = false
	st := &h.model.PathPicker
	st.Open = true
	dialog.EnsurePathPickerListScroll(st, h.PathPickerListRows()) // the terminal may have resized
}

// editFilePickerCursor opens the file under the cursor in the external editor and keeps the picker open.
func (h *Handler) editFilePickerCursor() {
	if file, ok := h.filePickerCursorFile("Edit"); ok {
		h.host.EditFile(file)
	}
}

// openFilePickerCursorIn shows the file under the cursor in panelID and keeps the picker open.
func (h *Handler) openFilePickerCursorIn(panelID int) {
	item, ok := h.pathPickerSelectedItem()
	if !ok {
		return
	}
	name := filepath.Base(item.Path)
	if host.ShowInPanel(h.host, panelID, filepath.Dir(item.Path), name, h.model.PathPicker.IndexPanel, "Show file") {
		host.ReportOpenedInPanel(h.host, name, panelID)
	}
}

// pinFilePickerCursor pins or unpins the file under the cursor by the path its panel would use.
func (h *Handler) pinFilePickerCursor() {
	item, ok := h.pathPickerSelectedItem()
	if !ok {
		return
	}
	loc, err := pathloc.ForPanel(item.Path, h.model.PathPicker.IndexPanel)
	if err != nil {
		h.host.SetErrorMessage("Pin", err)
		return
	}
	h.host.PinTogglePath(filepath.Base(item.Path), loc.String(), false)
}
