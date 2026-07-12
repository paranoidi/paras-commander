package subshell

import "strings"

// QuoteArg single-quotes s for injection into the shell command line. POSIX shells and fish
// both parse this form ('\” ends the quote, escapes one quote, reopens).
// ponytail: one dialect for all shells; per-shell quoting tables if an exotic path ever misparses.
func QuoteArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// chdirCommand is the line written to the PTY by [Subshell.Chdir]. The leading space keeps the
// injected cd out of bash/zsh/fish history.
func chdirCommand(dir string) []byte {
	return []byte(" cd " + QuoteArg(dir) + "\n")
}
