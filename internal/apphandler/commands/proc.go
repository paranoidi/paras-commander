package commands

import (
	"os"
	"syscall"
)

// procHandle lets terminateSelectedCommand/killSelectedCommand reach a running Commands-view
// row's subprocess by row index (model.CommandsList index).
type procHandle struct {
	proc *os.Process
}

// SetProcess records the *os.Process for a Commands-view row once its subprocess starts, via
// cmdrun.RunTracked's onStart callback.
func (h *Handler) SetProcess(idx int, p *os.Process) {
	h.procsMu.Lock()
	if h.procs == nil {
		h.procs = make(map[int]*procHandle)
	}
	h.procs[idx] = &procHandle{proc: p}
	h.procsMu.Unlock()
}

// UnregisterProc drops tracking for a row once its subprocess has finished.
func (h *Handler) UnregisterProc(idx int) {
	h.procsMu.Lock()
	delete(h.procs, idx)
	h.procsMu.Unlock()
}

// signalCommandRow sends sig to the row's running subprocess. cmdrun sets Setsid, making the
// subprocess its own process-group leader, so signaling the negative pid also reaches anything
// it forked (e.g. a shell script's commands) — signaling only the top pid would leave those
// running. Returns false if the row has no tracked (started) subprocess.
func (h *Handler) signalCommandRow(idx int, sig syscall.Signal) bool {
	h.procsMu.Lock()
	handle, ok := h.procs[idx]
	h.procsMu.Unlock()
	if !ok || handle.proc == nil {
		return false
	}
	return syscall.Kill(-handle.proc.Pid, sig) == nil
}
