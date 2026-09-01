package pin

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// listRows returns the list row count for a dialog sized to the terminal, delegating to
// dialog.PinDialogListRows so app-side and render-side sizing can't drift.
func (h *Handler) listRows() int {
	termW, termH := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(termW, termH)
	return dialog.PinDialogListRows(layout.Height)
}

// OpenDialog opens the Pin dialog, recomputing each pinned item's PathMissing (same
// recompute-once-at-open timing as History, not a live per-render stat).
func (h *Handler) OpenDialog() {
	if ui.IsAuxiliaryView(h.model.ViewMode) {
		return
	}
	if h.host.InQuickFilterUI() {
		h.host.CancelActiveQuickFilter()
	}
	if len(h.model.PinnedItems) == 0 {
		h.host.SetTransientMessage("No pinned items", ui.MessageUrgencyInfo)
		return
	}
	for i := range h.model.PinnedItems {
		h.model.PinnedItems[i].PathMissing = dialogctrl.PathEntryMissing("", "", h.model.PinnedItems[i].Path)
	}
	h.model.PinDialog = dialog.PinDialogState{Open: true, Selected: 0, ListScroll: 0}
	h.SyncDialogRanks()
}

// CloseDialog closes the Pin dialog.
func (h *Handler) CloseDialog() {
	h.model.PinDialog = dialog.PinDialogState{}
}

// SyncDialogRanks re-ranks PinnedItems against the dialog's current Query, clamps Selected,
// and re-syncs list scroll. Called on open, every query edit, and after F8 removes an item
// (a full rebuild, not an index patch — removing an item shifts every Ranked/MatchRanges
// entry keyed above it).
func (h *Handler) SyncDialogRanks() {
	st := &h.model.PinDialog
	if !st.Open {
		return
	}
	lines := make([]string, len(h.model.PinnedItems))
	for i, it := range h.model.PinnedItems {
		lines[i] = it.Path
	}
	st.Ranked, st.MatchRanges = h.host.SyncFilteredListRanks(lines, st.Query, len(h.model.PinnedItems), h.host.Config().Filter.CaseInsensitive)
	h.host.ClampFilteredListSelection(&st.Selected, len(st.Ranked))
	dialog.EnsurePinListScroll(st, h.listRows())
}

func (h *Handler) queryWidth() int {
	termW, _ := h.screen.Size()
	width := 78
	if width > termW-4 {
		width = termW - 4
	}
	if width < 36 {
		width = 36
	}
	return scrollquery.DialogInputWidthFromFrame(width)
}

// selectedItem resolves the Pin dialog's current selection to its index into PinnedItems
// and the item itself. False on an out-of-range selection.
func (h *Handler) selectedItem() (idx int, item ui.PinnedItem, ok bool) {
	st := &h.model.PinDialog
	if st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return 0, ui.PinnedItem{}, false
	}
	idx = st.Ranked[st.Selected]
	if idx < 0 || idx >= len(h.model.PinnedItems) {
		return 0, ui.PinnedItem{}, false
	}
	return idx, h.model.PinnedItems[idx], true
}

// panelLabel mirrors internal/app's identical helper (also used by dialog_settings.go and
// menu.go there, so it can't move) — trivial enough to duplicate rather than add a Host
// method for a two-branch string lookup.
func panelLabel(panelID int) string {
	if panelID == ui.PrimaryPanel {
		return "Primary panel"
	}
	return "Secondary panel"
}

// OpenSelected points panelID at the selected pin (cd into a directory; cd into a file's
// parent and highlight it), leaving the dialog open — mirrors find's navigateFindEntryToPanel.
// Returns true on success, false on an out-of-range selection or a navigation error (mirrors
// History's error handling: on error, report it and leave the dialog open).
func (h *Handler) OpenSelected(panelID int) bool {
	_, item, ok := h.selectedItem()
	if !ok {
		return false
	}
	dir, name := item.Path, ""
	if !item.IsDir {
		dir = filepath.Clean(filepath.Dir(item.Path))
		name = filepath.Base(item.Path)
	}
	if err := h.host.NavigatePanelToPath(panelID, dir, name); err != nil {
		h.host.SetErrorMessage("Pin", err)
		return false
	}
	h.host.PanelByID(panelID).EnsureCursorVisible(h.host.PanelViewportRows(panelID))
	h.host.SetTransientMessage(fmt.Sprintf("Opened %s in %s", filepath.Base(item.Path), strings.ToLower(panelLabel(panelID))), ui.MessageUrgencyInfo)
	return true
}

