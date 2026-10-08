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
	if ui.JobBlockerDialogIsAdvanced(st.Blocker, focus) {
		h.model.ConflictDialog.StartAdvanced()
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

	if st.Advanced {
		h.handleAdvancedKey(event)
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

// handleAdvancedKey routes keys for the "Conflict rules" mode of the conflict dialog.
func (h *Handler) handleAdvancedKey(event *tcell.EventKey) {
	st := &h.model.ConflictDialog
	form := st.ConflictAdvancedForm()
	skipFocus, allFocus := st.ConflictAdvancedSkipIdenticalFocus(), st.ConflictAdvancedAllFocus()

	if event.Key() == tcell.KeyEsc {
		h.cancelAdvanced()
		return
	}
	extras := []dialog.ExtraMnemonic{{Rune: 'a', Fn: func() { st.AdvAll = !st.AdvAll }}}
	if skipFocus >= 0 {
		extras = append(extras, dialog.ExtraMnemonic{Rune: 'i', Fn: func() { st.Rules.SkipIdentical = !st.Rules.SkipIdentical }})
	}
	if dialog.TryStandardDialogActions(event, h.applyAdvanced, h.cancelAdvanced, extras) {
		return
	}
	// Left/Right move between the choices of a rule row (not only buttons, like the config dialog's
	// horizontal inputs).
	if st.AdvFocus < dialog.ConflictRuleRows && (event.Key() == tcell.KeyLeft || event.Key() == tcell.KeyRight) {
		if event.Key() == tcell.KeyLeft {
			st.AdvMoveChoice(-1)
		} else {
			st.AdvMoveChoice(1)
		}
		return
	}
	if newFocus, handled := form.MoveFocus(st.AdvFocus, event.Key()); handled {
		st.AdvFocus = newFocus
		st.SyncAdvCol()
		return
	}
	space := event.Key() == tcell.KeyRune && event.Rune() == ' '
	switch {
	case (space || event.Key() == tcell.KeyEnter) && st.AdvFocus < dialog.ConflictRuleRows:
		st.AdvSelectChoice()
	case space && st.AdvFocus == skipFocus:
		st.Rules.SkipIdentical = !st.Rules.SkipIdentical
	case space && st.AdvFocus == allFocus:
		st.AdvAll = !st.AdvAll
	case event.Key() == tcell.KeyEnter:
		if st.AdvFocus == form.CancelIndex() {
			h.cancelAdvanced()
			return
		}
		h.applyAdvanced()
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
