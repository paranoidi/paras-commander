package app

import (
	"os"
	"syscall"
)

// commandProcHandle lets commands.terminate/commands.kill reach a running Commands-view
// row's subprocess by row index (model.CommandsList index).
type commandProcHandle struct {
	proc *os.Process
}

// setCommandProcess records the *os.Process for a Commands-view row once its subprocess
// starts, via cmdrun.RunTracked's onStart callback.
func (a *App) setCommandProcess(idx int, p *os.Process) {
	a.commandProcsMu.Lock()
	if a.commandProcs == nil {
		a.commandProcs = make(map[int]*commandProcHandle)
	}
	a.commandProcs[idx] = &commandProcHandle{proc: p}
	a.commandProcsMu.Unlock()
}

// unregisterCommandProc drops tracking for a row once its subprocess has finished.
func (a *App) unregisterCommandProc(idx int) {
	a.commandProcsMu.Lock()
	delete(a.commandProcs, idx)
	a.commandProcsMu.Unlock()
}

// signalCommandRow sends sig to the row's running subprocess. cmdrun sets Setsid, making the
// subprocess its own process-group leader, so signaling the negative pid also reaches anything
// it forked (e.g. a shell script's commands) — signaling only the top pid would leave those
// running. Returns false if the row has no tracked (started) subprocess.
func (a *App) signalCommandRow(idx int, sig syscall.Signal) bool {
	a.commandProcsMu.Lock()
	h, ok := a.commandProcs[idx]
	a.commandProcsMu.Unlock()
	if !ok || h.proc == nil {
		return false
	}
	return syscall.Kill(-h.proc.Pid, sig) == nil
}
