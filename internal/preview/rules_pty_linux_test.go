//go:build linux

package preview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
)

func TestRunRuleCommandCaptureAnswersDA1Query(t *testing.T) {
	// Sends a real DA1 query on its own stdout, reads exactly as many bytes back on stdin as
	// da1Reply(true) is long, and echoes what it received — proving the round trip actually
	// happens over the pty, not just that the scanner logic is correct in isolation.
	script := `printf '\033[0c'; dd bs=1 count=8 2>/dev/null; printf 'DONE'`
	res := runRuleCommandCapture(context.Background(), []string{"sh", "-c", script}, t.TempDir(), 1<<20, true)
	if res.LaunchErr != nil {
		t.Fatalf("LaunchErr = %v", res.LaunchErr)
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0 (stderr/stdout: %q)", res.ExitCode, res.Stdout)
	}
	got := string(res.Stdout)
	if !strings.Contains(got, "\x1b[?62;4c") {
		t.Fatalf("captured output %q does not contain the DA1 reply the child should have read back", got)
	}
	if !strings.HasSuffix(got, "DONE") {
		t.Fatalf("captured output %q does not end with DONE — child read fewer/more bytes than expected", got)
	}
}

func TestRunRuleCommandCaptureExitCodePropagates(t *testing.T) {
	res := runRuleCommandCapture(context.Background(), []string{"sh", "-c", "exit 3"}, t.TempDir(), 1<<20, false)
	if res.ExitCode != 3 {
		t.Fatalf("ExitCode = %d, want 3", res.ExitCode)
	}
}

func TestRunRuleCommandCaptureTruncates(t *testing.T) {
	res := runRuleCommandCapture(context.Background(), []string{"sh", "-c", "printf '0123456789'"}, t.TempDir(), 4, false)
	if !res.StdoutTrim {
		t.Fatal("StdoutTrim = false, want true")
	}
	if len(res.Stdout) != 4 {
		t.Fatalf("len(Stdout) = %d, want 4 (tail bytes kept)", len(res.Stdout))
	}
	if string(res.Stdout) != "6789" {
		t.Fatalf("Stdout = %q, want %q (tail of the output)", res.Stdout, "6789")
	}
}

// holdSlaveThenExit prints DONE, records a child pid that keeps the PTY slave open
// (HUP ignored so the session-leader exit cannot reap it), then replaces the shell
// with true so job-control wait cannot keep the primary alive.
func holdSlaveThenExit(pidFile string) string {
	return fmt.Sprintf(`set +m; (trap "" HUP; exec sleep 120) & echo $! > %q; printf DONE; exec true`, pidFile)
}

func TestRunRuleCommandCaptureReturnsWhenDescendantHoldsSlave(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	t.Cleanup(func() { killPIDFile(pidFile) })

	res := runCaptureOrTimeout(t, context.Background(), []string{"sh", "-c", holdSlaveThenExit(pidFile)}, dir, 1<<20, false)
	if res.LaunchErr != nil {
		t.Fatalf("LaunchErr = %v", res.LaunchErr)
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0 (stdout: %q)", res.ExitCode, res.Stdout)
	}
	if !strings.Contains(string(res.Stdout), "DONE") {
		t.Fatalf("Stdout = %q, want DONE from the exiting shell", res.Stdout)
	}
}

func TestRunRuleCommandCaptureCancelUnblocksInheritedSlave(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	t.Cleanup(func() { killPIDFile(pidFile) })

	script := fmt.Sprintf(`set +m; (trap "" HUP; exec sleep 120) & echo $! > %q; exec sleep 120`, pidFile)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	res := runCaptureOrTimeout(t, ctx, []string{"sh", "-c", script}, dir, 1<<20, false)
	if res.LaunchErr == nil && res.ExitCode == 0 {
		t.Fatalf("cancelled run succeeded (ExitCode=0); want the deadline to stop the primary")
	}
}

func runCaptureOrTimeout(t *testing.T, ctx context.Context, argv []string, dir string, maxBytes int, sixelOK bool) cmdrun.RunResult {
	t.Helper()
	done := make(chan cmdrun.RunResult, 1)
	go func() {
		done <- runRuleCommandCapture(ctx, argv, dir, maxBytes, sixelOK)
	}()
	select {
	case res := <-done:
		return res
	case <-time.After(3 * time.Second):
		t.Fatal("runRuleCommandCapture did not return within 3s; a descendant is likely pinning the PTY master read")
	}
	return cmdrun.RunResult{}
}

func killPIDFile(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = p.Signal(syscall.SIGKILL)
}
