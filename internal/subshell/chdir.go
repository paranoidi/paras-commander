package subshell

import (
	"strconv"
	"strings"
)

// QuoteArg single-quotes s for injection into the shell command line. POSIX shells and fish
// both parse this form ('\” ends the quote, escapes one quote, reopens).
// ponytail: one dialect for all shells; per-shell quoting tables if an exotic path ever misparses.
func QuoteArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// chdirAckPrefix starts the line an injected cd writes to the cwd pipe once it has run. Prompt
// hook lines are absolute paths, so the prefix never collides with a reported $PWD.
const chdirAckPrefix = "pc-chdir-ack "

// chdirCommand is the line written to the PTY by [Subshell.Chdir]. The leading space keeps the
// injected cd out of bash/zsh/fish history. A non-zero ack appends a write of
// chdirAckPrefix+ack to the cwd pipe (fd 3), so Chdir can tell its own prompt cycle from a
// stale prompt report still in flight; it runs whether or not the cd succeeds.
func chdirCommand(dir string, ack int) []byte {
	line := " cd " + QuoteArg(dir)
	if ack > 0 {
		line += "; echo " + chdirAckPrefix + strconv.Itoa(ack) + " >&3"
	}
	return []byte(line + "\n")
}

// Private key sequences bound by [precmdInit] to stash/restore the line editor's pending
// input, so [Subshell.Chdir] never appends its cd to a half-typed command (and runs both).
// Real terminals never send these CSIs.
const (
	chdirStashLine   = "\x1b[9001~"
	chdirRestoreLine = "\x1b[9002~"
	// chdirDiscardLine is the tty VKILL for hookless shells (sh/dash, canonical mode): no
	// binding to restore from, so pending input is dropped rather than executed with the cd.
	chdirDiscardLine = "\x15"
)
