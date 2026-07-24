package app

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// jobBlockerNextPayload opens the next quick blocker dialog after debounce elapses.
type jobBlockerNextPayload struct {
	gen uint64
}

func (a *App) tryOpenJobBlockerDialog() {
	if a.model.ConflictDialog.Open {
		return
	}
	if a.jobState.JobsWaitingDecision() == 0 {
		return
	}
	job := a.jobState.FirstWaitingBlockerJob()
	if job == nil || job.PendingBlocker == nil {
		return
	}
	blocker := *job.PendingBlocker
	a.model.ConflictDialog = dialog.ConflictDialogState{
		Open:    true,
		JobID:   job.ID,
		Blocker: blocker,
		Focus:   0,
	}
}

func (a *App) closeJobBlockerDialog() {
	a.model.ConflictDialog = dialog.ConflictDialogState{}
	a.stopJobBlockerNextTimer()
}

func (a *App) postponeJobBlockerDialog() {
	a.closeJobBlockerDialog()
}

func (a *App) confirmJobBlockerDialog() {
	a.confirmJobBlockerDialogWithFocus(a.model.ConflictDialog.Focus)
}

func (a *App) confirmJobBlockerDialogWithFocus(focus int) {
	st := a.model.ConflictDialog
	if !st.Open {
		return
	}
	d, ok := ui.JobBlockerDialogDecision(st.Blocker, focus)
	if !ok {
		a.postponeJobBlockerDialog()
		return
	}
	jobID := st.JobID
	a.closeJobBlockerDialog()
	a.jobState.SubmitBlockerDecision(jobID, d)
	a.jobsCtrl.PollEvents()
	a.jobsCtrl.SetListStale(true)
	a.scheduleJobBlockerDialogChain()
}

func (a *App) handleConflictDialogKey(event *tcell.EventKey) {
	st := &a.model.ConflictDialog
	if !st.Open {
		return
	}

	if event.Key() == tcell.KeyEsc {
		a.postponeJobBlockerDialog()
		return
	}

	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if focus, ok := ui.JobBlockerDialogFocusFromShortcut(st.Blocker, event.Rune()); ok {
			if ui.JobBlockerDialogIsPostpone(st.Blocker, focus) {
				a.postponeJobBlockerDialog()
				return
			}
			a.confirmJobBlockerDialogWithFocus(focus)
			return
		}
	}

	if newFocus, handled := ui.JobBlockerDialogMoveFocus(st.Blocker, st.Focus, event.Key()); handled {
		st.Focus = newFocus
		return
	}

	if event.Key() == tcell.KeyEnter {
		if ui.JobBlockerDialogIsPostpone(st.Blocker, st.Focus) {
			a.postponeJobBlockerDialog()
			return
		}
		a.confirmJobBlockerDialog()
	}
}

func (a *App) scheduleJobBlockerDialogChain() {
	a.jobBlockerNextGen.Add(1)
	delay := time.Duration(a.config.Jobs.BlockerDialogNextDebounceMS) * time.Millisecond
	gen := a.jobBlockerNextGen.Add(1)
	a.jobBlockerNext.Reset(delay, func() {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(jobBlockerNextPayload{gen: gen}))
	})
}

func (a *App) stopJobBlockerNextTimer() {
	a.jobBlockerNextGen.Add(1)
	a.jobBlockerNext.Clear()
}

func (a *App) applyJobBlockerNextPayload(p jobBlockerNextPayload) bool {
	if p.gen != a.jobBlockerNextGen.Load() {
		return false
	}
	if a.model.ConflictDialog.Open {
		return false
	}
	if a.jobState.JobsWaitingDecision() == 0 {
		return false
	}
	a.tryOpenJobBlockerDialog()
	return a.model.ConflictDialog.Open
}

func (a *App) tryDispatchJobs(actionID string) bool {
	if actionID == keymap.ActionJobsAnswerBlocker {
		// The raw-key path answers blockers pre-dispatch (input.go); this covers
		// menu/help activation. No-op when nothing is waiting for a decision.
		a.handleJobsAnswerBlockerKey()
		return true
	}
	return a.jobsCtrl.TryDispatch(actionID)
}

func (a *App) handleJobsAnswerBlockerKey() (rendered bool) {
	if a.model.ConflictDialog.Open {
		return false
	}
	if a.jobState.JobsWaitingDecision() > 0 {
		a.tryOpenJobBlockerDialog()
		return a.model.ConflictDialog.Open
	}
	return false
}
