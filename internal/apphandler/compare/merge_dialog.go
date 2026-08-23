package compare

import (
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
	form := dialog.NewCompareMergeDialogLinearForm()
	if dialog.AltDialogOK(event) {
		h.confirmMerge()
		return true
	}
	if dialog.AltDialogCancel(event) {
		h.closeMergeDialog()
		return true
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'l', 'L':
			d.Direction = comparepkg.MergeTowardPrimary
			h.refreshMergePreview(d)
			return true
		case 'r', 'R':
			d.Direction = comparepkg.MergeTowardSecondary
			h.refreshMergePreview(d)
			return true
		case 'm', 'M':
			d.CopyMissing = !d.CopyMissing
			h.refreshMergePreview(d)
			return true
		case 'f', 'F':
			d.CopyModified = !d.CopyModified
			h.refreshMergePreview(d)
			return true
		case 'k', 'K':
			d.MoveMode = false
			h.refreshMergePreview(d)
			return true
		case 'd', 'D':
			d.MoveMode = true
			h.refreshMergePreview(d)
			return true
		}
	}
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModNone {
		switch event.Rune() {
		case ' ':
			h.activateCompareMergeDialogFocus(d)
			return true
		}
	}
	if event.Key() == tcell.KeyEnter {
		if d.Focus == form.CancelIndex() {
			h.closeMergeDialog()
		} else if h.activateCompareMergeDialogFocus(d) {
			// radio/checkbox activated; stay open
		} else {
			h.confirmMerge()
		}
		return true
	}
	if nf, ok := dialog.CompareMergeDialogMoveFocus(d.Focus, event.Key()); ok {
		d.Focus = nf
		return true
	}
	return true
}

func (h *Handler) activateCompareMergeDialogFocus(d *dialog.CompareMergeDialogState) bool {
	switch d.Focus {
	case 0:
		d.Direction = comparepkg.MergeTowardPrimary
	case 1:
		d.Direction = comparepkg.MergeTowardSecondary
	case 2:
		d.CopyMissing = !d.CopyMissing
	case 3:
		d.CopyModified = !d.CopyModified
	case 4:
		d.MoveMode = false
	case 5:
		d.MoveMode = true
	default:
		return false
	}
	h.refreshMergePreview(d)
	return true
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
