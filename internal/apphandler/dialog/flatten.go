package dialog

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
	h.model.FlattenDialog = dialog.FlattenDialogState{
		Open:         true,
		Destination:  TransferPrefilledDestination(destPanel.PathString()),
		DestSubFocus: dialog.FlattenDestSubFocusText,
		Recursive:    h.host.Config().Operations.FlattenRecursive,
		RemoveEmpty:  h.host.Config().Operations.FlattenRemoveEmptyDirs,
		FocusField:   0,
		DirRoots:     rootStrs,
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
	h.model.FlattenDialog = dialog.FlattenDialogState{}
	h.model.DestinationTargetPrimary = false
	h.model.DestinationTargetSecondary = false
}

// tryFlattenToggle handles the Recursive/RemoveEmpty toggle shortcuts: Alt+R/Alt+E always,
// and the plain r/e/Space mnemonics when the matching checkbox row is focused. Returns true
// when handled.
func (h *Handler) tryFlattenToggle(event *tcell.EventKey) bool {
	d := &h.model.FlattenDialog
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'r', 'R':
			d.Recursive = !d.Recursive
			return true
		case 'e', 'E':
			d.RemoveEmpty = !d.RemoveEmpty
			return true
		}
	}
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModNone {
		switch event.Rune() {
		case 'r', 'R':
			if d.FocusField == 1 {
				d.Recursive = !d.Recursive
				return true
			}
		case 'e', 'E':
			if d.FocusField == 2 {
				d.RemoveEmpty = !d.RemoveEmpty
				return true
			}
		case ' ':
			switch d.FocusField {
			case 1:
				d.Recursive = !d.Recursive
				return true
			case 2:
				d.RemoveEmpty = !d.RemoveEmpty
				return true
			}
		}
	}
	return false
}

// handleFlattenDestNavKey handles Left/Right cursor movement and picker sub-focus
// navigation on the destination field while it is focused. Returns true when the key was
// handled (caller should return); false to fall through to the generic focus-move / Enter
// handling below.
func (h *Handler) handleFlattenDestNavKey(event *tcell.EventKey) bool {
	d := &h.model.FlattenDialog
	return h.DestFieldNav(event, &d.Destination, &d.DestSubFocus, &d.FocusField,
		dialog.FlattenDestSubFocusText, dialog.FlattenDestSubFocusPicker, h.OpenPathPickerForFlatten)
}

// HandleFlattenDialogKey dispatches a key event to the open flatten dialog.
func (h *Handler) HandleFlattenDialogKey(event *tcell.EventKey) {
	d := &h.model.FlattenDialog
	if h.tryFlattenToggle(event) {
		return
	}
	if dialog.TryStandardDialogActions(event, h.confirmFlatten, h.CloseFlattenDialog, nil) {
		return
	}
	if event.Key() == tcell.KeyEsc {
		h.CloseFlattenDialog()
		return
	}
	if h.TryPathPickerHostShortcut(event) {
		return
	}
	if h.TryFlattenDialogDestinationShortcut(event) {
		return
	}
	if event.Key() == tcell.KeyTab &&
		h.DestFieldAcceptCompletion(&d.Destination, d.DestSubFocus, d.FocusField, dialog.FlattenDestSubFocusText, h.ArmFlattenDestinationValidateTimer) {
		return
	}
	if h.handleFlattenDestNavKey(event) {
		return
	}
	if focus, ok := dialog.FlattenDialogMoveFocus(d.FocusField, event.Key()); ok {
		prev := d.FocusField
		d.FocusField = focus
		if prev == 0 && focus != 0 {
			d.DestSubFocus = dialog.FlattenDestSubFocusText
		}
		return
	}
	if event.Key() == tcell.KeyEnter {
		tform := dialog.NewFlattenDialogLinearForm()
		if d.FocusField == 0 && d.DestSubFocus == dialog.FlattenDestSubFocusText {
			h.confirmFlatten()
			return
		}
		switch d.FocusField {
		case 1:
			d.Recursive = !d.Recursive
			return
		case 2:
			d.RemoveEmpty = !d.RemoveEmpty
			return
		case tform.OKIndex():
			h.confirmFlatten()
			return
		case tform.CancelIndex():
			h.CloseFlattenDialog()
			return
		}
	}
	if d.FocusField == 0 {
		if h.editFlattenFieldKey(event, &d.Destination) {
			h.SyncPathFieldCompletion(&d.Destination, h.TransferDestinationTextWidth())
			h.ArmFlattenDestinationValidateTimer()
			return
		}
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
	destLoc, err := pathloc.Parse(dest)
	if err != nil {
		h.host.SetTransientMessage("Invalid destination path", ui.MessageUrgencyWarn)
		return
	}
	sources, err := ops.CollectFlattenSources(context.Background(), roots, destLoc, d.Recursive)
	if err != nil {
		var opsErr *ops.Error
		if errors.As(err, &opsErr) {
			h.host.SetTransientMessage(opsErr.Text, ui.MessageUrgencyWarn)
		} else {
			h.host.SetErrorMessage("Flatten", err)
		}
		return
	}
	if len(sources) == 0 {
		h.host.SetTransientMessage("Nothing to flatten", ui.MessageUrgencyWarn)
		return
	}
	nSelf := 0
	for _, src := range sources {
		if ops.ResolvedSameAsSource(pathloc.MustParse(src), destLoc) {
			nSelf++
		}
	}
	if nSelf > 0 {
		if len(sources) > 1 {
			h.host.SetTransientMessage("Cannot flatten when some items would overwrite themselves", ui.MessageUrgencyWarn)
			return
		}
		h.host.SetTransientMessage("Nothing to flatten", ui.MessageUrgencyWarn)
		return
	}
	h.CloseFlattenDialog()
	h.host.ActivePanel().ClearSelection()
	h.jobs.AddFlattenJob(jobsctrl.FlattenJobRequest{
		Sources: sources, Dest: destLoc.String(), RemoveEmpty: d.RemoveEmpty, FlattenRoots: d.DirRoots,
	})
	noun := "items"
	if len(sources) == 1 {
		noun = "item"
	}
	h.host.SetTransientMessage(fmt.Sprintf("Flatten queued (%d %s)", len(sources), noun), ui.MessageUrgencyInfo)
}
