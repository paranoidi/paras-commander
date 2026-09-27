package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// syncPathCompletion recomputes filesystem completion candidates for a path input's value.
func (h *Handler) syncPathCompletion(c *dialog.PathCompletion, value string, cursor int, dirsOnly bool) {
	start, cands, ok := pathpick.Suggest(h.host.ActivePanel().PathString(), h.model.UserHomeDir, value, cursor, h.host.Config().Panels.ShowHidden, dirsOnly)
	if !ok {
		c.Clear()
		return
	}
	c.Set(value, start, cands)
}

// resyncPathFieldCompletion recomputes filesystem completion for a path input field. A
// still-pending prefill offers no completion.
func (h *Handler) resyncPathFieldCompletion(f *dialog.FileDialogField, textWidth int) {
	if f == nil {
		return
	}
	if f.Prefill != "" && f.PrefillPending && f.Value == f.Prefill {
		f.Completion.Clear()
	} else {
		h.syncPathCompletion(&f.Completion, f.Value, f.Cursor, f.CompletionDirsOnly)
	}
	h.syncPathFieldScroll(f, textWidth)
}

// SyncPathFieldCompletion updates filesystem completion dropdown state on a path input field.
func (h *Handler) SyncPathFieldCompletion(f *dialog.FileDialogField, textWidth int) {
	h.resyncPathFieldCompletion(f, textWidth)
}

func (h *Handler) syncPathFieldScroll(f *dialog.FileDialogField, textWidth int) {
	if f == nil || textWidth <= 0 {
		return
	}
	valueLen := len([]rune(f.Value))
	suffixLen := len([]rune(f.Completion.GhostSuffix(f.Value)))
	f.Cursor, f.Scroll = dialog.EnsurePathInputScroll(valueLen, f.Cursor, f.Scroll, textWidth, suffixLen)
}

// tryPathFieldCompletionKey handles Tab/Up/Down/Enter/Esc for the completion dropdown on a
// path field. Returns true when the key was consumed by the dropdown. Callers gate on their own
// focus condition (file_input.go: f.PathPicker; dest_field.go's DestFieldTryCompletionKey:
// focusField == 0) since transfer/flatten's Destination field never sets
// FileDialogField.PathPicker (that flag only drives the generic file-dialog completion/shortcut
// gating). afterAccept (dialog-type-specific extras, e.g. mass-rename preview recompute, or
// arming the destination-validate timer) runs after a successful accept, before the completion
// is re-synced.
func (h *Handler) tryPathFieldCompletionKey(event *tcell.EventKey, f *dialog.FileDialogField, textWidth int, afterAccept func()) bool {
	if f == nil {
		return false
	}
	handled, accepted := dialog.HandlePathCompletionKey(event, &f.Completion, &f.Value, &f.Cursor)
	if !handled {
		return false
	}
	if accepted {
		if afterAccept != nil {
			afterAccept()
		}
		h.resyncPathFieldCompletion(f, textWidth)
	}
	return true
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

// TransferDestinationTextWidth returns the visible width of the transfer destination
// text row as painted, so Tab-complete scroll matches the drawn input.
func (h *Handler) TransferDestinationTextWidth() int {
	termW, termH := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(termW, termH)
	return dialog.TransferDestinationTextWidth(
		layout,
		h.model.TransferDialog,
		h.model.UserHomeDir,
		ui.DialogListIconLeadingWidth(h.model.UseNerdfontIcons),
	)
}
