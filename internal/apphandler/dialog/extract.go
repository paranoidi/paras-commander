package dialog

import (
	"errors"
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
	if len(sources) == 0 {
		h.CloseFileDialog()
		h.host.SetTransientMessage("No archives to extract", ui.MessageUrgencyWarn)
		return
	}
	if h.useRemoteFileOp(dest) {
		h.startRemoteExtractProbe(sources, dest)
		return
	}
	h.CloseFileDialog()
	tc := archive.ProbeToolchain()
	plan, skipped, err := ops.PlanExtract(sources, dest, tc)
	if extractPlanDestFailed(err) {
		h.host.OpenMessageDialog("Extract", err.Error())
		return
	}
	if len(plan.Items) == 0 && !extractSkipsIncludeKeepExisting(skipped) {
		if err != nil {
			h.host.OpenMessageDialog("Extract", err.Error())
			return
		}
		h.host.SetTransientMessage("No archives to extract", ui.MessageUrgencyWarn)
		return
	}
	// Queue the dialog's archives, including stream outputs PlanExtract skipped
	// as existing/colliding, so the extract job's Conflict resolver can keep
	// existing files or open the overwrite blocker.
	h.finishExtractEnqueueSources(sources, dest, skipped)
}

func extractPlanDestFailed(err error) bool {
	if err == nil {
		return false
	}
	var opErr *ops.Error
	if !errors.As(err, &opErr) {
		return false
	}
	return strings.HasPrefix(opErr.Text, "destination")
}

func extractSkipIsKeepExisting(reason string) bool {
	return strings.Contains(reason, "already exists") || strings.Contains(reason, "collides")
}

func extractSkipsIncludeKeepExisting(skipped []string) bool {
	for _, s := range skipped {
		if extractSkipIsKeepExisting(s) {
			return true
		}
	}
	return false
}

func extractQueuedMessage(n int, skipped []string) string {
	noun := "archives"
	if n == 1 {
		noun = "archive"
	}
	msg := fmt.Sprintf("Extract queued (%d %s)", n, noun)
	other := 0
	for _, s := range skipped {
		if !extractSkipIsKeepExisting(s) {
			other++
		}
	}
	if other > 0 {
		msg += fmt.Sprintf("; %d skipped (unsupported or missing tool)", other)
	}
	return msg
}

func (h *Handler) finishExtractEnqueueSources(sources []string, dest string, skipped []string) {
	p := h.host.ActivePanel()
	p.ClearSelection()
	h.jobs.EnqueueExtractJob(sources, dest)
	h.host.SetTransientMessage(extractQueuedMessage(len(sources), skipped), ui.MessageUrgencyInfo)
}

func (h *Handler) finishExtractEnqueue(plan ops.ExtractPlan, skipped []string) {
	h.finishExtractEnqueueSources(ops.ExtractItemPaths(plan.Items), plan.Destination, skipped)
}
