package dialog

import (
	"fmt"
	"strings"

	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// OpenExtractDialog opens the archive-extract dialog for p's selection (or cursor entry),
// prefilling the destination with the inactive panel's path.
func (h *Handler) OpenExtractDialog(p *panel.State) {
	source, err := ops.ResolveSource(p)
	if err != nil {
		h.host.SetErrorMessage("Extract", err)
		return
	}
	paths, skipped := ops.FilterArchiveEntries(source.Entries)
	if len(paths) == 0 {
		h.host.SetTransientMessage("No supported archives selected", ui.MessageUrgencyWarn)
		return
	}
	dest := TransferPrefilledDestination(h.host.InactivePanel().PathString())
	dest.Label = "Destination"
	dest.PathPicker = true
	fields := []dialog.FileDialogField{dest}
	msg := ""
	if skipped > 0 {
		msg = fmt.Sprintf("%d non-archive item(s) will be skipped.", skipped)
	}
	h.model.FileDialog = dialog.FileDialogState{
		Open:           true,
		DialogType:     dialog.FileDialogExtract,
		Fields:         fields,
		FocusedField:   0,
		Message:        msg,
		ExtractSources: append([]string(nil), paths...),
	}
	h.SyncFocusedFileDialogPathFieldCompletion()
	h.host.ClearTransientMessage()
}

// ExecuteExtract runs the extract dialog's OK action: plans and queues an extract job for the
// archives gathered when the dialog was opened.
func (h *Handler) ExecuteExtract() {
	fd := h.model.FileDialog
	field := h.FocusedField()
	if field == nil {
		h.CloseFileDialog()
		return
	}
	dest := strings.TrimSpace(field.Value)
	sources := append([]string(nil), fd.ExtractSources...)
	h.CloseFileDialog()
	if len(sources) == 0 {
		h.host.SetTransientMessage("No archives to extract", ui.MessageUrgencyWarn)
		return
	}
	tc := archive.ProbeToolchain()
	plan, skipped, err := ops.PlanExtract(sources, dest, tc)
	if err != nil {
		h.host.OpenMessageDialog("Extract", err.Error())
		return
	}
	p := h.host.ActivePanel()
	p.ClearSelection()
	h.jobs.EnqueueExtractJob(ops.ExtractItemPaths(plan.Items), plan.Destination)
	n := len(plan.Items)
	noun := "archives"
	if n == 1 {
		noun = "archive"
	}
	msg := fmt.Sprintf("Extract queued (%d %s)", n, noun)
	if len(skipped) > 0 {
		msg += fmt.Sprintf("; %d skipped (unsupported or missing tool)", len(skipped))
	}
	h.host.SetTransientMessage(msg, ui.MessageUrgencyInfo)
}
