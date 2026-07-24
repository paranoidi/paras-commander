package app

import (
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/bookmarks"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// tryBookmarkDialogShortcut handles [dialog.bookmark] while the bookmarks
// path picker is open. Returns true when the event was consumed.
func (a *App) tryBookmarkDialogShortcut(ev *tcell.EventKey) bool {
	if a.keys.BookmarkDialog == nil {
		return false
	}
	if !a.bookmarkDialogOpen() {
		return false
	}
	id, ok := a.keys.BookmarkDialog.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionBookmarkDelete:
		return a.deleteSelectedBookmark()
	case keymap.ActionBookmarkOpenOther:
		return a.openSelectedBookmarkInInactivePanel()
	default:
		return false
	}
}

// openSelectedBookmarkInInactivePanel navigates the inactive panel to the
// selected bookmark path and closes the picker (like Enter). Returns true when handled.
func (a *App) openSelectedBookmarkInInactivePanel() bool {
	item, ok := a.pathPickerSelectedItem()
	if !ok {
		return false
	}
	path := filepath.Clean(item.Path)
	id := a.inactivePanelID()
	if err := a.navigatePanelToDirectory(id, path, ""); err != nil {
		a.setErrorMessage("Bookmark", err)
		return true
	}
	a.panelByID(id).EnsureCursorVisible(a.panelViewportRows(id))
	a.closePathPicker()
	a.setTransientMessage(fmt.Sprintf("Opened in other panel: %s", path), ui.MessageUrgencyInfo)
	return true
}

func (a *App) bookmarkDialogOpen() bool {
	st := &a.model.PathPicker
	return st.Open && st.Purpose == dialog.PathPickerPurposeNavigate
}

func (a *App) pathPickerSelectedItem() (dialog.PathPickerItem, bool) {
	st := &a.model.PathPicker
	if !st.Open || len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return dialog.PathPickerItem{}, false
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Items) {
		return dialog.PathPickerItem{}, false
	}
	return st.Items[entIdx], true
}

func (a *App) bookmarkDialogDeleteEligible() bool {
	if !a.bookmarkDialogOpen() {
		return false
	}
	item, ok := a.pathPickerSelectedItem()
	if !ok {
		return false
	}
	return item.Source == bookmarks.OriginFZFMarks.PathPickerSource()
}

func (a *App) bookmarkDialogDeleteFooterEligible() bool {
	if a.keys.BookmarkDialog == nil || !a.bookmarkDialogDeleteEligible() {
		return false
	}
	return a.keys.BookmarkDialog.MenuBindingLabel(keymap.ActionBookmarkDelete) != ""
}

// deleteSelectedBookmark removes the selected fzf-marks entry from disk and the open list.
// Returns true when the shortcut was handled (including errors shown to the user).
func (a *App) deleteSelectedBookmark() bool {
	if !a.bookmarkDialogDeleteEligible() {
		return false
	}
	item, ok := a.pathPickerSelectedItem()
	if !ok {
		return false
	}
	marksPath, err := bookmarks.ResolveFile(a.config.Bookmarks.File, a.model.UserHomeDir)
	if err != nil {
		a.setErrorMessage("Delete bookmark", err)
		return true
	}
	m := bookmarks.Mark{Name: item.Name, Path: item.Path, Origin: bookmarks.OriginFZFMarks}
	if err := bookmarks.Remove(marksPath, m); err != nil {
		a.setErrorMessage("Delete bookmark", err)
		return true
	}
	st := &a.model.PathPicker
	entIdx := st.Ranked[st.Selected]
	st.Items = append(st.Items[:entIdx], st.Items[entIdx+1:]...)
	a.syncPathPickerRanks()
	label := item.Name
	if label == "" {
		label = item.Path
	}
	a.setTransientMessage(fmt.Sprintf("Bookmark removed: %s", label), ui.MessageUrgencyInfo)
	return true
}
