package dialog

import (
	"fmt"

	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// AddTransferJob enqueues a copy/move job through Deps.Jobs, then optionally arms
// focus-other-panel-after-transfer when the destination is the inactive panel's cwd.
func (h *Handler) AddTransferJob(jobType jobs.Type, sources []string, dest string, startPaused bool, preserve jobs.TransferPreserve) {
	h.jobs.AddTransferJob(jobsctrl.TransferJobRequest{
		Type: jobType, Sources: sources, Dest: dest, StartPaused: startPaused, Preserve: preserve,
	})
	h.maybeScheduleTransferOtherPanelFocus(jobType, sources, dest, preserve)
}

// TransferPreserveFromConfig reads the live copy-preserve settings (permissions/timestamps)
// from Host.Config(), not a construction-time snapshot: the settings dialog mutates them at runtime.
func (h *Handler) TransferPreserveFromConfig() jobs.TransferPreserve {
	cfg := h.host.Config()
	return jobs.TransferPreserveFromConfig(cfg.Operations.PreservePermissions, cfg.Operations.PreserveTimestamps)
}

// activateDuplicateAction copies the highlighted file or directory beside itself under a new name.
func (h *Handler) activateDuplicateAction() {
	h.openDuplicateDialog()
}

func (h *Handler) openDuplicateDialog() {
	p := h.host.ActivePanel()
	entry, err := ops.ValidateDuplicateSource(p)
	if err != nil {
		h.host.SetErrorMessage("Duplicate", err)
		return
	}
	name := entry.Name
	nameRunes := len([]rune(name))
	fields := []dialog.FileDialogField{
		{Label: "Name", Value: name, Prefill: name, Cursor: nameRunes, PrefillPending: true},
	}
	h.model.FileDialog = dialog.FileDialogState{
		Open:             true,
		DialogType:       dialog.FileDialogDuplicate,
		Fields:           fields,
		DuplicateSource:  entry.Path,
		RenamePhase:      dialog.RenamePhaseMain,
		RenameSlugifySep: dialog.RenameSlugifyDot,
		RenameFocusAfter: h.host.Config().Operations.RenameFocusAfter,
	}
}

func (h *Handler) executeDuplicate() {
	p := h.host.ActivePanel()
	d := &h.model.FileDialog
	if len(d.Fields) == 0 {
		h.CloseFileDialog()
		return
	}
	newName := d.Fields[0].Value
	sourcePath := d.DuplicateSource
	if sourcePath == "" {
		h.CloseFileDialog()
		return
	}
	entry, err := ops.ValidateDuplicateSource(p)
	if err != nil {
		h.host.SetErrorMessage("Duplicate source", err)
		h.CloseFileDialog()
		return
	}
	if entry.Path != sourcePath {
		h.host.SetErrorMessage("Duplicate", ops.SourceError("source changed while dialog was open"))
		h.CloseFileDialog()
		return
	}
	plan, err := ops.PlanDuplicate(entry, newName, p.PathString())
	if err != nil {
		h.host.SetErrorMessage("Duplicate", err)
		h.CloseFileDialog()
		return
	}
	focusAfter := d.RenameFocusAfter
	listDir := p.PathString()
	panelID := h.model.ActivePanel
	h.CloseFileDialog()
	h.AddTransferJob(jobs.TypeCopy, []string{plan.SourcePath}, plan.DestPath, false, h.TransferPreserveFromConfig())
	if focusAfter {
		// The copy is only queued above, not yet on disk, so it can't be tied to a specific
		// reload (unlike rename/mkdir's RefreshBothPanelsWithFocus); this polls instead, via
		// ReconcilePendingPanelFocus, until the job's terminal event eventually refreshes the panel.
		h.schedulePanelFocus(panelID, listDir, plan.NewName)
	}
	h.RefreshBothPanels()
	h.host.SetTransientMessage(fmt.Sprintf("Copy queued as %s", plan.NewName), ui.MessageUrgencyInfo)
}
