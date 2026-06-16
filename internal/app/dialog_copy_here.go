package app

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// activateCopyHereAction copies the highlighted directory beside itself under a new name.
func (a *App) activateCopyHereAction() {
	a.openCopyHereDialog()
}

func (a *App) openCopyHereDialog() {
	p := a.activePanel()
	entry, err := ops.ValidateCopyHereSource(p)
	if err != nil {
		a.setErrorMessage("Copy here", err)
		return
	}
	name := entry.Name
	nameRunes := len([]rune(name))
	fields := []ui.FileDialogField{
		{Label: "Name", Value: name, Prefill: name, Cursor: nameRunes, PrefillPending: true},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:             true,
		DialogType:       ui.FileDialogCopyHere,
		Fields:           fields,
		CopyHereSource:   entry.Path,
		RenamePhase:      ui.RenamePhaseMain,
		RenameSlugifySep: ui.RenameSlugifyDot,
		RenameFocusAfter: a.config.Operations.RenameFocusAfter,
	}
}

func (a *App) executeCopyHere() {
	p := a.activePanel()
	d := &a.model.FileDialog
	field := a.focusedField()
	if field == nil {
		a.closeFileDialog()
		return
	}
	sourcePath := d.CopyHereSource
	if sourcePath == "" {
		a.closeFileDialog()
		return
	}
	entry, err := ops.ValidateCopyHereSource(p)
	if err != nil {
		a.setErrorMessage("Copy here source", err)
		a.closeFileDialog()
		return
	}
	if entry.Path != sourcePath {
		a.setErrorMessage("Copy here", ops.SourceError("source changed while dialog was open"))
		a.closeFileDialog()
		return
	}
	plan, err := ops.PlanCopyHere(entry, field.Value, p.PathString())
	if err != nil {
		a.setErrorMessage("Copy here", err)
		a.closeFileDialog()
		return
	}
	focusAfter := d.RenameFocusAfter
	listDir := p.PathString()
	panelID := a.model.ActivePanel
	a.closeFileDialog()
	a.addTransferJob(jobs.TypeCopy, []string{plan.SourcePath}, plan.DestPath, false, a.transferPreserveFromConfig())
	if focusAfter {
		a.scheduleCopyHereFocus(panelID, listDir, plan.NewName)
	}
	a.refreshBothPanels()
	a.setTransientMessage(fmt.Sprintf("Copy queued as %s", plan.NewName), ui.MessageUrgencyInfo)
}
