package dialog

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// OK / Cancel entry points for the dialogs this handler owns. App.activeDialog maps
// ui.input.dialog-ok / dialog-cancel to these (see internal/app/active_dialog.go); the key
// handlers here never see those chords. Dialogs with sub-phases or an open completion
// dropdown pick the right layer themselves, so a Cancel closes the innermost layer first.

// closeCompletion closes the completion dropdown c, reporting whether it was open.
func closeCompletion(c *dialog.PathCompletion) bool {
	if c == nil || !c.Open {
		return false
	}
	c.Open = false
	return true
}

// fileDialogSubPhase returns OK / Cancel for the open file dialog's rename-tool /
// mass-rename prompt / picker / run-for-each history sub-phase (nil, nil when none is open).
func (h *Handler) fileDialogSubPhase() (ok, cancel func()) {
	d := &h.model.FileDialog
	switch {
	case !d.Open:
	case dialog.FileDialogHasRenamePhase(d.DialogType) && d.RenamePhase != dialog.RenamePhaseMain:
		return h.applyRenameToolAndReturnMain, h.closeRenameToolPhase
	case d.DialogType == dialog.FileDialogMassRename && d.MassRenamePhase == dialog.MassRenamePhaseSavePrompt:
		return h.confirmMassRenameSavePrompt, h.closeMassRenameSavePrompt
	case d.DialogType == dialog.FileDialogMassRename && massRenamePickerPhaseOpen(d.MassRenamePhase):
		return h.activateMassRenamePickerSelection, h.closeMassRenamePicker
	case d.DialogType == dialog.FileDialogRunForEach && d.RunForEachHistoryOpen:
		return h.activateRunForEachHistorySelection, h.closeRunForEachHistoryPicker
	}
	return nil, nil
}

// FileDialogOK applies the open file dialog's sub-phase, else the dialog.
func (h *Handler) FileDialogOK() {
	if ok, _ := h.fileDialogSubPhase(); ok != nil {
		ok()
		return
	}
	h.ExecuteFileDialog()
}

// FileDialogCancel closes the innermost layer of the open file dialog: a sub-phase, an
// open path completion dropdown, or the dialog itself.
func (h *Handler) FileDialogCancel() {
	if _, cancel := h.fileDialogSubPhase(); cancel != nil {
		cancel()
		return
	}
	if f := h.FocusedField(); f != nil && f.PathPicker && closeCompletion(&f.Completion) {
		return
	}
	h.CloseFileDialog()
}

// TransferDialogOK confirms the transfer (copy / move) dialog.
func (h *Handler) TransferDialogOK() { h.confirmTransfer() }

// TransferDialogCancel closes the destination completion dropdown, else the dialog.
func (h *Handler) TransferDialogCancel() {
	d := &h.model.TransferDialog
	if d.FocusOnDest() && closeCompletion(&d.Destination.Completion) {
		return
	}
	h.CloseTransferDialog()
}

// FlattenDialogOK confirms the flatten dialog.
func (h *Handler) FlattenDialogOK() { h.confirmFlatten() }

// FlattenDialogCancel closes the destination completion dropdown, else the dialog.
func (h *Handler) FlattenDialogCancel() {
	d := &h.model.FlattenDialog
	if d.FocusOnDest() && closeCompletion(&d.Destination.Completion) {
		return
	}
	h.CloseFlattenDialog()
}

// FilterDialogOK applies the Filter dialog (pattern row first).
func (h *Handler) FilterDialogOK() { h.confirmFilterDialogFromInput() }

// PathPickerOK activates the selected path picker item.
func (h *Handler) PathPickerOK() { h.activatePathPickerSelection() }

// PathPickerCancel closes the query completion dropdown, else the picker.
func (h *Handler) PathPickerCancel() {
	st := &h.model.PathPicker
	if st.FilterFocused() && closeCompletion(&st.Completion) {
		return
	}
	h.ClosePathPicker()
}
