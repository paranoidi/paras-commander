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
// stop muting the injected cd's echo from the terminal panel emulator. It also binds
// [chdirStashLine]/[chdirRestoreLine] to widgets that park and restore the pending command line.
func precmdInit(shell string) string {
	switch filepath.Base(shell) {
	case "bash":
		// ponytail: string form; MC also handles bash>=5.1 array PROMPT_COMMAND if it ever bites.
		return " PROMPT_COMMAND=${PROMPT_COMMAND:+$PROMPT_COMMAND;}'pwd >&3'; " +
			"_pc_stash() { _pc_line=$READLINE_LINE _pc_point=$READLINE_POINT READLINE_LINE= READLINE_POINT=0; }; " +
			"_pc_restore() { READLINE_LINE=$_pc_line READLINE_POINT=$_pc_point _pc_line=; }; " +
			"for _pc_m in emacs vi-insert; do bind -m $_pc_m -x '\"\\e[9001~\": _pc_stash'; " +
			"bind -m $_pc_m -x '\"\\e[9002~\": _pc_restore'; done\n"
	case "zsh":
		return " _pc_precmd() { pwd >&3; }; precmd_functions+=(_pc_precmd); " +
			"_pc_stash() { _pc_line=$BUFFER _pc_cursor=$CURSOR; BUFFER=; }; " +
			"_pc_restore() { BUFFER=$_pc_line; CURSOR=$_pc_cursor; _pc_line=; }; " +
			"zle -N _pc_stash; zle -N _pc_restore; for _pc_m in emacs viins; do " +
			"bindkey -M $_pc_m '\\e[9001~' _pc_stash; bindkey -M $_pc_m '\\e[9002~' _pc_restore; done\n"
	case "fish":
		return " functions -q fish_prompt_pc; or functions -c fish_prompt fish_prompt_pc; " +
			"function fish_prompt; echo \"$PWD\" >&3; fish_prompt_pc; end; " +
			"function _pc_stash; set -g _pc_line (commandline | string collect); " +
			"set -g _pc_cursor (commandline -C); commandline ''; end; " +
			"function _pc_restore; commandline -- \"$_pc_line\"; commandline -C $_pc_cursor; set -e _pc_line; end; " +
			"for _pc_m in default insert; bind -M $_pc_m \\e\\[9001~ _pc_stash; bind -M $_pc_m \\e\\[9002~ _pc_restore; end\n"
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
