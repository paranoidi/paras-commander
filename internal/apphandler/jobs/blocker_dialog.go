package jobs

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// JobBlockerNextPayload wakes the event loop to open the next quick-blocker dialog after the
// chain debounce elapses.
type JobBlockerNextPayload struct {
	gen uint64
}

func (h *Handler) tryOpenBlockerDialog() {
	if h.model.ConflictDialog.Open {
		return
	}
	if h.state.JobsWaitingDecision() == 0 {
		return
	}
	job := h.state.FirstWaitingBlockerJob()
	if job == nil || job.PendingBlocker == nil {
		return
	}
	blocker := *job.PendingBlocker
	h.model.ConflictDialog = dialog.ConflictDialogState{
		Open:    true,
		JobID:   job.ID,
		Blocker: blocker,
	}
}

func (h *Handler) closeBlockerDialog() {
	h.model.ConflictDialog = dialog.ConflictDialogState{}
	h.StopBlockerNextTimer()
}

func (h *Handler) postponeBlockerDialog() {
	h.closeBlockerDialog()
}

func (h *Handler) confirmBlockerDialog() {
	s := h.BlockerDialogScreen()
	h.activateBlockerChoice(dlg.Focused(s.Spec, *s.Focus))
}

// activateBlockerChoice runs the button id of the File exists / Disk space screen: Postpone
// closes the dialog, Advanced… opens the rules screen, every other button submits its decision.
func (h *Handler) activateBlockerChoice(id dlg.ID) {
	st := h.model.ConflictDialog
	if !st.Open {
		return
	}
	switch id {
	case dialog.ConflictPostpone:
		h.postponeBlockerDialog()
		return
	case dialog.ConflictAdvanced:
		h.model.ConflictDialog.StartAdvanced()
		return
	}
	d, ok := dialog.ConflictDecisionOf(id)
	if !ok {
		h.postponeBlockerDialog()
		return
	}
	jobID := st.JobID
	h.closeBlockerDialog()
	h.state.SubmitBlockerDecision(jobID, d)
	h.PollEvents()
	h.SetListStale(true)
	h.scheduleBlockerDialogChain()
}

// DialogOK confirms the open blocker dialog: the focused choice, or the rules in advanced mode.
func (h *Handler) DialogOK() {
	if !h.model.ConflictDialog.Open {
		return
	}
	if h.model.ConflictDialog.Advanced {
		h.applyAdvanced()
		return
	}
	h.confirmBlockerDialog()
}

// DialogCancel postpones the open blocker dialog, or leaves advanced mode.
func (h *Handler) DialogCancel() {
	if !h.model.ConflictDialog.Open {
		return
	}
	if h.model.ConflictDialog.Advanced {
		h.cancelAdvanced()
		return
	}
	h.postponeBlockerDialog()
}

// BlockerDialogScreen is the open conflict dialog's screen on top, the one the renderer draws.
func (h *Handler) BlockerDialogScreen() dialog.DialogScreen {
	return h.model.ConflictDialog.Screen(h.model.UserHomeDir)
}

// HandleBlockerDialogKey routes keys for the open quick-blocker (ConflictDialog) dialog: the
// engine owns focus, Tab and the button mnemonics. No-op when the dialog is not open.
func (h *Handler) HandleBlockerDialogKey(event *tcell.EventKey) {
	st := &h.model.ConflictDialog
	if !st.Open {
		return
	}
	s := h.BlockerDialogScreen()
	if ev, ok := dlg.HandleKey(s.Spec, s.Focus, event); ok {
		h.ApplyBlockerDialogEvent(ev)
	}
}

// ApplyBlockerDialogEvent acts on a widget event of the open conflict dialog (key or click): a
// choice on the quick screens, a rule, option or button on the Conflict rules screen.
func (h *Handler) ApplyBlockerDialogEvent(ev dlg.Event) {
	st := &h.model.ConflictDialog
	if ev.ID == "" {
		return
	}
	if st.Advanced && st.Blocker.Kind != jobs.BlockerKindDiskSpace {
		h.applyAdvancedEvent(ev)
		return
	}
	h.activateBlockerChoice(ev.ID)
}

