package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/bookmarks"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// tryBookmarkDialogShortcut handles [bookmark_dialog_action_keys] while the bookmarks
// path picker is open. Returns true when the event was consumed.
func (a *App) tryBookmarkDialogShortcut(ev *tcell.EventKey) bool {
	if a.keysBookmarkDialog == nil {
		return false
	}
	if !a.bookmarkDialogOpen() {
		return false
	}
	id, ok := a.keysBookmarkDialog.Lookup(ev)
	if !ok || id != keymap.ActionBookmarkDelete {
		return false
	}
	return a.deleteSelectedBookmark()
}

func (a *App) bookmarkDialogOpen() bool {
	st := &a.model.PathPicker
	return st.Open && st.Purpose == ui.PathPickerPurposeNavigate
}

func (a *App) pathPickerSelectedItem() (ui.PathPickerItem, bool) {
	st := &a.model.PathPicker
	if !st.Open || len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return ui.PathPickerItem{}, false
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Items) {
		return ui.PathPickerItem{}, false
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
	if a.keysBookmarkDialog == nil || !a.bookmarkDialogDeleteEligible() {
		return false
	}
	return a.keysBookmarkDialog.MenuBindingLabel(keymap.ActionBookmarkDelete) != ""
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
