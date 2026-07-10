package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
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

func (a *App) handleFlattenDialogKey(event *tcell.EventKey) {
	d := &a.model.FlattenDialog
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'r', 'R':
			d.Recursive = !d.Recursive
			return
		case 'e', 'E':
			d.RemoveEmpty = !d.RemoveEmpty
			return
		}
	}
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModNone {
		switch event.Rune() {
		case 'r', 'R':
			if d.FocusField == 1 {
				d.Recursive = !d.Recursive
				return
			}
		case 'e', 'E':
			if d.FocusField == 2 {
				d.RemoveEmpty = !d.RemoveEmpty
				return
			}
		case ' ':
			switch d.FocusField {
			case 1:
				d.Recursive = !d.Recursive
				return
			case 2:
				d.RemoveEmpty = !d.RemoveEmpty
				return
			}
		}
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
	if d.FocusField == 0 && d.DestSubFocus == dialog.FlattenDestSubFocusText &&
		event.Key() == tcell.KeyTab && d.Destination.CompletionSuffix != "" {
		if d.Destination.AcceptCompletion() {
			a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
			a.armFlattenDestinationValidateTimer()
			return
		}
		return
	}
	if d.FocusField == 0 {
		if d.DestSubFocus == dialog.FlattenDestSubFocusPicker {
			switch event.Key() {
			case tcell.KeyLeft:
				d.DestSubFocus = dialog.FlattenDestSubFocusText
				runes := []rune(d.Destination.Value)
				d.Destination.Cursor = len(runes)
				return
			case tcell.KeyEnter:
				a.openPathPickerForFlatten()
				return
			case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyDown, tcell.KeyUp:
				d.DestSubFocus = dialog.FlattenDestSubFocusText
			default:
				return
			}
		} else {
			switch event.Key() {
			case tcell.KeyRight:
				dest := &d.Destination
				runes := []rune(dest.Value)
				c := dest.Cursor
				if c < 0 {
					c = 0
				}
				if c > len(runes) {
					c = len(runes)
				}
				if dest.Prefill != "" && dest.PrefillPending && dest.Value == dest.Prefill && c >= len(runes) {
					dest.CommitPrefill()
					return
				}
				if c >= len(runes) {
					d.DestSubFocus = dialog.FlattenDestSubFocusPicker
					return
				}
				dest.MoveCursor(1)
				a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
				return
			case tcell.KeyLeft:
				d.Destination.MoveCursor(-1)
				a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
				return
			}
		}
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
	a.addFlattenJob(sources, destLoc.String(), d.RemoveEmpty, d.DirRoots)
	noun := "items"
	if len(sources) == 1 {
		noun = "item"
	}
	a.setTransientMessage(fmt.Sprintf("Flatten queued (%d %s)", len(sources), noun), ui.MessageUrgencyInfo)
}