// openAdvancedFromPanel opens the "Conflict rules" dialog for the job selected in the jobs-view
// conflict panel.
func (h *Handler) openAdvancedFromPanel(sel ui.JobEntry) {
	if sel.PendingBlocker == nil || h.model.ConflictDialog.Open {
		return
	}
	h.model.ConflictDialog = dialog.ConflictDialogState{
		Open:            true,
		JobID:           sel.ID,
		Blocker:         *sel.PendingBlocker,
		OpenedFromPanel: true,
	}
	h.model.ConflictDialog.StartAdvanced()
}

func (h *Handler) cancelAdvanced() {
	if h.model.ConflictDialog.OpenedFromPanel {
		h.model.ConflictDialog = dialog.ConflictDialogState{}
		return
	}
	h.model.ConflictDialog.Advanced = false
}

func (h *Handler) applyAdvanced() {
	st := h.model.ConflictDialog
	answer := st.ConflictAdvancedAnswer()
	h.model.ConflictDialog = dialog.ConflictDialogState{}
	h.state.SubmitBlockerAnswer(st.JobID, answer)
	h.PollEvents()
	h.SetListStale(true)
	if st.OpenedFromPanel {
		h.model.JobsView.ConflictButtonFocus = 0
		return
	}
	h.StopBlockerNextTimer()
	h.scheduleBlockerDialogChain()
}

// applyAdvancedEvent acts on the "Conflict rules" mode of the conflict dialog. Space, a click
// and the mnemonics toggle a checkbox and pick a radio; Enter on a radio picks it, Enter on a
// checkbox or OK applies the rules.
func (h *Handler) applyAdvancedEvent(ev dlg.Event) {
	st := &h.model.ConflictDialog
	switch ev.ID {
	case dlg.OK:
		h.applyAdvanced()
	case dlg.Cancel:
		h.cancelAdvanced()
	case dialog.ConflictVerify, dialog.ConflictApplyAll:
		switch {
		case ev.Enter:
			h.applyAdvanced()
		case ev.ID == dialog.ConflictVerify:
			st.ToggleVerify()
		default:
			st.AdvAll = !st.AdvAll
		}
	default:
		st.ApplyRule(ev.ID)
	}
}

func (h *Handler) scheduleBlockerDialogChain() {
	h.jobBlockerNextGen.Add(1)
	delay := time.Duration(h.config.Jobs.BlockerDialogNextDebounceMS) * time.Millisecond
	gen := h.jobBlockerNextGen.Add(1)
	h.jobBlockerNext.Arm(delay, func() {
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(JobBlockerNextPayload{gen: gen}))
	})
}

// StopBlockerNextTimer cancels any pending quick-blocker chain timer.
func (h *Handler) StopBlockerNextTimer() {
	h.jobBlockerNextGen.Add(1)
	h.jobBlockerNext.Stop()
}

// ApplyBlockerNextPayload opens the next quick blocker dialog for a chain wake, ignoring stale
// generations. Returns true when the UI should repaint.
func (h *Handler) ApplyBlockerNextPayload(p JobBlockerNextPayload) bool {
	if p.gen != h.jobBlockerNextGen.Load() {
		return false
	}
	if h.model.ConflictDialog.Open {
		return false
	}
	if h.state.JobsWaitingDecision() == 0 {
		return false
	}
	h.tryOpenBlockerDialog()
	return h.model.ConflictDialog.Open
}

// HandleAnswerBlockerKey opens the quick-blocker dialog for the oldest job awaiting a decision.
// Returns true when the UI should repaint (dialog opened).
func (h *Handler) HandleAnswerBlockerKey() (rendered bool) {
	if h.model.ConflictDialog.Open {
		return false
	}
	if h.state.JobsWaitingDecision() > 0 {
		h.tryOpenBlockerDialog()
		return h.model.ConflictDialog.Open
	}
	return false
}
