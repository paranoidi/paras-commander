package cmdrun

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

const truncationMarker = "\n\n[output truncated]\n"

// MaxStreamBytes is the default cap per stdout/stderr capture.
const MaxStreamBytes = 512 * 1024

// RunResult holds captured output and process outcome.
type RunResult struct {
	Stdout     []byte
	Stderr     []byte
	ExitCode   int
	LaunchErr  error // failure before or instead of exit status (e.g. executable not found)
	StdoutTrim bool
	StderrTrim bool
}

// Run executes argv as a subprocess with working directory dir. argv must be non-empty.
// Each stream is capped at maxStreamBytes; when exceeded, tail bytes are kept and StdoutTrim/StderrTrim are set.
func Run(ctx context.Context, argv []string, dir string, maxStreamBytes int) RunResult {
	if len(argv) == 0 {
		return RunResult{LaunchErr: errors.New("empty argv"), ExitCode: -1}
	}
	if maxStreamBytes <= 0 {
		maxStreamBytes = MaxStreamBytes
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir

	var stdoutBuf, stderrBuf cappedWriter
	stdoutBuf.max = maxStreamBytes
	stderrBuf.max = maxStreamBytes
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	out := RunResult{
		Stdout:    stdoutBuf.Bytes(),
		Stderr:    stderrBuf.Bytes(),
		StdoutTrim: stdoutBuf.trimmed,
		StderrTrim: stderrBuf.trimmed,
		ExitCode:  -1,
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			out.ExitCode = exitErr.ExitCode()
			return out
		}
		out.LaunchErr = err
		return out
	}
	out.ExitCode = 0
	return out
}

type cappedWriter struct {
	data    []byte
	max     int
	trimmed bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.data = append(w.data, p...)
	if len(w.data) > w.max {
		w.trimmed = true
		w.data = w.data[len(w.data)-w.max:]
	}
	return len(p), nil
}

func (w *cappedWriter) Bytes() []byte {
	b := w.data
	if !w.trimmed {
		return append([]byte(nil), b...)
	}
	suffix := truncationMarker
	if len(b)+len(suffix) > w.max {
		keep := w.max - len(suffix)
		if keep < 0 {
			keep = 0
		}
		out := append([]byte(nil), b[len(b)-keep:]...)
		return append(out, suffix...)
	}
	return append(append([]byte(nil), b...), suffix...)
}

// FormatArgvDisplay returns a compact human string for list titles (not shell-safe).
func FormatArgvDisplay(argv []string) string {
	return strings.Join(argv, " ")
}
