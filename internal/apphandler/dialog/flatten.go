package dialog

import (
	"errors"
	"strings"

	"github.com/gdamore/tcell/v2"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// OpenFlattenDialog opens the flatten dialog for the active panel's selection, prefilling the
// destination from config's FlattenDefaultLocation (falling back to the active panel when the
// inactive panel is itself one of the flatten sources).
func (h *Handler) OpenFlattenDialog() {
	roots, err := ops.ValidateFlattenSource(h.host.ActivePanel())
	if err != nil {
		h.flattenSourceErrorToast(err)
		return
	}
	rootStrs := make([]string, len(roots))
	for i, r := range roots {
		rootStrs[i] = r.String()
	}
	destPanel := h.host.InactivePanel()
	if h.host.Config().Operations.FlattenDefaultLocation == config.FlattenDefaultLocationActive {
		destPanel = h.host.ActivePanel()
	}
	inactiveIsSource := false
	if destPanel == h.host.InactivePanel() {
		inactiveLoc, parseErr := pathloc.Parse(h.host.InactivePanel().PathString())
		if parseErr == nil {
			for _, root := range roots {
				if root.Equal(inactiveLoc) {
					inactiveIsSource = true
					destPanel = h.host.ActivePanel()
					break
				}
			}
		}
	}
	dest := TransferPrefilledDestination(destPanel.PathString())
	dest.CompletionDirsOnly = true
	h.model.FlattenDialog = dialog.FlattenDialogState{
		Open:        true,
		Destination: dest,
		Recursive:   h.host.Config().Operations.FlattenRecursive,
		RemoveEmpty: h.host.Config().Operations.FlattenRemoveEmptyDirs,
		Focus:       dialog.FlattenDest,
		DirRoots:    rootStrs,
	}
	h.host.ClearTransientMessage()
	if inactiveIsSource {
		h.host.SetTransientMessage("Destination set to active panel (inactive panel is the flatten source)", ui.MessageUrgencyWarn)
	}
	h.ArmFlattenDestinationValidateTimer()
}

func (h *Handler) flattenSourceErrorToast(err error) {
	var opsErr *ops.Error
	urgency := ui.MessageUrgencyWarn
	msg := err.Error()
	if errors.As(err, &opsErr) {
		msg = opsErr.Text
		if strings.Contains(opsErr.Text, "mix") {
			urgency = ui.MessageUrgencyError
		}
	}
	h.host.SetTransientMessage(msg, urgency)
}

// CloseFlattenDialog closes the flatten dialog and clears its destination-target panel markers.
func (h *Handler) CloseFlattenDialog() {
	h.InvalidateTransferDestValidate()
	h.invalidateRemoteFileOp()
	h.model.FlattenDialog = dialog.FlattenDialogState{}
	h.model.DestinationTargetPrimary = false
	h.model.DestinationTargetSecondary = false
}

// handleFlattenDestNavKey handles Left/Right cursor movement on the destination field while
// it is focused. Returns true when the key was handled (caller should return); false to fall
// through to the generic focus-move / Enter handling below.
func (h *Handler) handleFlattenDestNavKey(event *tcell.EventKey) bool {
	d := &h.model.FlattenDialog
	return h.DestFieldNav(event, &d.Destination, d.FocusOnDest())
}

// ApplyFlattenEvent acts on a widget event (key or click): a click on the destination moves its
// cursor, checkboxes toggle, Enter in the destination field or OK confirms, Cancel closes.
func (h *Handler) ApplyFlattenEvent(ev dlg.Event) {
	if ev.Kind == dlg.EventCaret {
		h.setPathFieldCursor(&h.model.FlattenDialog.Destination, ev.Index)
		return
	}
	h.model.FlattenDialog.Apply(ev)
	switch ev.ID {
	case dialog.FlattenDest, dlg.OK:
		h.confirmFlatten()
	case dlg.Cancel:
		h.CloseFlattenDialog()
	}
}

// HandleFlattenDialogKey dispatches a key event to the open flatten dialog.
func (h *Handler) HandleFlattenDialogKey(event *tcell.EventKey) {
	d := &h.model.FlattenDialog
	// Alt mnemonics (Recursive, Remove empty) win over every other chord, also in the destination field.
	if ev, ok := dlg.HandleMnemonic(dialog.FlattenView(*d), &d.Focus, event); ok {
		h.ApplyFlattenEvent(ev)
		return
	}
	if h.DestFieldTryCompletionKey(event, &d.Destination, d.FocusOnDest(), h.ArmFlattenDestinationValidateTimer) {
		return
	}
	if h.TryPathPickerHostShortcut(event) {
		return
	}
	if h.TryFlattenDialogDestinationShortcut(event) {
		return
	}
	if h.handleFlattenDestNavKey(event) {
		return
	}
	if ev, ok := dlg.HandleKey(dialog.FlattenView(*d), &d.Focus, event); ok {
		h.ApplyFlattenEvent(ev)
		return
	}
	if d.FocusOnDest() && h.editFlattenFieldKey(event, &d.Destination) {
		h.SyncPathFieldCompletion(&d.Destination, h.TransferDestinationTextWidth())
		h.ArmFlattenDestinationValidateTimer()
	}
}

func (h *Handler) editFlattenFieldKey(event *tcell.EventKey, f *dialog.FileDialogField) bool {
	return dialog.HandleFileDialogFieldKey(event, f, h.keysDialogInput, func() {
		h.SyncPathFieldCompletion(f, h.TransferDestinationTextWidth())
	})
}

func (h *Handler) confirmFlatten() {
	d := h.model.FlattenDialog
	roots, err := pathloc.ParseAll(d.DirRoots)
	if err != nil {
		h.host.SetTransientMessage("Invalid flatten source paths", ui.MessageUrgencyWarn)
		return
	}
	dest := strings.TrimSpace(d.Destination.Value)
	if dest == "" {
		h.host.SetTransientMessage("Destination required", ui.MessageUrgencyWarn)
		return
	}
	destLoc, err := h.resolveDestination(dest)
	if err != nil {
		h.host.SetTransientMessage("Invalid destination path", ui.MessageUrgencyWarn)
		return
	}
	if err := ops.ValidateFlattenTarget(roots, destLoc); err != nil {
		var opsErr *ops.Error
		if errors.As(err, &opsErr) {
			h.host.SetTransientMessage(opsErr.Text, ui.MessageUrgencyWarn)
		} else {
			h.host.SetErrorMessage("Flatten", err)
		}
		return
	}
	h.CloseFlattenDialog()
	h.host.ActivePanel().ClearSelection()
	h.jobs.AddFlattenJob(jobsctrl.FlattenJobRequest{
		Roots: d.DirRoots, Dest: destLoc.String(), Recursive: d.Recursive, RemoveEmpty: d.RemoveEmpty,
	})
	h.host.SetTransientMessage("Flatten queued", ui.MessageUrgencyInfo)
}
