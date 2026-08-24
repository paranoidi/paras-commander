package jobs

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
		Focus:   0,
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
	h.confirmBlockerDialogWithFocus(h.model.ConflictDialog.Focus)
}

func (h *Handler) confirmBlockerDialogWithFocus(focus int) {
	st := h.model.ConflictDialog
	if !st.Open {
		return
	}
	d, ok := ui.JobBlockerDialogDecision(st.Blocker, focus)
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

// HandleBlockerDialogKey routes keys for the open quick-blocker (ConflictDialog) dialog.
// No-op when the dialog is not open.
func (h *Handler) HandleBlockerDialogKey(event *tcell.EventKey) {
	st := &h.model.ConflictDialog
	if !st.Open {
		return
	}

	if event.Key() == tcell.KeyEsc {
		h.postponeBlockerDialog()
		return
	}

	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if focus, ok := ui.JobBlockerDialogFocusFromShortcut(st.Blocker, event.Rune()); ok {
			if ui.JobBlockerDialogIsPostpone(st.Blocker, focus) {
				h.postponeBlockerDialog()
				return
			}
			h.confirmBlockerDialogWithFocus(focus)
			return
		}
	}

	if newFocus, handled := ui.JobBlockerDialogMoveFocus(st.Blocker, st.Focus, event.Key()); handled {
		st.Focus = newFocus
		return
	}

	if event.Key() == tcell.KeyEnter {
		if ui.JobBlockerDialogIsPostpone(st.Blocker, st.Focus) {
			h.postponeBlockerDialog()
			return
		}
		h.confirmBlockerDialog()
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
