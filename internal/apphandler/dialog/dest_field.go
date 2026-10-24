package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// DestFieldNav handles Left/Right cursor movement on a destination path field while
// the destination field is focused. Shared by the transfer and flatten dialogs.
// Returns true when the key was handled (caller should return).
func (h *Handler) DestFieldNav(event *tcell.EventKey, field *dialog.FileDialogField, focused bool) bool {
	if !focused || field == nil {
		return false
	}
	switch event.Key() {
	case tcell.KeyRight:
		runes := []rune(field.Value)
		c := field.Cursor
		if c < 0 {
			c = 0
		}
		if c > len(runes) {
			c = len(runes)
		}
		// Right on a pending placeholder commits it; otherwise Right at end-of-text is a no-op.
		if field.Prefill != "" && field.PrefillPending && field.Value == field.Prefill && c >= len(runes) {
			field.CommitPrefill()
			return true
		}
		if c >= len(runes) {
			return true
		}
		field.MoveCursor(1)
		h.SyncPathFieldCompletion(field, h.TransferDestinationTextWidth())
		return true
	case tcell.KeyLeft:
		field.MoveCursor(-1)
		h.SyncPathFieldCompletion(field, h.TransferDestinationTextWidth())
		return true
	}
	return false
}

// DestFieldTryCompletionKey handles Tab/Up/Down/Enter/Esc for the completion dropdown on a
// destination path field. Shared by the transfer and flatten dialogs.
func (h *Handler) DestFieldTryCompletionKey(event *tcell.EventKey, field *dialog.FileDialogField, focused bool, armValidate func()) bool {
	if !focused || field == nil {
		return false
	}
	return h.tryPathFieldCompletionKey(event, field, h.TransferDestinationTextWidth(), armValidate)
}
