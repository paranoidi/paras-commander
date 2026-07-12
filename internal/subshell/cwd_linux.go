//go:build linux

package subshell

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Cwd returns the shell child's current working directory: the logical $PWD reported by the
// precmd pipe hook when available (symlinked paths preserved), else the /proc readlink
// fallback (shells without a precmd hook, or before the first prompt fired).
func (s *Subshell) Cwd() (string, error) {
	if !s.Alive() {
		return "", ErrNotAlive
	}
	s.mu.Lock()
	cwd := s.cwdPipe
	s.mu.Unlock()
	if cwd != "" {
		return cwd, nil
	}
	return os.Readlink(fmt.Sprintf("/proc/%d/cwd", s.cmd.Process.Pid))
}

// Busy reports whether the shell is running a foreground command: the PTY's foreground
// process group differs from the shell's own (the shell is session leader, so pgid == pid).
func (s *Subshell) Busy() bool {
	if !s.Alive() {
		return false
	}
	pgrp, err := unix.IoctlGetInt(int(s.pty.Fd()), unix.TIOCGPGRP)
	if err != nil {
		return false
	}
	return pgrp != s.cmd.Process.Pid
}

// Chdir injects a cd into the idle shell. Returns [ErrBusy] while a foreground command runs
// (the shell stays where it is; caller may retry on the next directory change).
func (s *Subshell) Chdir(dir string) error {
	if !s.Alive() {
		return ErrNotAlive
	}
	if s.Busy() {
		return ErrBusy
	}
	_, err := s.WritePTY(chdirCommand(dir))
	return err
}

// InsertText writes text into the idle shell's input buffer without a newline — it stays on
// the command line for the user to complete. Returns [ErrBusy] while a foreground command runs.
func (s *Subshell) InsertText(text string) error {
	if !s.Alive() {
		return ErrNotAlive
	}
	if s.Busy() {
		return ErrBusy
	}
	_, err := s.WritePTY([]byte(text))
	return err
}
