package compare

import (
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
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
	in, opts := h.mergeInputs(*d)
	plan := comparepkg.PreviewMergePlan(h.model.CompareSnapshot, h.FilteredRows(), in, opts)
	d.PreviewText = formatMergePreviewText(d.MoveMode, plan)
}

// mergeInputs is the single place the dialog state, panel selections and per-row name
// choices become merge plan inputs (preview and confirm).
func (h *Handler) mergeInputs(d dialog.CompareMergeDialogState) (comparepkg.MergeInput, comparepkg.MergeOptions) {
	in := comparepkg.MergeInput{
		PrimarySelected:   h.model.Primary.SelectedPaths,
		SecondarySelected: h.model.Secondary.SelectedPaths,
		Filter:            h.model.CompareView.Filter,
	}
	opts := comparepkg.MergeOptions{
		Direction:          d.Direction,
		CopyMissing:        d.CopyMissing,
		CopyModified:       d.CopyModified,
		MoveMode:           d.MoveMode,
		RelocatedUseSource: d.RelocatedUseSource,
		RelocatedNames:     h.model.CompareView.RelocatedNames,
	}
	return in, opts
}

func formatMergePreviewText(moveMode bool, plan comparepkg.MergePlan) string {
	verb := "to copy"
	if moveMode {
		verb = "to move"
	}
	text := fmt.Sprintf("Preview: %d %s (%s)", len(plan.Copies), verb, ui.FormatSelectionByteSize(plan.TotalBytes))
	if n := len(plan.Renames); n > 0 {
		text += fmt.Sprintf(", %d to rename", n)
	}
	return text
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
	in, opts := h.mergeInputs(d)
	plan, err := comparepkg.BuildMergePlan(h.model.CompareSnapshot, h.FilteredRows(), in, opts)
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
		h.jobsCtrl.AddTransferJob(jobsctrl.TransferJobRequest{
			Type: jobType, Sources: []string{item.Src}, Dest: mergeCopyDest(item.Dst), Preserve: preserve,
		})
	}
	// A rename's Dest is the target file path (not a directory), so the move takes its name.
	for _, item := range plan.Renames {
		h.jobsCtrl.AddTransferJob(jobsctrl.TransferJobRequest{
			Type: jobs.TypeMove, Sources: []string{item.Src}, Dest: item.Dst, Preserve: preserve,
		})
	}
	h.closeMergeDialog()
	h.Refresh()
	verb := "copies"
	if d.MoveMode {
		verb = "moves"
	}
	msg := fmt.Sprintf("Merge queued (%d %s)", len(plan.Copies), verb)
	if n := len(plan.Renames); n > 0 {
		msg = fmt.Sprintf("Merge queued (%d %s, %d %s)", len(plan.Copies), verb, n, jobbridge.Plural(n, "rename", "renames"))
	}
	h.host.SetTransientMessage(msg, ui.MessageUrgencyInfo)
}

// mergeCopyDest is the job destination for a merge copy to the file path dst: dst itself, so
// the job writes that name and creates missing parent directories. When a directory already
// sits at dst it is the parent instead, so the file-vs-directory clash goes through the
// conflict prompt rather than landing inside that directory.
func mergeCopyDest(dst string) string {
	if loc, err := pathloc.Parse(dst); err == nil && ops.DestinationIsDirAtEnqueue(loc) {
		return filepath.Dir(dst)
	}
	return dst
}

// MergeDialogOK confirms the merge dialog.
func (h *Handler) MergeDialogOK() { h.confirmMerge() }

// MergeDialogCancel closes the merge dialog.
func (h *Handler) MergeDialogCancel() { h.closeMergeDialog() }

// ToggleRelocatedName sets the cursor row's merge name to the focused column's path, or back
// to the merge dialog's default when that column was already chosen. Relocated rows only.
func (h *Handler) ToggleRelocatedName() {
	st := &h.model.CompareView
	rows := h.FilteredRows()
	if st.Selected < 0 || st.Selected >= len(rows) {
		return
	}
	row := rows[st.Selected]
	if row.Kind != comparepkg.KindRelocated {
		h.host.SetTransientMessage("Name choice applies to relocated files only", ui.MessageUrgencyInfo)
		return
	}
	side := comparepkg.NamePrimary
	if st.FocusColumn == ui.CompareColumnSecondary {
		side = comparepkg.NameSecondary
	}
	key := row.Key()
	if st.RelocatedNames[key] == side {
		delete(st.RelocatedNames, key)
		return
	}
	if st.RelocatedNames == nil {
		st.RelocatedNames = make(map[comparepkg.RowKey]comparepkg.NameSide)
	}
	st.RelocatedNames[key] = side
}
