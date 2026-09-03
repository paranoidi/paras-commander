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
	// Orphaned child processes (e.g. clipboard tools that fork into the background
	// to keep serving a selection) may inherit our stdout/stderr pipe and never
	// close it on their own — no amount of waiting makes them flush, since they're
	// designed to keep running. WaitDelay bounds how long cmd.Wait() blocks after
	// the primary process exits before forcibly closing the pipes; keep it short
	// so a normal, already-finished command doesn't sit around waiting on a daemon
	// that was never going to close the pipe.
	cmd.WaitDelay = 200 * time.Millisecond
	// New session so the child has no controlling terminal. Without this, interactive
	// shells (e.g. bash -i) call tcsetpgrp() to grab the terminal and receive SIGTTOU,
	// which suspends the child and sends the app to the background.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	var stdoutBuf, stderrBuf CappedWriter
	stdoutBuf.Max = maxStreamBytes
	stderrBuf.Max = maxStreamBytes
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
		StdoutTrim: stdoutBuf.Trimmed,
		StderrTrim: stderrBuf.Trimmed,
		ExitCode:   -1,
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			out.ExitCode = exitErr.ExitCode()
			return out
		}
		if errors.Is(err, exec.ErrWaitDelay) {
			// The primary process already exited (ProcessState is set before
			// WaitDelay is even considered); only an orphaned grandchild held
			// the output pipes open past WaitDelay. Trust the real exit status
			// instead of surfacing this as a launch failure.
			if cmd.ProcessState != nil {
				out.ExitCode = cmd.ProcessState.ExitCode()
			}
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

// CappedWriter is an io.Writer that keeps only the last Max bytes written, setting Trimmed once
// the cap is exceeded. Shared by Run's pipe-captured stdout/stderr and any other subprocess
// capture (e.g. preview.runRuleCommandCapture's PTY read loop) that needs the same tail-keeping
// cap.
type CappedWriter struct {
	Data    []byte
	Max     int
	Trimmed bool
}

func (w *CappedWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.Data = append(w.Data, p...)
	if len(w.Data) > w.Max {
		w.Trimmed = true
		w.Data = w.Data[len(w.Data)-w.Max:]
	}
	return len(p), nil
}

func (w *CappedWriter) Bytes() []byte {
	b := w.Data
	if !w.Trimmed {
		return append([]byte(nil), b...)
	}
	suffix := truncationMarker
	if len(b)+len(suffix) > w.Max {
		keep := w.Max - len(suffix)
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
