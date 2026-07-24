package dialog

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/bookmarks"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// OpenBookmarkDialog opens the fuzzy bookmarks path picker (fzf-marks entries merged with
// GNOME/GTK bookmarks) for navigating the active panel.
func (h *Handler) OpenBookmarkDialog() {
	if ui.IsAuxiliaryView(h.model.ViewMode) {
		return
	}
	if h.host.InQuickFilterUI() {
		h.host.ActivePanel().CancelFilter(h.host.ActiveViewportRows())
	}
	marks, err := bookmarks.LoadAll(h.host.Config().Bookmarks.File, h.model.UserHomeDir)
	if err != nil {
		h.host.SetErrorMessage("Bookmarks", err)
		return
	}
	panelPath := h.host.ActivePanel().PathString()
	home := h.model.UserHomeDir
	items := make([]dialog.PathPickerItem, len(marks))
	for i := range marks {
		cp := filepath.Clean(marks[i].Path)
		items[i] = dialog.PathPickerItem{
			Source:      marks[i].Origin.PathPickerSource(),
			Name:        marks[i].Name,
			Path:        marks[i].Path,
			PathMissing: PathEntryMissing(panelPath, home, cp),
		}
	}
	h.model.PathPicker = dialog.PathPickerState{
		Open:       true,
		Title:      "Bookmarks",
		Purpose:    dialog.PathPickerPurposeNavigate,
		Query:      "",
		Items:      items,
		Focus:      0,
		Selected:   0,
		ListScroll: 0,
	}
	h.SyncPathPickerRanks()
}

// OpenAddBookmarkDialog presents the centered dialog to append a new fzf-marks entry
// for the active panel directory. Refuses while jobs view is active and cancels any
// open quick filter (mirroring OpenBookmarkDialog).
func (h *Handler) OpenAddBookmarkDialog() {
	if ui.IsAuxiliaryView(h.model.ViewMode) {
		return
	}
	if h.host.InQuickFilterUI() {
		h.host.ActivePanel().CancelFilter(h.host.ActiveViewportRows())
	}
	path := h.host.ActivePanel().PathString()
	if strings.TrimSpace(path) == "" {
		h.host.SetErrorMessage("Add bookmark", fmt.Errorf("no active panel path"))
		return
	}
	defaultName := DefaultBookmarkName(path)
	cursor := len([]rune(defaultName))
	pending := defaultName != ""
	h.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogAddBookmark,
		Fields: []dialog.FileDialogField{
			{
				Label:          "Name",
				Value:          defaultName,
				Prefill:        defaultName,
				Cursor:         cursor,
				PrefillPending: pending,
			},
		},
		Message: path,
	}
}

// DefaultBookmarkName returns a sensible suggested mark name for path.
// Uses the basename, falling back to "root" for "/" or empty results.
func DefaultBookmarkName(path string) string {
	base := filepath.Base(filepath.Clean(path))
	switch base {
	case "", ".", string(filepath.Separator):
		return "root"
	}
	return base
}

// addBookmarkDialogInputField returns the name field for Add bookmark, including when
// keyboard focus is on OK/Cancel (focusedField returns nil in that case).
func (h *Handler) addBookmarkDialogInputField() *dialog.FileDialogField {
	d := &h.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogAddBookmark || len(d.Fields) != 1 {
		return nil
	}
	if d.FocusedField >= 0 && d.FocusedField < len(d.Fields) {
		return &d.Fields[d.FocusedField]
	}
	return &d.Fields[0]
}

// ExecuteAddBookmark validates the input, resolves the marks file, and appends
// a new mark line. Closes the dialog with a transient banner on success or error.
func (h *Handler) ExecuteAddBookmark() {
	field := h.addBookmarkDialogInputField()
	if field == nil {
		h.CloseFileDialog()
		return
	}
	name := strings.TrimSpace(field.Value)
	path := strings.TrimSpace(h.model.FileDialog.Message)
	if name == "" {
		h.host.SetErrorMessage("Add bookmark", fmt.Errorf("name is required"))
		h.CloseFileDialog()
		return
	}
	if path == "" {
		h.host.SetErrorMessage("Add bookmark", fmt.Errorf("missing target path"))
		h.CloseFileDialog()
		return
	}
	marksPath, err := bookmarks.ResolveFile(h.host.Config().Bookmarks.File, h.model.UserHomeDir)
	if err != nil {
		h.host.SetErrorMessage("Add bookmark", err)
		h.CloseFileDialog()
		return
	}
	if err := bookmarks.Append(marksPath, bookmarks.Mark{Name: name, Path: path}); err != nil {
		h.host.SetErrorMessage("Add bookmark", err)
		h.CloseFileDialog()
		return
	}
	h.CloseFileDialog()
	h.host.SetTransientMessage(fmt.Sprintf("Bookmark added: %s → %s", name, marksPath), ui.MessageUrgencyInfo)
}
