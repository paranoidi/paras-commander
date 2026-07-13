//go:build linux

package subshell

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

const (
	// subshellChdirSyncTimeout bounds how long Chdir waits for the prompt hook to report
	// a fresh cycle before giving up and unmuting anyway.
	subshellChdirSyncTimeout = 2 * time.Second
	// subshellChdirQuietFallback is the fixed settle window used for shells with no
	// prompt hook (no per-prompt signal to wait on). ponytail: same tradeoff already
	// accepted by absorbPTYUntilQuiet at Start, now paid per hookless Chdir call too;
	// upgrade to a real barrier (MC's kill -STOP $$ + WUNTRACED) only if this is ever
	// observed to actually flicker.
	subshellChdirQuietFallback = 150 * time.Millisecond
	// subshellChdirSettleAfterSignal is an extra discard window kept after the cwdGen
	// signal fires: some shells (fish) can flush the injected line's own echo slightly
	// after their prompt hook already reported the new prompt cycle, so unmuting the
	// instant the signal arrives lets that straggling echo leak through. ponytail: at
	// the cost of occasionally also discarding the very next prompt redraw (cosmetic
	// only — the shell's actual cwd is already correct by then).
	subshellChdirSettleAfterSignal = 60 * time.Millisecond
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
	pgrp, err := unix.IoctlGetInt(s.ptyFD, unix.TIOCGPGRP)
	if err != nil {
		return false
	}
	return pgrp != s.cmd.Process.Pid
}

// Chdir injects a cd into the idle shell. Returns [ErrBusy] while a foreground command runs
// (the shell stays where it is; caller may retry on the next directory change). The injected
// command and its echo are muted from the embedded terminal panel's emulator (MC's QUIETLY
// feed mode, done here via [PanelFeed.Mute]/[PanelFeed.Unmute]) so it never reaches the screen.
func (s *Subshell) Chdir(dir string) error {
	if !s.Alive() {
		return ErrNotAlive
	}
	if s.Busy() {
		return ErrBusy
	}
	feed := s.panelFeed()
	if feed != nil {
		feed.Mute()
		defer feed.Unmute()
	}
	s.mu.Lock()
	beforeGen := s.cwdGen
	s.mu.Unlock()
	if _, err := s.WritePTY(chdirCommand(dir)); err != nil {
		return err
	}
	s.waitForPromptCycle(beforeGen)
	return nil
}

// waitForPromptCycle blocks until the prompt hook reports a fresh cycle (cwdGen bumps on
// every line, even an unchanged $PWD, so a failed cd's prompt still unblocks this promptly)
// or subshellChdirSyncTimeout elapses. Shells without a hook have no signal to wait on, so
// this just waits a fixed settle window instead.
func (s *Subshell) waitForPromptCycle(beforeGen int) {
	if !s.hasPrecmdHook {
		time.Sleep(subshellChdirQuietFallback)
		return
	}
	deadline := time.Now().Add(subshellChdirSyncTimeout)
	timer := time.AfterFunc(subshellChdirSyncTimeout, s.cwdCond.Broadcast)
	defer timer.Stop()
	s.mu.Lock()
	for s.cwdGen == beforeGen && time.Now().Before(deadline) {
		s.cwdCond.Wait()
	}
	s.mu.Unlock()
	time.Sleep(subshellChdirSettleAfterSignal)
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
