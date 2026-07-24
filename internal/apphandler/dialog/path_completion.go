package dialog

import (
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// SyncPathFieldCompletion updates filesystem completion ghost text on a path input field.
func (h *Handler) SyncPathFieldCompletion(f *dialog.FileDialogField, textWidth int) {
	if f == nil {
		return
	}
	if f.Prefill != "" && f.PrefillPending && f.Value == f.Prefill {
		f.ClearCompletion()
		h.syncPathFieldScroll(f, textWidth)
		return
	}
	panel := h.host.ActivePanel()
	cfg := h.host.Config()
	c, ok := pathpick.SuggestAtCursor(panel.PathString(), h.model.UserHomeDir, f.Value, f.Cursor, cfg.Panels.ShowHidden)
	if !ok {
		f.ClearCompletion()
		h.syncPathFieldScroll(f, textWidth)
		return
	}
	f.CompletionSuffix = c.Suffix
	f.CompletionIsDir = c.IsDir
	h.syncPathFieldScroll(f, textWidth)
}

func (h *Handler) syncPathFieldScroll(f *dialog.FileDialogField, textWidth int) {
	if f == nil || textWidth <= 0 {
		return
	}
	valueLen := len([]rune(f.Value))
	suffixLen := len([]rune(f.CompletionSuffix))
	f.Cursor, f.Scroll = dialog.EnsurePathInputScroll(valueLen, f.Cursor, f.Scroll, textWidth, suffixLen)
}

// SyncOpenPathInputsAfterFSChange refreshes filesystem completion on open path fields
// after the directory listing may have changed (panel refresh, validation tick, etc.).
func (h *Handler) SyncOpenPathInputsAfterFSChange() {
	if h.model.PathPicker.Open {
		h.SyncPathPickerCompletion()
	}
	d := &h.model.TransferDialog
	if d.Open && d.Phase == dialog.TransferPhaseDestination {
		h.SyncPathFieldCompletion(&d.Destination, h.TransferDestinationTextWidth())
	}
	fd := &h.model.FlattenDialog
	if fd.Open {
		h.SyncPathFieldCompletion(&fd.Destination, h.TransferDestinationTextWidth())
	}
	if h.model.FileDialog.Open {
		for i := range h.model.FileDialog.Fields {
			f := &h.model.FileDialog.Fields[i]
			if f.PathPicker {
				h.SyncPathFieldCompletion(f, h.TransferDestinationTextWidth())
			}
		}
	}
}

// TransferDestinationTextWidth returns the visible width of the transfer/flatten/file-dialog
// destination text row (constant across those dialogs since they share PreferredFormDialogWidth).
func (h *Handler) TransferDestinationTextWidth() int {
	termW, _ := h.screen.Size()
	frameW := dialog.PreferredFormDialogWidth
	if frameW > termW-4 {
		frameW = termW - 4
	}
	if frameW < 36 {
		frameW = 36
	}
	return frameW - 4 - 2
}
