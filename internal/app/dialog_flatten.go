package app

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

func (a *App) openFlattenDialog() {
	roots, err := ops.ValidateFlattenSource(a.activePanel())
	if err != nil {
		a.flattenSourceErrorToast(err)
		return
	}
	rootStrs := make([]string, len(roots))
	for i, r := range roots {
		rootStrs[i] = r.String()
	}
	destPanel := a.inactivePanel()
	if a.config.Operations.FlattenDefaultLocation == config.FlattenDefaultLocationActive {
		destPanel = a.activePanel()
	}
	inactiveIsSource := false
	if destPanel == a.inactivePanel() {
		inactiveLoc, parseErr := pathloc.Parse(a.inactivePanel().PathString())
		if parseErr == nil {
			for _, root := range roots {
				if root.Equal(inactiveLoc) {
					inactiveIsSource = true
					destPanel = a.activePanel()
					break
				}
			}
		}
	}
	a.model.FlattenDialog = dialog.FlattenDialogState{
		Open:         true,
		Destination:  transferPrefilledDestination(destPanel.PathString()),
		DestSubFocus: dialog.FlattenDestSubFocusText,
		Recursive:    a.config.Operations.FlattenRecursive,
		RemoveEmpty:  a.config.Operations.FlattenRemoveEmptyDirs,
		FocusField:   0,
		DirRoots:     rootStrs,
	}
	a.clearTransientMessage()
	if inactiveIsSource {
		a.setTransientMessage("Destination set to active panel (inactive panel is the flatten source)", ui.MessageUrgencyWarn)
	}
	a.armFlattenDestinationValidateTimer()
}

func (a *App) flattenSourceErrorToast(err error) {
	var opsErr *ops.Error
	urgency := ui.MessageUrgencyWarn
	msg := err.Error()
	if errors.As(err, &opsErr) {
		msg = opsErr.Text
		if strings.Contains(opsErr.Text, "mix") {
			urgency = ui.MessageUrgencyError
		}
	}
	a.setTransientMessage(msg, urgency)
}

func (a *App) closeFlattenDialog() {
	a.transferDestValidate.Invalidate()
	a.model.FlattenDialog = dialog.FlattenDialogState{}
	a.model.DestinationTargetPrimary = false
	a.model.DestinationTargetSecondary = false
}

// tryFlattenToggle handles the Recursive/RemoveEmpty toggle shortcuts: Alt+R/Alt+E always,
// and the plain r/e/Space mnemonics when the matching checkbox row is focused. Returns true
// when handled.
func (a *App) tryFlattenToggle(event *tcell.EventKey) bool {
	d := &a.model.FlattenDialog
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
func (a *App) handleFlattenDestNavKey(event *tcell.EventKey) bool {
	d := &a.model.FlattenDialog
	return a.destFieldNav(event, &d.Destination, &d.DestSubFocus, &d.FocusField,
		dialog.FlattenDestSubFocusText, dialog.FlattenDestSubFocusPicker, a.openPathPickerForFlatten)
}

func (a *App) handleFlattenDialogKey(event *tcell.EventKey) {
	d := &a.model.FlattenDialog
	if a.tryFlattenToggle(event) {
		return
	}
	if a.tryStandardDialogActions(event, a.confirmFlatten, a.closeFlattenDialog, nil) {
		return
	}
	if event.Key() == tcell.KeyEsc {
		a.closeFlattenDialog()
		return
	}
	if a.tryPathPickerHostShortcut(event) {
		return
	}
	if a.tryFlattenDialogDestinationShortcut(event) {
		return
	}
	if event.Key() == tcell.KeyTab &&
		a.destFieldAcceptCompletion(&d.Destination, d.DestSubFocus, d.FocusField, dialog.FlattenDestSubFocusText, a.armFlattenDestinationValidateTimer) {
		return
	}
	if a.handleFlattenDestNavKey(event) {
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
			a.confirmFlatten()
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
			a.confirmFlatten()
			return
		case tform.CancelIndex():
			a.closeFlattenDialog()
			return
		}
	}
	if d.FocusField == 0 {
		if a.editFlattenFieldKey(event, &d.Destination) {
			a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
			a.armFlattenDestinationValidateTimer()
			return
		}
	}
}

func (a *App) editFlattenFieldKey(event *tcell.EventKey, f *dialog.FileDialogField) bool {
	return a.handleFileDialogFieldKey(event, f, func() {
		a.syncPathFieldCompletion(f, a.transferDestinationTextWidth())
	})
}

func (a *App) confirmFlatten() {
	d := a.model.FlattenDialog
	roots, err := pathloc.ParseAll(d.DirRoots)
	if err != nil {
		a.setTransientMessage("Invalid flatten source paths", ui.MessageUrgencyWarn)
		return
	}
	dest := strings.TrimSpace(d.Destination.Value)
	if dest == "" {
		a.setTransientMessage("Destination required", ui.MessageUrgencyWarn)
		return
	}
	destLoc, err := pathloc.Parse(dest)
	if err != nil {
		a.setTransientMessage("Invalid destination path", ui.MessageUrgencyWarn)
		return
	}
	sources, err := ops.CollectFlattenSources(context.Background(), roots, destLoc, d.Recursive)
	if err != nil {
		var opsErr *ops.Error
		if errors.As(err, &opsErr) {
			a.setTransientMessage(opsErr.Text, ui.MessageUrgencyWarn)
		} else {
			a.setErrorMessage("Flatten", err)
		}
		return
	}
	if len(sources) == 0 {
		a.setTransientMessage("Nothing to flatten", ui.MessageUrgencyWarn)
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
			a.setTransientMessage("Cannot flatten when some items would overwrite themselves", ui.MessageUrgencyWarn)
			return
		}
		a.setTransientMessage("Nothing to flatten", ui.MessageUrgencyWarn)
		return
	}
	a.closeFlattenDialog()
	a.activePanel().ClearSelection()
	a.jobsCtrl.AddFlattenJob(jobsctrl.FlattenJobRequest{
		Sources: sources, Dest: destLoc.String(), RemoveEmpty: d.RemoveEmpty, FlattenRoots: d.DirRoots,
	})
	noun := "items"
	if len(sources) == 1 {
		noun = "item"
	}
	a.setTransientMessage(fmt.Sprintf("Flatten queued (%d %s)", len(sources), noun), ui.MessageUrgencyInfo)
}
