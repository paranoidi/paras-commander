package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
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
	a.model.FlattenDialog = ui.FlattenDialogState{
		Open:        true,
		Destination: a.inactivePanel().PathString(),
		Recursive:   a.config.Operations.FlattenRecursive,
		RemoveEmpty: a.config.Operations.FlattenRemoveEmptyDirs,
		FocusField:  0,
		DirRoots:    rootStrs,
	}
	a.clearTransientMessage()
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
	a.model.FlattenDialog = ui.FlattenDialogState{}
}

func (a *App) handleFlattenDialogKey(event *tcell.EventKey) {
	d := &a.model.FlattenDialog
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'o', 'O':
			a.confirmFlatten()
			return
		case 'c', 'C':
			a.closeFlattenDialog()
			return
		case 'r', 'R':
			d.Recursive = !d.Recursive
			return
		case 'e', 'E':
			d.RemoveEmpty = !d.RemoveEmpty
			return
		}
	}
	if ui.AltDialogOK(event) {
		a.confirmFlatten()
		return
	}
	if ui.AltDialogCancel(event) || event.Key() == tcell.KeyEsc {
		a.closeFlattenDialog()
		return
	}
	if focus, ok := ui.FlattenDialogMoveFocus(d.FocusField, event.Key()); ok {
		d.FocusField = focus
		return
	}
	tform := ui.NewFlattenDialogLinearForm()
	if event.Key() == tcell.KeyEnter || event.Key() == ' ' {
		switch d.FocusField {
		case 0:
			d.Recursive = !d.Recursive
			return
		case 1:
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
}

func (a *App) confirmFlatten() {
	d := a.model.FlattenDialog
	roots, err := pathloc.ParseAll(d.DirRoots)
	if err != nil {
		a.setTransientMessage("Invalid flatten source paths", ui.MessageUrgencyWarn)
		return
	}
	destLoc, err := pathloc.Parse(d.Destination)
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
	a.addFlattenJob(sources, d.Destination, d.RemoveEmpty, d.DirRoots)
	noun := "items"
	if len(sources) == 1 {
		noun = "item"
	}
	a.setTransientMessage(fmt.Sprintf("Flatten queued (%d %s)", len(sources), noun), ui.MessageUrgencyInfo)
}
