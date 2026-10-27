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

// TryBookmarkDialogShortcut handles [dialog.bookmark] and the global
// panel.pin-toggle while the bookmarks path picker is open. Returns true when
// the event was consumed.
func (h *Handler) TryBookmarkDialogShortcut(ev *tcell.EventKey) bool {
	if !h.bookmarkDialogOpen() {
		return false
	}
	if h.keysGlobal != nil {
		if id, ok := h.keysGlobal.Lookup(ev); ok && id == keymap.ActionPanelPinToggle {
			if item, ok := h.pathPickerSelectedItem(); ok {
				path := filepath.Clean(item.Path)
				h.host.PinTogglePath(filepath.Base(path), path, true)
			}
			return true
		}
	}
	if h.keysBookmarkDialog == nil {
		return false
	}
	id, ok := h.keysBookmarkDialog.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionBookmarkDelete:
		return h.deleteSelectedBookmark()
	case keymap.ActionOpenInPrimary:
		return h.openSelectedBookmarkInPanel(ui.PrimaryPanel)
	case keymap.ActionOpenInSecondary:
		return h.openSelectedBookmarkInPanel(ui.SecondaryPanel)
	default:
		return false
	}
}

// openSelectedBookmarkInPanel navigates panelID to the selected bookmark path; the picker stays
// open (like the Pin dialog's open-in-panel). Returns true when handled.
func (h *Handler) openSelectedBookmarkInPanel(panelID int) bool {
	item, ok := h.pathPickerSelectedItem()
	if !ok {
		return false
	}
	path := filepath.Clean(item.Path)
	if err := h.host.NavigatePanelToPath(panelID, path, ""); err != nil {
		h.host.SetErrorMessage("Bookmark", err)
		return true
	}
	h.host.PanelByID(panelID).EnsureCursorVisible(h.host.PanelViewportRows(panelID))
	h.host.SetTransientMessage(fmt.Sprintf("Opened: %s", path), ui.MessageUrgencyInfo)
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

// BookmarkDialogOpenInPanelFooterEligible reports whether the bookmarks path picker is open with
// a selected item eligible for the open-in-panel shortcut footer hints.
func (h *Handler) BookmarkDialogOpenInPanelFooterEligible() bool {
	if h.keysBookmarkDialog == nil || !h.bookmarkDialogOpen() {
		return false
	}
	_, ok := h.pathPickerSelectedItem()
	return ok
}

// deleteSelectedBookmark removes the selected fzf-marks entry from disk and the open list.
// The atomic write runs on a worker; ApplyBookmarkIO updates the list when it completes.
// Returns true when the shortcut was handled (including errors shown to the user).
func (h *Handler) deleteSelectedBookmark() bool {
	if !h.bookmarkDialogDeleteEligible() {
		return false
	}
	item, ok := h.pathPickerSelectedItem()
	if !ok {
		return false
	}
	marksPath, err := bookmarks.ResolveFile(h.host.Config().Bookmarks.File, h.model.UserHomeDir, h.getenv)
	if err != nil {
		h.host.SetErrorMessage("Delete bookmark", err)
		return true
	}
	m := bookmarks.Mark{Name: item.Name, Path: item.Path, Origin: bookmarks.OriginFZFMarks}
	st := &h.model.PathPicker
	entIdx := st.Ranked[st.Selected]
	label := item.Name
	if label == "" {
		label = item.Path
	}
	gen := h.nextRemoteFileOpGen()
	result := make(chan BookmarkIOPayload, 1)
	go func() {
		stallBookmarkFileIO()
		err := bookmarks.Remove(marksPath, m)
		p := BookmarkIOPayload{
			Gen: gen, Err: err, Label: label, RemoveIndex: entIdx, kind: bookmarkIORemove,
		}
		if bookmarkIOInline() {
			result <- p
		} else {
			h.postBookmarkIO(p)
		}
	}()
	if bookmarkIOInline() {
		h.ApplyBookmarkIO(<-result)
	}
	return true
}
