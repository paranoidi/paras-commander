package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// DestFieldNav handles Left/Right cursor movement and text/picker sub-focus on a
// destination path field while FocusField is 0. Shared by the transfer and flatten dialogs.
// openPicker runs when Enter is pressed with picker sub-focus.
// Returns true when the key was handled (caller should return).
func (h *Handler) DestFieldNav(
	event *tcell.EventKey,
	field *dialog.FileDialogField,
	subFocus *int,
	focusField *int,
	textSub, pickerSub int,
	openPicker func(),
) bool {
	if focusField == nil || *focusField != 0 || field == nil || subFocus == nil {
		return false
	}
	if *subFocus == pickerSub {
		switch event.Key() {
		case tcell.KeyLeft:
			*subFocus = textSub
			runes := []rune(field.Value)
			field.Cursor = len(runes)
			return true
		case tcell.KeyEnter:
			if openPicker != nil {
				openPicker()
			}
			return true
		case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyDown, tcell.KeyUp:
			*subFocus = textSub
			return false
		default:
			return true
		}
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
		// First Right on a pending placeholder commits it; second Right at EOT moves to the glyph.
		if field.Prefill != "" && field.PrefillPending && field.Value == field.Prefill && c >= len(runes) {
			field.CommitPrefill()
			return true
		}
		if c >= len(runes) {
			*subFocus = pickerSub
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

// DestFieldAcceptCompletion accepts Tab completion on the destination text sub-focus.
func (h *Handler) DestFieldAcceptCompletion(field *dialog.FileDialogField, subFocus, focusField, textSub int, onAccepted func()) bool {
	if focusField != 0 || subFocus != textSub || field == nil {
		return false
	}
	if field.CompletionSuffix == "" {
		return false
	}
	if field.AcceptCompletion() {
		h.SyncPathFieldCompletion(field, h.TransferDestinationTextWidth())
		if onAccepted != nil {
			onAccepted()
		}
		return true
	}
	return true
}
