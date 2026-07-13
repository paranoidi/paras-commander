//go:build linux

package subshell

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// precmdInit returns the line typed into the shell right after start to hook its prompt:
// every prompt writes $PWD to child fd 3 (the cwd pipe — MC's subshell_pipe mechanism, see
// init_subshell_precmd in mc/src/subshell/common.c; we drop MC's `kill -STOP $$` because
// busy detection uses TIOCGPGRP instead). The reported pwd is logical, so panel sync keeps
// symlinked paths as typed. Empty for shells without a known hook — /proc fallback applies.
// Doubles as the "prompt cycle completed" signal for [Subshell.Chdir]: each line bumps
// Subshell.cwdGen and broadcasts cwdCond, which Chdir waits on to know when it's safe to
// stop muting the injected cd's echo from the terminal panel emulator.
func precmdInit(shell string) string {
	switch filepath.Base(shell) {
	case "bash":
		// ponytail: string form; MC also handles bash>=5.1 array PROMPT_COMMAND if it ever bites.
		return " PROMPT_COMMAND=${PROMPT_COMMAND:+$PROMPT_COMMAND;}'pwd >&3'\n"
	case "zsh":
		return " _pc_precmd() { pwd >&3; }; precmd_functions+=(_pc_precmd)\n"
	case "fish":
		return " functions -q fish_prompt_pc; or functions -c fish_prompt fish_prompt_pc; " +
			"function fish_prompt; echo \"$PWD\" >&3; fish_prompt_pc; end\n"
	default:
		return ""
	}
}

// absorbPTYUntilQuiet discards PTY output until it stays quiet for the given window (bounded
// by max) — used once at start so the precmd init line's echo never reaches the screen.
func absorbPTYUntilQuiet(ptmx *os.File, quiet, maxWait time.Duration) {
	fd := int(ptmx.Fd())
	// File.Fd() flips a pollable fd back to blocking mode — same gotcha as drainPTYToWriter.
	_ = unix.SetNonblock(fd, true)
	defer func() { _ = unix.SetNonblock(fd, false) }()
	buf := make([]byte, 4096)
	deadline := time.Now().Add(maxWait)
	last := time.Now()
	for time.Now().Before(deadline) && time.Since(last) < quiet {
		n, err := unix.Read(fd, buf)
		if n > 0 {
			last = time.Now()
			continue
		}
		if err != nil && !errors.Is(err, unix.EAGAIN) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
