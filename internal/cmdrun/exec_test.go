package cmdrun

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStripJobControlNoise(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no noise untouched", "warning: file exists\n", "warning: file exists\n"},
		{
			"bash -ic startup noise around real output",
			"bash: cannot set terminal process group (-1): Inappropriate ioctl for device\n" +
				"bash: no job control in this shell\n" +
				"real stderr line\n",
			"real stderr line\n",
		},
		{"only noise becomes empty (no toast)", "bash: no job control in this shell\n", ""},
	}
	for _, c := range cases {
		if got := string(stripJobControlNoise([]byte(c.in))); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRun_oversizedStdoutKeepsTail(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("head /dev/zero not available on Windows")
	}
	const maxBytes = 64
	res := Run(context.Background(), []string{"head", "-c", "200", "/dev/zero"}, t.TempDir(), maxBytes)
	if res.LaunchErr != nil {
		t.Fatalf("LaunchErr = %v", res.LaunchErr)
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", res.ExitCode)
	}
	if !res.StdoutTrim {
		t.Fatal("StdoutTrim = false, want true")
	}
	if len(res.Stdout) > maxBytes {
		t.Fatalf("len(Stdout) = %d, want <= %d", len(res.Stdout), maxBytes)
	}
	if !bytes.Contains(res.Stdout, []byte("[output truncated]")) {
		t.Fatalf("Stdout missing truncation marker: %q", res.Stdout)
	}
}

func TestRun_parentExitsWhileChildLives(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process-group capture is Unix-only")
	}
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	t.Cleanup(func() { killPIDFile(pidFile) })

	start := time.Now()
	res := runOrTimeout(t, context.Background(), []string{"sh", "-c", holdSlaveThenExit(pidFile)}, dir, MaxStreamBytes)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Run blocked %v waiting on a descendant pipe; want WaitDelay to unblock", elapsed)
	}
	if res.LaunchErr != nil {
		t.Fatalf("LaunchErr = %v", res.LaunchErr)
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0 (stdout: %q)", res.ExitCode, res.Stdout)
	}
	if !bytes.Contains(res.Stdout, []byte("DONE")) {
		t.Fatalf("Stdout = %q, want DONE from the exiting shell", res.Stdout)
	}
}

func TestRunTracked_cancelKillsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process-group cancel is Unix-only")
	}
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	t.Cleanup(func() { killPIDFile(pidFile) })

	script := fmt.Sprintf(`set +m; (trap "" HUP; exec sleep 120) & echo $! > %q; exec sleep 120`, pidFile)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan RunResult, 1)
	go func() {
		done <- Run(ctx, []string{"sh", "-c", script}, dir, MaxStreamBytes)
	}()

	childPID := waitPIDFile(t, pidFile, 5*time.Second)
	cancel()

	var res RunResult
	select {
	case res = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel; a descendant is likely holding the pipes")
	}
	if res.LaunchErr == nil && res.ExitCode == 0 {
		t.Fatal("cancelled run succeeded (ExitCode=0); want cancel to stop the process group")
	}

	deadline := time.Now().Add(2 * time.Second)
	for processAlive(childPID) {
		if time.Now().After(deadline) {
			t.Fatalf("descendant pid %d still alive after cancel", childPID)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// holdSlaveThenExit records a child pid that keeps stdout open (HUP ignored so
// the session-leader exit cannot reap it), prints DONE, then replaces the shell
// with true so job-control wait cannot keep the primary alive.
func holdSlaveThenExit(pidFile string) string {
	return fmt.Sprintf(`set +m; (trap "" HUP; exec sleep 120) & echo $! > %q; printf DONE; exec true`, pidFile)
}

func runOrTimeout(t *testing.T, ctx context.Context, argv []string, dir string, maxBytes int) RunResult {
	t.Helper()
	done := make(chan RunResult, 1)
	go func() {
		done <- Run(ctx, argv, dir, maxBytes)
	}()
	select {
	case res := <-done:
		return res
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return within 3s; a descendant is likely pinning the output pipes")
	}
	return RunResult{}
}

func waitPIDFile(t *testing.T, path string, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("pid file %s not written", path)
	return 0
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
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
