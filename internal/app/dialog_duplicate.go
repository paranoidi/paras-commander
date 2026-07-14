package app

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// activateDuplicateAction copies the highlighted file or directory beside itself under a new name.
func (a *App) activateDuplicateAction() {
	a.openDuplicateDialog()
}

func (a *App) openDuplicateDialog() {
	p := a.activePanel()
	entry, err := ops.ValidateDuplicateSource(p)
	if err != nil {
		a.setErrorMessage("Duplicate", err)
		return
	}
	name := entry.Name
	nameRunes := len([]rune(name))
	fields := []dialog.FileDialogField{
		{Label: "Name", Value: name, Prefill: name, Cursor: nameRunes, PrefillPending: true},
	}
	a.model.FileDialog = dialog.FileDialogState{
		Open:             true,
		DialogType:       dialog.FileDialogDuplicate,
		Fields:           fields,
		DuplicateSource:  entry.Path,
		RenamePhase:      dialog.RenamePhaseMain,
		RenameSlugifySep: dialog.RenameSlugifyDot,
		RenameFocusAfter: a.config.Operations.RenameFocusAfter,
	}
}

func (a *App) executeDuplicate() {
	p := a.activePanel()
	d := &a.model.FileDialog
	if len(d.Fields) == 0 {
		a.closeFileDialog()
		return
	}
	newName := d.Fields[0].Value
	sourcePath := d.DuplicateSource
	if sourcePath == "" {
		a.closeFileDialog()
		return
	}
	entry, err := ops.ValidateDuplicateSource(p)
	if err != nil {
		a.setErrorMessage("Duplicate source", err)
		a.closeFileDialog()
		return
	}
	if entry.Path != sourcePath {
		a.setErrorMessage("Duplicate", ops.SourceError("source changed while dialog was open"))
		a.closeFileDialog()
		return
	}
	plan, err := ops.PlanDuplicate(entry, newName, p.PathString())
	if err != nil {
		a.setErrorMessage("Duplicate", err)
		a.closeFileDialog()
		return
	}
	focusAfter := d.RenameFocusAfter
	listDir := p.PathString()
	panelID := a.model.ActivePanel
	a.closeFileDialog()
	a.addTransferJob(jobs.TypeCopy, []string{plan.SourcePath}, plan.DestPath, false, a.transferPreserveFromConfig())
	if focusAfter {
		a.scheduleDuplicateFocus(panelID, listDir, plan.NewName)
	}
	a.refreshBothPanels()
	a.setTransientMessage(fmt.Sprintf("Copy queued as %s", plan.NewName), ui.MessageUrgencyInfo)
}
