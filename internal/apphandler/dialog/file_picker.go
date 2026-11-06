package dialog

import (
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// TryFilePickerShortcut handles the Find-style actions of the file-list picker
// (PathPickerPurposeNavigateFile): the [dialog.find] view, edit and open-in-panel chords and the
// global select and pin toggles. Returns true when the event was consumed.
func (h *Handler) TryFilePickerShortcut(ev *tcell.EventKey) bool {
	if h.model.PathPicker.Purpose != dialog.PathPickerPurposeNavigateFile {
		return false
	}
	if h.keysFindDialog != nil {
		if id, ok := h.keysFindDialog.Lookup(ev); ok {
			switch id {
			case keymap.ActionFindView:
				h.viewFilePickerCursor()
				return true
			case keymap.ActionFindEdit:
				h.editFilePickerCursor()
				return true
			case keymap.ActionOpenInPrimary:
				h.openFilePickerCursorIn(ui.PrimaryPanel, "primary")
				return true
			case keymap.ActionOpenInSecondary:
				h.openFilePickerCursorIn(ui.SecondaryPanel, "secondary")
				return true
			}
		}
	}
	if h.keysGlobal != nil {
		if id, ok := h.keysGlobal.Lookup(ev); ok {
			switch {
			case id == keymap.ActionPanelPinToggle:
				h.pinFilePickerCursor()
				return true
			// Plain space is filter text here, even though it is globally bound to select-toggle.
			case id == keymap.ActionPanelSelectToggle && !keymap.IsPlainPrintableRune(ev):
				h.toggleFilePickerMark()
				return true
			}
		}
	}
	return false
}

// filePanelPath is where the host path file lives in a panel of the picker's kind.
func filePanelPath(file string, indexPanel bool) (pathloc.Path, error) {
	if indexPanel {
		return pathloc.FileIndex(file)
	}
	return pathloc.File(file)
}

// toggleFilePickerMark marks or unmarks the row under the cursor and moves to the next row.
func (h *Handler) toggleFilePickerMark() {
	st := &h.model.PathPicker
	item, ok := h.pathPickerSelectedItem()
	if !ok || !st.FilterFocused() {
		return
	}
	path := filepath.Clean(item.Path)
	if st.Marked[path] {
		delete(st.Marked, path)
	} else {
		if st.Marked == nil {
			st.Marked = map[string]bool{}
		}
		st.Marked[path] = true
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
		loc, err := filePanelPath(file, st.IndexPanel)
		if err != nil || p.SelectedPaths[loc.String()] {
			continue
		}
		paths = append(paths, loc.String())
	}
	conflicts := p.BulkAddSelections(paths, func(string) bool { return false })
	h.ClosePathPicker()
	switch {
	case len(paths) == 0:
		h.host.SetTransientMessage("No new selections", ui.MessageUrgencyInfo)
	case conflicts:
		h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
	default:
		h.host.SetTransientMessage(fmt.Sprintf("Added %d to selection", len(paths)), ui.MessageUrgencyInfo)
	}
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
	return filepath.Clean(item.Path), true
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
func (h *Handler) openFilePickerCursorIn(panelID int, which string) {
	item, ok := h.pathPickerSelectedItem()
	if !ok {
		return
	}
	file := filepath.Clean(item.Path)
	if h.showFileInPanel(panelID, file, h.model.PathPicker.IndexPanel) {
		h.host.SetTransientMessage(fmt.Sprintf("Opened %s in %s panel", filepath.Base(file), which), ui.MessageUrgencyInfo)
	}
}

// pinFilePickerCursor pins or unpins the file under the cursor by the path its panel would use.
func (h *Handler) pinFilePickerCursor() {
	item, ok := h.pathPickerSelectedItem()
	if !ok {
		return
	}
	file := filepath.Clean(item.Path)
	loc, err := filePanelPath(file, h.model.PathPicker.IndexPanel)
	if err != nil {
		h.host.SetErrorMessage("Pin", err)
		return
	}
	h.host.PinTogglePath(filepath.Base(file), loc.String(), false)
}
