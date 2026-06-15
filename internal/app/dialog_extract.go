package app

import (
	"fmt"
	"strings"

	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
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
	dest := transferPrefilledDestination(a.inactivePanel().PathString())
	dest.Label = "Destination"
	dest.PathPicker = true
	fields := []ui.FileDialogField{dest}
	msg := ""
	if skipped > 0 {
		msg = fmt.Sprintf("%d non-archive item(s) will be skipped.", skipped)
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:           true,
		DialogType:     ui.FileDialogExtract,
		Fields:         fields,
		FocusedField:   0,
		Message:        msg,
		ExtractSources: append([]string(nil), paths...),
	}
	a.syncFocusedFileDialogPathFieldCompletion()
	a.clearTransientMessage()
}

func (a *App) executeExtract() {
	fd := a.model.FileDialog
	field := a.focusedField()
	if field == nil {
		a.closeFileDialog()
		return
	}
	dest := strings.TrimSpace(field.Value)
	sources := append([]string(nil), fd.ExtractSources...)
	a.closeFileDialog()
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
	a.enqueueExtractJob(ops.ExtractItemPaths(plan.Items), plan.Destination)
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
