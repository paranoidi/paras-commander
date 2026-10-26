package compare

import (
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// OpenMergeDialog opens the merge/sync dialog for the current compare snapshot.
func (h *Handler) OpenMergeDialog() {
	if h.model.ViewMode != ui.ViewCompare {
		return
	}
	d := dialog.CompareMergeDialogState{
		Open:          true,
		Direction:     comparepkg.MergeTowardSecondary,
		CopyMissing:   true,
		CopyModified:  true,
		MoveMode:      false,
		PrimaryPath:   h.model.CompareSnapshot.PrimaryRoot.String(),
		SecondaryPath: h.model.CompareSnapshot.SecondaryRoot.String(),
	}
	h.refreshMergePreview(&d)
	h.model.CompareMergeDialog = d
	h.host.ClearTransientMessage()
}

func (h *Handler) closeMergeDialog() {
	h.model.CompareMergeDialog = dialog.CompareMergeDialogState{}
}

func (h *Handler) refreshMergePreview(d *dialog.CompareMergeDialogState) {
	rows := h.FilteredRows()
	in := comparepkg.MergeInput{
		PrimarySelected:   h.model.Primary.SelectedPaths,
		SecondarySelected: h.model.Secondary.SelectedPaths,
		Filter:            h.model.CompareView.Filter,
	}
	opts := comparepkg.MergeOptions{
		Direction:    d.Direction,
		CopyMissing:  d.CopyMissing,
		CopyModified: d.CopyModified,
		MoveMode:     d.MoveMode,
	}
	copies, bytes := comparepkg.PreviewMergePlan(h.model.CompareSnapshot, rows, in, opts)
	d.PreviewText = formatMergePreviewText(d.MoveMode, copies, bytes)
}

func formatMergePreviewText(moveMode bool, n int, bytes int64) string {
	verb := "to copy"
	if moveMode {
		verb = "to move"
	}
	return fmt.Sprintf("Preview: %d %s (%s)", n, verb, ui.FormatSelectionByteSize(bytes))
}

// HandleMergeDialogKey routes keys for the open merge dialog. Returns true always while the
// dialog is open (matching the caller's "consumed" convention); false when the dialog is closed.
func (h *Handler) HandleMergeDialogKey(event *tcell.EventKey) bool {
	d := &h.model.CompareMergeDialog
	if !d.Open {
		return false
	}
	if ev, ok := dlg.HandleKey(dialog.CompareMergeView(*d, h.model.UserHomeDir), &d.Focus, event); ok {
		h.ApplyMergeDialogEvent(ev)
	}
	return true
}

// ApplyMergeDialogEvent acts on a widget event of the open merge dialog (key or click).
func (h *Handler) ApplyMergeDialogEvent(ev dlg.Event) {
	d := &h.model.CompareMergeDialog
	switch ev.ID {
	case "":
	case dlg.OK:
		h.confirmMerge()
	case dlg.Cancel:
		h.closeMergeDialog()
	default:
		d.Apply(ev)
		h.refreshMergePreview(d)
	}
}

func (h *Handler) confirmMerge() {
	d := h.model.CompareMergeDialog
	rows := h.FilteredRows()
	in := comparepkg.MergeInput{
		PrimarySelected:   h.model.Primary.SelectedPaths,
		SecondarySelected: h.model.Secondary.SelectedPaths,
		Filter:            h.model.CompareView.Filter,
	}
	opts := comparepkg.MergeOptions{
		Direction:    d.Direction,
		CopyMissing:  d.CopyMissing,
		CopyModified: d.CopyModified,
		MoveMode:     d.MoveMode,
	}
	plan, err := comparepkg.BuildMergePlan(h.model.CompareSnapshot, rows, in, opts)
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyWarn)
		return
	}
	preserve := jobs.TransferPreserveFromConfig(
		h.config.Operations.PreservePermissions,
		h.config.Operations.PreserveTimestamps,
		h.config.Operations.DereferenceSymlinks,
	)
	jobType := jobs.TypeCopy
	if d.MoveMode {
		jobType = jobs.TypeMove
	}
	for _, item := range plan.Copies {
		destDir := filepath.Dir(item.Dst)
		h.jobsCtrl.AddTransferJob(jobsctrl.TransferJobRequest{
			Type: jobType, Sources: []string{item.Src}, Dest: destDir, Preserve: preserve,
		})
	}
	h.closeMergeDialog()
	h.Refresh()
	verb := "copies"
	if d.MoveMode {
		verb = "moves"
	}
	msg := fmt.Sprintf("Merge queued (%d %s)", len(plan.Copies), verb)
	h.host.SetTransientMessage(msg, ui.MessageUrgencyInfo)
}

// MergeDialogOK confirms the merge dialog.
func (h *Handler) MergeDialogOK() { h.confirmMerge() }

// MergeDialogCancel closes the merge dialog.
func (h *Handler) MergeDialogCancel() { h.closeMergeDialog() }
