//go:build linux

package preview

import (
	"context"
	"strings"
	"testing"
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
