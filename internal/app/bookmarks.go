package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/bookmarks"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) openBookmarkDialog() {
	if ui.IsAuxiliaryView(a.model.ViewMode) {
		return
	}
	if a.inQuickFilterUI() {
		a.activePanel().CancelFilter(a.activeViewportRows())
	}
	marks, err := bookmarks.LoadAll(a.config.Bookmarks.File, a.model.UserHomeDir)
	if err != nil {
		a.setErrorMessage("Bookmarks", err)
		return
	}
	items := make([]ui.PathPickerItem, len(marks))
	for i := range marks {
		items[i] = ui.PathPickerItem{
			Source: marks[i].Origin.PathPickerSource(),
			Name:   marks[i].Name,
			Path:   marks[i].Path,
		}
	}
	a.model.PathPicker = ui.PathPickerState{
		Open:       true,
		Title:      "Bookmarks",
		Purpose:    ui.PathPickerPurposeNavigate,
		Query:      "",
		Items:      items,
		Focus:      0,
		Selected:   0,
		ListScroll: 0,
	}
	a.syncPathPickerRanks()
}

// openAddBookmarkDialog presents the centered dialog to append a new fzf-marks entry
// for the active panel directory. Refuses while jobs view is active and cancels any
// open quick filter (mirroring openBookmarkDialog).
func (a *App) openAddBookmarkDialog() {
	if ui.IsAuxiliaryView(a.model.ViewMode) {
		return
	}
	if a.inQuickFilterUI() {
		a.activePanel().CancelFilter(a.activeViewportRows())
	}
	path := a.activePanel().PathString()
	if strings.TrimSpace(path) == "" {
		a.setErrorMessage("Add bookmark", fmt.Errorf("no active panel path"))
		return
	}
	defaultName := defaultBookmarkName(path)
	cursor := len([]rune(defaultName))
	pending := defaultName != ""
	a.model.FileDialog = ui.FileDialogState{
		Open:       true,
		DialogType: ui.FileDialogAddBookmark,
		Fields: []ui.FileDialogField{
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

// defaultBookmarkName returns a sensible suggested mark name for path.
// Uses the basename, falling back to "root" for "/" or empty results.
func defaultBookmarkName(path string) string {
	base := filepath.Base(filepath.Clean(path))
	switch base {
	case "", ".", string(filepath.Separator):
		return "root"
	}
	return base
}

// addBookmarkDialogInputField returns the name field for Add bookmark, including when
// keyboard focus is on OK/Cancel (focusedField returns nil in that case).
func (a *App) addBookmarkDialogInputField() *ui.FileDialogField {
	d := &a.model.FileDialog
	if !d.Open || d.DialogType != ui.FileDialogAddBookmark || len(d.Fields) != 1 {
		return nil
	}
	if d.FocusedField >= 0 && d.FocusedField < len(d.Fields) {
		return &d.Fields[d.FocusedField]
	}
	return &d.Fields[0]
}

// executeAddBookmark validates the input, resolves the marks file, and appends
// a new mark line. Closes the dialog with a transient banner on success or error.
func (a *App) executeAddBookmark() {
	field := a.addBookmarkDialogInputField()
	if field == nil {
		a.closeFileDialog()
		return
	}
	name := strings.TrimSpace(field.Value)
	path := strings.TrimSpace(a.model.FileDialog.Message)
	if name == "" {
		a.setErrorMessage("Add bookmark", fmt.Errorf("name is required"))
		a.closeFileDialog()
		return
	}
	if path == "" {
		a.setErrorMessage("Add bookmark", fmt.Errorf("missing target path"))
		a.closeFileDialog()
		return
	}
	marksPath, err := bookmarks.ResolveFile(a.config.Bookmarks.File, a.model.UserHomeDir)
	if err != nil {
		a.setErrorMessage("Add bookmark", err)
		a.closeFileDialog()
		return
	}
	if err := bookmarks.Append(marksPath, bookmarks.Mark{Name: name, Path: path}); err != nil {
		a.setErrorMessage("Add bookmark", err)
		a.closeFileDialog()
		return
	}
	a.closeFileDialog()
	a.setTransientMessage(fmt.Sprintf("Bookmark added: %s → %s", name, marksPath), ui.MessageUrgencyInfo)
}
