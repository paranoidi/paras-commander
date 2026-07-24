package app

import (
	"fmt"
	"strings"

	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (a *App) openExtractDialog(p *panel.State) {
	source, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Extract", err)
		return
	}
	paths, skipped := ops.FilterArchiveEntries(source.Entries)
	if len(paths) == 0 {
		a.setTransientMessage("No supported archives selected", ui.MessageUrgencyWarn)
		return
	}
	dest := dialogctrl.TransferPrefilledDestination(a.inactivePanel().PathString())
	dest.Label = "Destination"
	dest.PathPicker = true
	fields := []dialog.FileDialogField{dest}
	msg := ""
	if skipped > 0 {
		msg = fmt.Sprintf("%d non-archive item(s) will be skipped.", skipped)
	}
	a.model.FileDialog = dialog.FileDialogState{
		Open:           true,
		DialogType:     dialog.FileDialogExtract,
		Fields:         fields,
		FocusedField:   0,
		Message:        msg,
		ExtractSources: append([]string(nil), paths...),
	}
	a.dialogCtrl.SyncFocusedFileDialogPathFieldCompletion()
	a.clearTransientMessage()
}

func (a *App) executeExtract() {
	fd := a.model.FileDialog
	field := a.dialogCtrl.FocusedField()
	if field == nil {
		a.dialogCtrl.CloseFileDialog()
		return
	}
	dest := strings.TrimSpace(field.Value)
	sources := append([]string(nil), fd.ExtractSources...)
	a.dialogCtrl.CloseFileDialog()
	if len(sources) == 0 {
		a.setTransientMessage("No archives to extract", ui.MessageUrgencyWarn)
		return
	}
	tc := archive.ProbeToolchain()
	plan, skipped, err := ops.PlanExtract(sources, dest, tc)
	if err != nil {
		a.openMessageDialog("Extract", err.Error())
		return
	}
	p := a.activePanel()
	p.ClearSelection()
	a.jobsCtrl.EnqueueExtractJob(ops.ExtractItemPaths(plan.Items), plan.Destination)
	n := len(plan.Items)
	noun := "archives"
	if n == 1 {
		noun = "archive"
	}
	msg := fmt.Sprintf("Extract queued (%d %s)", n, noun)
	if len(skipped) > 0 {
		msg += fmt.Sprintf("; %d skipped (unsupported or missing tool)", len(skipped))
	}
	a.setTransientMessage(msg, ui.MessageUrgencyInfo)
}
