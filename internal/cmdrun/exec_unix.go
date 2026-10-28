//go:build !windows

package cmdrun

import (
	"os"
	"os/exec"
	"syscall"
)

// detachProcessGroup starts cmd in a new session so the child has no controlling terminal.
// Without this, interactive shells (e.g. bash -i) call tcsetpgrp() to grab the terminal and
// receive SIGTTOU, which suspends the child and sends the app to the background. Setsid also
// makes the child the process-group leader (pgid == pid); CommandContext's default Cancel only
// SIGKILLs that one pid, so a shell that already forked would leave descendants holding the
// pipes. Kill the group on ctx cancel (quit/rerun) so the wait cannot stall on an orphaned
// grandchild.
func detachProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := SignalGroup(cmd.Process, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
}

// SignalGroup sends sig to the process group led by p (started via detachProcessGroup), so
// anything it forked receives the signal too.
func SignalGroup(p *os.Process, sig syscall.Signal) error {
	return syscall.Kill(-p.Pid, sig)
}
