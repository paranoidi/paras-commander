package app

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
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
	a.model.ConflictDialog = ui.ConflictDialogState{
		Open:    true,
		JobID:   job.ID,
		Blocker: blocker,
		Focus:   0,
	}
}

func (a *App) closeJobBlockerDialog() {
	a.model.ConflictDialog = ui.ConflictDialogState{}
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
	a.pollJobEvents()
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
	a.stopJobBlockerNextTimer()
	delay := time.Duration(a.config.Jobs.BlockerDialogNextDebounceMS) * time.Millisecond
	gen := a.jobBlockerNextGen.Add(1)
	a.jobBlockerNextMu.Lock()
	a.jobBlockerNextTimer = time.AfterFunc(delay, func() {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(jobBlockerNextPayload{gen: gen}))
	})
	a.jobBlockerNextMu.Unlock()
}

func (a *App) stopJobBlockerNextTimer() {
	a.jobBlockerNextGen.Add(1)
	a.jobBlockerNextMu.Lock()
	defer a.jobBlockerNextMu.Unlock()
	if a.jobBlockerNextTimer == nil {
		return
	}
	if !a.jobBlockerNextTimer.Stop() {
		select {
		case <-a.jobBlockerNextTimer.C:
		default:
		}
	}
	a.jobBlockerNextTimer = nil
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
