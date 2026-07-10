package app

import (
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (a *App) openCompareMergeDialog() {
	if a.model.ViewMode != ui.ViewCompare {
		return
	}
	d := dialog.CompareMergeDialogState{
		Open:          true,
		Direction:     comparepkg.MergeTowardSecondary,
		CopyMissing:   true,
		CopyModified:  true,
		MoveMode:      false,
		PrimaryPath:   a.model.CompareSnapshot.PrimaryRoot.String(),
		SecondaryPath: a.model.CompareSnapshot.SecondaryRoot.String(),
	}
	a.refreshCompareMergePreview(&d)
	a.model.CompareMergeDialog = d
	a.clearTransientMessage()
}

func (a *App) closeCompareMergeDialog() {
	a.model.CompareMergeDialog = dialog.CompareMergeDialogState{}
}

func (a *App) refreshCompareMergePreview(d *dialog.CompareMergeDialogState) {
	rows := a.compareCtrl.FilteredRows()
	in := comparepkg.MergeInput{
		PrimarySelected:   a.model.Primary.SelectedPaths,
		SecondarySelected: a.model.Secondary.SelectedPaths,
		Filter:            a.model.CompareView.Filter,
	}
	opts := comparepkg.MergeOptions{
		Direction:    d.Direction,
		CopyMissing:  d.CopyMissing,
		CopyModified: d.CopyModified,
		MoveMode:     d.MoveMode,
	}
	copies, bytes := comparepkg.PreviewMergePlan(a.model.CompareSnapshot, rows, in, opts)
	d.PreviewText = formatMergePreviewText(d.MoveMode, copies, bytes)
}

func formatMergePreviewText(moveMode bool, n int, bytes int64) string {
	verb := "to copy"
	if moveMode {
		verb = "to move"
	}
	return fmt.Sprintf("Preview: %d %s (%s)", n, verb, ui.FormatSelectionByteSize(bytes))
}

func (a *App) handleCompareMergeDialogKey(event *tcell.EventKey) bool {
	d := &a.model.CompareMergeDialog
	if !d.Open {
		return false
	}
	form := dialog.NewCompareMergeDialogLinearForm()
	if a.tryStandardDialogActions(event, a.confirmCompareMerge, a.closeCompareMergeDialog, nil) {
		return true
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'l', 'L':
			d.Direction = comparepkg.MergeTowardPrimary
			a.refreshCompareMergePreview(d)
			return true
		case 'r', 'R':
			d.Direction = comparepkg.MergeTowardSecondary
			a.refreshCompareMergePreview(d)
			return true
		case 'm', 'M':
			d.CopyMissing = !d.CopyMissing
			a.refreshCompareMergePreview(d)
			return true
		case 'f', 'F':
			d.CopyModified = !d.CopyModified
			a.refreshCompareMergePreview(d)
			return true
		case 'k', 'K':
			d.MoveMode = false
			a.refreshCompareMergePreview(d)
			return true
		case 'd', 'D':
			d.MoveMode = true
			a.refreshCompareMergePreview(d)
			return true
		}
	}
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModNone {
		switch event.Rune() {
		case ' ':
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
			}
			a.refreshCompareMergePreview(d)
			return true
		}
	}
	if event.Key() == tcell.KeyEnter {
		if d.Focus == form.CancelIndex() {
			a.closeCompareMergeDialog()
		} else {
			a.confirmCompareMerge()
		}
		return true
	}
	if nf, ok := dialog.CompareMergeDialogMoveFocus(d.Focus, event.Key()); ok {
		d.Focus = nf
		return true
	}
	return true
}

func (a *App) confirmCompareMerge() {
	d := a.model.CompareMergeDialog
	rows := a.compareCtrl.FilteredRows()
	in := comparepkg.MergeInput{
		PrimarySelected:   a.model.Primary.SelectedPaths,
		SecondarySelected: a.model.Secondary.SelectedPaths,
		Filter:            a.model.CompareView.Filter,
	}
	opts := comparepkg.MergeOptions{
		Direction:    d.Direction,
		CopyMissing:  d.CopyMissing,
		CopyModified: d.CopyModified,
		MoveMode:     d.MoveMode,
	}
	plan, err := comparepkg.BuildMergePlan(a.model.CompareSnapshot, rows, in, opts)
	if err != nil {
		a.setTransientMessage(err.Error(), ui.MessageUrgencyWarn)
		return
	}
	preserve := jobs.TransferPreserveFromConfig(
		a.config.Operations.PreservePermissions,
		a.config.Operations.PreserveTimestamps,
	)
	jobType := jobs.TypeCopy
	if d.MoveMode {
		jobType = jobs.TypeMove
	}
	for _, item := range plan.Copies {
		destDir := filepath.Dir(item.Dst)
		a.jobsCtrl.AddTransferJob(jobType, []string{item.Src}, destDir, false, preserve)
	}
	a.closeCompareMergeDialog()
	a.compareCtrl.Refresh()
	verb := "copies"
	if d.MoveMode {
		verb = "moves"
	}
	msg := fmt.Sprintf("Merge queued (%d %s)", len(plan.Copies), verb)
	a.setTransientMessage(msg, ui.MessageUrgencyInfo)
}
