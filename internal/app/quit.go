package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) handleQuit() bool {
	if a.hasActiveJobs() || a.hasRunningCommands() {
		a.openQuitConfirm()
		return false
	}
	return true
}

// handleQuitImmediate exits without prompting, stopping background jobs and command batches.
func (a *App) handleQuitImmediate() bool {
	if a.model.QuitConfirm.Open {
		a.model.QuitConfirm = ui.QuitConfirmState{}
	}
	a.stopWorker()
	return true
}

func (a *App) stopWorker() {
	if a.commandsCancel != nil {
		a.commandsCancel()
	}
	if !a.jobStopOnce {
		a.jobStopOnce = true
		close(a.jobStopCh)
	}
	if a.jobsCtrl != nil {
		a.jobsCtrl.StopWakeTimer()
	}
	a.stopSpinnerRedrawTimer()
	a.stopDiskUsageRedrawDebounce()
	a.invalidateIdleDiskSortBothPanels()
	if a.diskUsage != nil {
		a.diskUsage.Abort()
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
	st := ui.QuitConfirmState{Open: true, Focus: 0}
	hasJobs := a.hasActiveJobs()
	cmds := a.hasRunningCommands()
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
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 's', 'S':
			a.model.QuitConfirm = ui.QuitConfirmState{}
			return false
		case 'q', 'Q':
			a.model.QuitConfirm = ui.QuitConfirmState{}
			return true
		}
	}
	switch event.Key() {
	case tcell.KeyEsc:
		a.model.QuitConfirm = ui.QuitConfirmState{}
		return false
	case tcell.KeyLeft:
		a.model.QuitConfirm.Focus = ui.DialogPairLeftRight(a.model.QuitConfirm.Focus, false)
		return false
	case tcell.KeyRight:
		a.model.QuitConfirm.Focus = ui.DialogPairLeftRight(a.model.QuitConfirm.Focus, true)
		return false
	case tcell.KeyEnter:
		quitAnyway := a.model.QuitConfirm.Focus == 1
		a.model.QuitConfirm = ui.QuitConfirmState{}
		return quitAnyway
	}
	return false
}
