//go:build linux

package preview

import (
	"context"
	"errors"
	"os/exec"

	"github.com/creack/pty"
	"golang.org/x/term"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
)

// runRuleCommandCapture runs argv with a real PTY attached to its stdin/stdout/stderr instead
// of cmdrun.Run's plain pipes, and answers a small set of terminal capability queries
// (terminalQueryScanner) on its behalf. Some preview tools (observed: movie-info) probe the
// terminal with DA1 then CPR before deciding whether to draw a Sixel/Kitty image; against a
// plain pipe (stdin /dev/null, stdout unconnected to any terminal) nothing ever answers those
// queries and the tool silently falls back to text — this gives it a real, if minimal,
// terminal to talk to instead. A PTY has one combined stream, so Stderr is always empty in the
// result; capture is capped at maxBytes the same way cmdrun.Run caps its pipes (tail bytes kept,
// StdoutTrim set).
func runRuleCommandCapture(ctx context.Context, argv []string, dir string, maxBytes int, sixelOK bool) cmdrun.RunResult {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return cmdrun.RunResult{LaunchErr: err, ExitCode: -1}
	}
	defer func() { _ = ptmx.Close() }()
	// A fresh pty starts in canonical (line-buffered, echoing) mode: a byte-oriented read by
	// the child (e.g. dd reading our DA1 reply) would never see it until a newline shows up,
	// and echo would mirror our synthetic replies back into the very stream we're capturing.
	// Raw mode fixes both. Master and slave share one termios, so setting it via the master
	// applies to the child's end too; the returned prior state is never restored since this
	// pty is discarded (not reused) once the command exits.
	_, _ = term.MakeRaw(int(ptmx.Fd()))

	scanner := &terminalQueryScanner{sixelOK: sixelOK}
	var out []byte
	var trimmed bool
	appendCapped := func(b []byte) {
		if len(b) == 0 {
			return
		}
		out = append(out, b...)
		if len(out) > maxBytes {
			trimmed = true
			out = out[len(out)-maxBytes:]
		}
	}

	buf := make([]byte, 32*1024)
	for {
		n, rErr := ptmx.Read(buf)
		if n > 0 {
			clean, reply := scanner.Scan(buf[:n])
			if len(reply) > 0 {
				_, _ = ptmx.Write(reply)
			}
			appendCapped(clean)
		}
		if rErr != nil {
			// Reading a pty master after the child (sole slave holder) exits and closes it
			// returns EIO on Linux, not io.EOF — either way, no more input is coming.
			break
		}
	}
	appendCapped(scanner.Flush())

	waitErr := cmd.Wait()
	res := cmdrun.RunResult{Stdout: out, StdoutTrim: trimmed, ExitCode: -1}
	var exitErr *exec.ExitError
	switch {
	case waitErr == nil:
		res.ExitCode = 0
	case errors.As(waitErr, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		res.LaunchErr = waitErr
	}
	return res
}
