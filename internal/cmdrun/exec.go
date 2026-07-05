package cmdrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
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
	return RunTracked(ctx, argv, dir, maxStreamBytes, nil)
}

// RunTracked behaves like Run but, once the subprocess starts, calls onStart with its
// *os.Process so a caller can send it signals (e.g. terminate/kill) independently of ctx
// cancellation. onStart may be nil.
func RunTracked(ctx context.Context, argv []string, dir string, maxStreamBytes int, onStart func(*os.Process)) RunResult {
	if len(argv) == 0 {
		return RunResult{LaunchErr: errors.New("empty argv"), ExitCode: -1}
	}
	if maxStreamBytes <= 0 {
		maxStreamBytes = MaxStreamBytes
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	// Allow orphaned child processes (e.g. clipboard tools spawned by shell wrappers)
	// up to 5 seconds to flush output after the primary process exits, then forcibly
	// close the pipes so cmd.Wait() doesn't block indefinitely.
	cmd.WaitDelay = 5 * time.Second
	// New session so the child has no controlling terminal. Without this, interactive
	// shells (e.g. bash -i) call tcsetpgrp() to grab the terminal and receive SIGTTOU,
	// which suspends the child and sends the app to the background.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	var stdoutBuf, stderrBuf cappedWriter
	stdoutBuf.max = maxStreamBytes
	stderrBuf.max = maxStreamBytes
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return RunResult{LaunchErr: err, ExitCode: -1}
	}
	if onStart != nil {
		onStart(cmd.Process)
	}
	err := cmd.Wait()
	out := RunResult{
		Stdout:     stdoutBuf.Bytes(),
		Stderr:     stripJobControlNoise(stderrBuf.Bytes()),
		StdoutTrim: stdoutBuf.trimmed,
		StderrTrim: stderrBuf.trimmed,
		ExitCode:   -1,
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

// jobControlNoise are substrings of the harmless startup warnings a job-control
// shell (e.g. bash -i) emits when Setsid puts it in a new session with no
// controlling terminal. tcsetpgrp() then fails with pgrp -1. This is an expected
// artifact of the session we deliberately create in RunTracked, never a real
// failure, so we drop those lines from captured stderr — otherwise callers that
// surface any stderr (e.g. the user-menu background notifier) toast them as errors.
var jobControlNoise = []string{
	"cannot set terminal process group",
	"no job control in this shell",
}

func stripJobControlNoise(b []byte) []byte {
	needsFilter := false
	for _, m := range jobControlNoise {
		if bytes.Contains(b, []byte(m)) {
			needsFilter = true
			break
		}
	}
	if !needsFilter {
		return b
	}
	lines := bytes.Split(b, []byte("\n"))
	kept := lines[:0]
	for _, ln := range lines {
		if isJobControlNoise(ln) {
			continue
		}
		kept = append(kept, ln)
	}
	return bytes.Join(kept, []byte("\n"))
}

func isJobControlNoise(line []byte) bool {
	for _, m := range jobControlNoise {
		if bytes.Contains(line, []byte(m)) {
			return true
		}
	}
	return false
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

// RunInteractive runs argv with stdin/stdout/stderr attached to the terminal.
func RunInteractive(ctx context.Context, argv []string, dir string) error {
	if len(argv) == 0 {
		return errors.New("empty argv")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if dir != "" {
		cmd.Dir = dir
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run command: %w", err)
	}
	return nil
}

// StartDetached starts argv in the background and releases the process handle.
func StartDetached(argv []string, dir string) error {
	if len(argv) == 0 {
		return errors.New("empty argv")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	if dir != "" {
		cmd.Dir = dir
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release process: %w", err)
	}
	return nil
}