// ActivateSelection opens the selected pin in the currently-active panel (read live at call
// time, not stored at dialog-open time — Pin is opened globally, not per-panel) and closes
// the dialog on success only.
func (h *Handler) ActivateSelection() {
	if h.OpenSelected(h.model.ActivePanel) {
		h.CloseDialog()
	}
}

// RemoveSelected unpins the selected item (F8).
func (h *Handler) RemoveSelected() {
	idx, removed, ok := h.selectedItem()
	if !ok {
		return
	}
	h.model.PinnedItems = append(h.model.PinnedItems[:idx], h.model.PinnedItems[idx+1:]...)
	h.SyncDialogRanks()
	h.host.SetTransientMessage(fmt.Sprintf("Unpinned %s", filepath.Base(removed.Path)), ui.MessageUrgencyInfo)
}

// RemoveAll unpins every item (Shift+F8) and closes the dialog. No-op if there are no pins.
func (h *Handler) RemoveAll() {
	count := len(h.model.PinnedItems)
	if count == 0 {
		return
	}
	h.model.PinnedItems = nil
	h.CloseDialog()
	h.host.SetTransientMessage(fmt.Sprintf("Unpinned all (%d)", count), ui.MessageUrgencyInfo)
}

// ViewSelected opens the highlighted pin in the fullscreen file viewer (F3): a directory
// selection shows a transient warning and leaves the dialog open; a non-previewable file
// shows an error and leaves the dialog open. On success the Pin dialog is hidden (Open =
// false, not CloseDialog — its Query/Selected/ListScroll are preserved) and the fullscreen
// preview opens directly via h.preview; reopenAfterPreview is armed so
// ReopenAfterPreviewClose reopens the dialog, restored exactly, once the preview later
// closes. If opening the preview itself fails, there is no preview session to close later,
// so the dialog is restored immediately.
func (h *Handler) ViewSelected() {
	_, item, ok := h.selectedItem()
	if !ok {
		return
	}
	if item.IsDir {
		h.host.SetTransientMessage("View: not a file", ui.MessageUrgencyWarn)
		return
	}
	err := localfs.CheckFilePreviewable(item.Path)
	isImage := errors.Is(err, localfs.ErrFilePreviewImage)
	isMedia := errors.Is(err, localfs.ErrFilePreviewMedia)
	if err != nil && !isImage && !isMedia {
		if errors.Is(err, localfs.ErrFilePreviewBinary) {
			h.host.SetTransientMessage("View: not a text file", ui.MessageUrgencyWarn)
		} else {
			h.host.SetErrorMessage("View", err)
		}
		return
	}
	st := &h.model.PinDialog
	st.Open = false
	if err := h.preview.OpenFullscreenFilePreviewAt(item.Path); err != nil {
		st.Open = true
		h.host.SetTransientMessage("View: "+err.Error(), ui.MessageUrgencyWarn)
		return
	}
	h.reopenAfterPreview = true
}

// ReopenAfterPreviewClose reopens the Pin dialog, restored exactly as it was, when the
// just-closed F3 preview was launched from there (ViewSelected); no-op otherwise.
func (h *Handler) ReopenAfterPreviewClose() {
	if !h.reopenAfterPreview {
		return
	}
	h.reopenAfterPreview = false
	h.model.PinDialog.Open = true
	dialog.EnsurePinListScroll(&h.model.PinDialog, h.listRows())
}

func (h *Handler) HandleDialogKey(event *tcell.EventKey) {
	if h.keysPinDialog != nil {
		if id, ok := h.keysPinDialog.Lookup(event); ok {
			switch id {
			case keymap.ActionPinView:
				h.ViewSelected()
				return
			case keymap.ActionPinOpenInPrimary:
				h.OpenSelected(ui.PrimaryPanel)
				return
			case keymap.ActionPinOpenInSecondary:
				h.OpenSelected(ui.SecondaryPanel)
				return
			case keymap.ActionPinRemove:
				h.RemoveSelected()
				return
			case keymap.ActionPinRemoveAll:
				h.RemoveAll()
				return
			}
		}
	}
	if dialog.TryStandardDialogActions(event, h.ActivateSelection, h.CloseDialog, nil) {
		return
	}

	st := &h.model.PinDialog
	edit := scrollquery.NewEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, h.queryWidth(), func() {
		h.SyncDialogRanks()
		st.Selected = 0
		dialog.EnsurePinListScroll(st, h.listRows())
	})
	if scrollquery.HandleKey(h.keysDialogInput, event, true, edit) {
		return
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.CloseDialog()
	case tcell.KeyEnter:
		h.ActivateSelection()
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
		h.host.HandleFilteredListSelectionKey(event, 0, &st.Selected, len(st.Ranked), h.listRows, func() {
			dialog.EnsurePinListScroll(st, h.listRows())
		})
	}
}
