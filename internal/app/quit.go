package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func (a *App) handleQuit() bool {
	if a.chooserMode() {
		return a.handleQuitImmediate()
	}
	if a.hasActiveJobs() || a.commandsCtrl.HasRunning() {
		a.openQuitConfirm()
		return false
	}
	return true
}

// handleQuitImmediate exits without prompting, stopping background jobs and command batches.
func (a *App) handleQuitImmediate() bool {
	if a.model.QuitConfirm.Open {
		a.model.QuitConfirm = dialog.QuitConfirmState{}
	}
	a.stopWorker()
	return true
}

func (a *App) stopWorker() {
	if a.commandsCancel != nil {
		a.commandsCancel()
	}
	if a.metaCtrl != nil {
		a.metaCtrl.CancelAll()
	}
	if !a.jobStopOnce {
		a.jobStopOnce = true
		close(a.jobStopCh)
	}
	if a.statusCmdStopCh != nil {
		close(a.statusCmdStopCh)
		a.statusCmdStopCh = nil
	}
	if a.jobsCtrl != nil {
		a.jobsCtrl.StopWakeTimer()
		a.jobsCtrl.StopBlockerNextTimer()
	}
	a.stopSpinnerRedrawTimer()
	a.stopDiskUsageRedrawDebounce()
	a.invalidateIdleDiskSortBothPanels()
	a.invalidateMetaIdleSortBothPanels()
	if a.disk.engine != nil {
		a.disk.engine.Abort()
	}
}

func (a *App) hasActiveJobs() bool {
	for _, j := range a.jobState.AllJobs() {
		if j.Status == jobs.StatusScanning || j.Status == jobs.StatusQueued || j.Status == jobs.StatusPaused || j.Status == jobs.StatusRunning || j.Status == jobs.StatusWaitingDecision {
			return true
		}
	}
	return false
}

func (a *App) openQuitConfirm() {
	st := dialog.QuitConfirmState{Open: true, Focus: dialog.QuitStay}
	hasJobs := a.hasActiveJobs()
	cmds := a.commandsCtrl.HasRunning()
	switch {
	case hasJobs && cmds:
		st.WarnLine1 = "Active jobs or commands are running."
		st.WarnLine2 = "Quitting will interrupt background work."
	case cmds && !hasJobs:
		st.WarnLine1 = "Commands are still running."
		st.WarnLine2 = "Quitting will cancel running subprocesses."
	}
	a.model.QuitConfirm = st
	a.render()
}

func (a *App) handleQuitConfirmKey(event *tcell.EventKey) bool {
	st := &a.model.QuitConfirm
	if ev, ok := dlg.HandleKey(dialog.QuitConfirmView(*st), &st.Focus, event); ok {
		return a.applyQuitConfirmEvent(ev)
	}
	return false
}

// applyQuitConfirmEvent answers the quit confirmation (key or click); reports whether to quit.
func (a *App) applyQuitConfirmEvent(ev dlg.Event) bool {
	if ev.ID == "" {
		return false
	}
	a.model.QuitConfirm = dialog.QuitConfirmState{}
	return ev.ID == dialog.QuitQuit
}
