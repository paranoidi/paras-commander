package commands

import (
	"os"
	"syscall"
	"time"
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
//
// A row's Phase flips to Running before its goroutine reaches cmd.Start()/onStart (queued rows
// show progress before a process exists), so a Kill/Terminate pressed right after Running appears
// can race SetProcess. Retry briefly instead of no-opping — the goroutine registers the handle
// within milliseconds of Running being visible.
func (h *Handler) signalCommandRow(idx int, sig syscall.Signal) bool {
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		h.procsMu.Lock()
		handle, ok := h.procs[idx]
		h.procsMu.Unlock()
		if ok && handle.proc != nil {
			return syscall.Kill(-handle.proc.Pid, sig) == nil
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(2 * time.Millisecond)
	}
}
