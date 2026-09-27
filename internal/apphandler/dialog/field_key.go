package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog"

// fileDialogExtra returns the dialog-type-specific recompute for the focused field (mass-rename
// preview, run-for-each validation), or nil. Split out of fileDialogFieldAfterEdit so the
// completion-dropdown key handler (tryPathFieldCompletionKey) can run just the extra without
// also re-syncing completion itself (it re-syncs completion on its own).
func (h *Handler) fileDialogExtra() func() {
	switch h.model.FileDialog.DialogType {
	case dialog.FileDialogMassRename:
		return h.RecomputeMassRenamePreview
	case dialog.FileDialogRunForEach:
		return h.commands.RecomputeRunForEachValidation
	default:
		return nil
	}
}

// fileDialogFieldAfterEdit builds the afterEdit callback for the currently focused field: a
// dialog-type-specific recompute (mass-rename preview, run-for-each validation) composed with
// path completion sync when the field is a path-picker field.
func (h *Handler) fileDialogFieldAfterEdit() func() {
	extra := h.fileDialogExtra()
	f := h.FocusedField()
	if f == nil || !f.PathPicker {
		return extra
	}
	textWidth := h.TransferDestinationTextWidth()
	return func() {
		if extra != nil {
			extra()
		}
		h.SyncPathFieldCompletion(f, textWidth)
	}
}

// SyncFocusedFileDialogPathFieldCompletion re-syncs filesystem completion for the currently
// focused field when it is a path-picker field (e.g. after opening a dialog with a prefilled
// path). No-op otherwise.
func (h *Handler) SyncFocusedFileDialogPathFieldCompletion() {
	f := h.FocusedField()
	if f == nil || !f.PathPicker {
		return
	}
	h.SyncPathFieldCompletion(f, h.TransferDestinationTextWidth())
}
