package dialog

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// PathPickerHostFooterEligible reports whether the currently focused dialog row is a
// path-picker-capable field (file dialog path field, or the transfer/flatten destination
// text row), so the footer can advertise the bookmark, history, pinned and all-paths shortcuts.
func (h *Handler) PathPickerHostFooterEligible() bool {
	if h.model.FileDialog.Open {
		if h.FileDialogOnButton() {
			return false
		}
		f := h.FocusedField()
		return f != nil && f.PathPicker
	}
	if h.model.TransferDialog.Open && h.model.TransferDialog.FocusOnDest() {
		return true
	}
	if h.model.FlattenDialog.Open && h.model.FlattenDialog.FocusOnDest() {
		return true
	}
	return false
}

// TryPathPickerHostShortcut opens a bookmarks, history, pinned, or combined path picker when
// the user presses bookmark.open, panel.history-dialog, panel.pin-dialog, or
// ui.input.path-picker-all while a path-picker host row is focused. A panel.tab-N chord there
// types the path of that tab of the destination panel into the field instead (applyTabPath).
func (h *Handler) TryPathPickerHostShortcut(ev *tcell.EventKey) bool {
	var kind pathPickerListKind
	if id, ok := h.lookupPathPickerAction(ev); ok {
		if slot, isTab := keymap.PanelTabSlot(id); isTab {
			if !h.PathPickerHostFooterEligible() {
				return false
			}
			h.applyTabPath(slot)
			return true
		}
		switch id {
		case keymap.ActionBookmarkOpen:
			kind = pathPickerListBookmarks
		case keymap.ActionPanelHistoryDialog:
			kind = pathPickerListHistory
		case keymap.ActionPanelPinDialog:
			kind = pathPickerListPinned
		case keymap.ActionDialogInputPathPickerAll:
			kind = pathPickerListAll
		default:
			return false
		}
	} else {
		return false
	}
	if !h.PathPickerHostFooterEligible() {
		return false
	}
	switch {
	case h.model.FileDialog.Open:
		h.OpenPathPickerForFileField(h.focusedFieldIndex(), kind)
	case h.model.TransferDialog.Open:
		h.OpenPathPickerForTransfer(kind)
	default:
		h.OpenPathPickerForFlatten(kind)
	}
	return true
}

// lookupPathPickerAction resolves ev against the dialog-input overlay first (the combined
// picker), then the global map.
func (h *Handler) lookupPathPickerAction(ev *tcell.EventKey) (string, bool) {
	if h.keysDialogInput != nil {
		if id, ok := h.keysDialogInput.Lookup(ev); ok && id == keymap.ActionDialogInputPathPickerAll {
			return id, true
		}
	}
	if h.keysGlobal == nil {
		return "", false
	}
	return h.keysGlobal.Lookup(ev)
}

// applyTabPath types the path of slot (1-based) of the destination panel into the focused path
// field; an unused slot leaves the field alone and says so.
func (h *Handler) applyTabPath(slot int) {
	loc := h.model.TabPath(h.host.DestinationPanelID(), slot)
	if loc.IsZero() {
		h.host.SetTransientMessage(fmt.Sprintf("Inactive panel has no tab %d", slot), ui.MessageUrgencyWarn)
		return
	}
	path := loc.String()
	switch {
	case h.model.FileDialog.Open && h.destinationDialogFooterEligible():
		h.applyDialogDestination(path)
	case h.model.FileDialog.Open:
		f := h.FocusedField()
		// Committed text (no pending prefill) so the user can go on typing a name.
		typed := TransferPrefilledDestination(path)
		f.Value, f.Cursor, f.Prefill, f.PrefillPending = typed.Value, typed.Cursor, "", false
		if extra := h.fileDialogExtra(); extra != nil {
			extra()
		}
		h.SyncFocusedFileDialogPathFieldCompletion()
	case h.model.TransferDialog.Open:
		h.applyTransferDestination(path)
	default:
		h.applyFlattenDestination(path)
	}
}
