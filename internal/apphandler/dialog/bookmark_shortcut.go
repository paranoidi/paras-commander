package dialog

import (
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/bookmarks"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// TryBookmarkDialogShortcut handles [dialog.bookmark] while the bookmarks
// path picker is open. Returns true when the event was consumed.
func (h *Handler) TryBookmarkDialogShortcut(ev *tcell.EventKey) bool {
	if h.keysBookmarkDialog == nil {
		return false
	}
	if !h.bookmarkDialogOpen() {
		return false
	}
	id, ok := h.keysBookmarkDialog.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionBookmarkDelete:
		return h.deleteSelectedBookmark()
	case keymap.ActionBookmarkOpenOther:
		return h.openSelectedBookmarkInInactivePanel()
	default:
		return false
	}
}

// openSelectedBookmarkInInactivePanel navigates the inactive panel to the
// selected bookmark path and closes the picker (like Enter). Returns true when handled.
func (h *Handler) openSelectedBookmarkInInactivePanel() bool {
	item, ok := h.pathPickerSelectedItem()
	if !ok {
		return false
	}
	path := filepath.Clean(item.Path)
	id := h.host.InactivePanelID()
	if err := h.host.NavigatePanelToPath(id, path, ""); err != nil {
		h.host.SetErrorMessage("Bookmark", err)
		return true
	}
	h.host.PanelByID(id).EnsureCursorVisible(h.host.PanelViewportRows(id))
	h.ClosePathPicker()
	h.host.SetTransientMessage(fmt.Sprintf("Opened in other panel: %s", path), ui.MessageUrgencyInfo)
	return true
}

func (h *Handler) bookmarkDialogOpen() bool {
	st := &h.model.PathPicker
	return st.Open && st.Purpose == dialog.PathPickerPurposeNavigate
}

func (h *Handler) pathPickerSelectedItem() (dialog.PathPickerItem, bool) {
	st := &h.model.PathPicker
	if !st.Open || len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return dialog.PathPickerItem{}, false
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Items) {
		return dialog.PathPickerItem{}, false
	}
	return st.Items[entIdx], true
}

func (h *Handler) bookmarkDialogDeleteEligible() bool {
	if !h.bookmarkDialogOpen() {
		return false
	}
	item, ok := h.pathPickerSelectedItem()
	if !ok {
		return false
	}
	return item.Source == bookmarks.OriginFZFMarks.PathPickerSource()
}

// BookmarkDialogDeleteFooterEligible reports whether the F8 "Delete bookmark" footer hint
// should show: the bookmarks path picker is open with a deletable (fzf-marks-origin) entry
// selected.
func (h *Handler) BookmarkDialogDeleteFooterEligible() bool {
	if h.keysBookmarkDialog == nil || !h.bookmarkDialogDeleteEligible() {
		return false
	}
	return h.keysBookmarkDialog.MenuBindingLabel(keymap.ActionBookmarkDelete) != ""
}

// BookmarkDialogOpenOtherFooterEligible reports whether the bookmarks path picker is open with
// a selected item eligible for the "open in other panel" shortcut footer hint.
func (h *Handler) BookmarkDialogOpenOtherFooterEligible() bool {
	if h.keysBookmarkDialog == nil || !h.bookmarkDialogOpen() {
		return false
	}
	_, ok := h.pathPickerSelectedItem()
	return ok
}

// deleteSelectedBookmark removes the selected fzf-marks entry from disk and the open list.
// Returns true when the shortcut was handled (including errors shown to the user).
func (h *Handler) deleteSelectedBookmark() bool {
	if !h.bookmarkDialogDeleteEligible() {
		return false
	}
	item, ok := h.pathPickerSelectedItem()
	if !ok {
		return false
	}
	marksPath, err := bookmarks.ResolveFile(h.host.Config().Bookmarks.File, h.model.UserHomeDir)
	if err != nil {
		h.host.SetErrorMessage("Delete bookmark", err)
		return true
	}
	m := bookmarks.Mark{Name: item.Name, Path: item.Path, Origin: bookmarks.OriginFZFMarks}
	if err := bookmarks.Remove(marksPath, m); err != nil {
		h.host.SetErrorMessage("Delete bookmark", err)
		return true
	}
	st := &h.model.PathPicker
	entIdx := st.Ranked[st.Selected]
	st.Items = append(st.Items[:entIdx], st.Items[entIdx+1:]...)
	h.SyncPathPickerRanks()
	label := item.Name
	if label == "" {
		label = item.Path
	}
	h.host.SetTransientMessage(fmt.Sprintf("Bookmark removed: %s", label), ui.MessageUrgencyInfo)
	return true
}
