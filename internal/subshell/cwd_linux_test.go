//go:build linux

package subshell

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// startInteractive launches a real interactive shell on the PTY and discards its output
// (prompts, echo) so the PTY buffer never fills.
func startInteractive(t *testing.T, shell, dir string) *Subshell {
	t.Helper()
	if _, err := exec.LookPath(shell); err != nil {
		t.Skipf("%s not installed", shell)
	}
	sub, err := Start(StartOptions{Shell: shell, Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	go func() { _, _ = io.Copy(io.Discard, sub.pty) }()
	return sub
}

func pollUntil(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

func testChdirCwdRoundtrip(t *testing.T, shell string) {
	base := t.TempDir()
	target := filepath.Join(base, "it's a dir")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}

	sub := startInteractive(t, shell, base)
	pollUntil(t, 5*time.Second, "shell idle at start", func() bool { return !sub.Busy() })

	if err := sub.Chdir(target); err != nil {
		t.Fatal(err)
	}
	pollUntil(t, 5*time.Second, "cwd to follow Chdir", func() bool {
		cwd, err := sub.Cwd()
		return err == nil && cwd == resolved
	})

	// Shell → commander direction: cd typed "in the shell" is visible via Cwd.
	if _, err := sub.WritePTY([]byte(" cd /\n")); err != nil {
		t.Fatal(err)
	}
	pollUntil(t, 5*time.Second, "cwd to follow typed cd", func() bool {
		cwd, err := sub.Cwd()
		return err == nil && cwd == "/"
	})
}

func TestChdirCwdRoundtripBash(t *testing.T) { testChdirCwdRoundtrip(t, "bash") }
func TestChdirCwdRoundtripFish(t *testing.T) { testChdirCwdRoundtrip(t, "fish") }

// Regression: File.Fd() flips the pollable PTY back to blocking mode, so drainPTYToWriter on
// an idle shell (no pending output — the interactive Ctrl+O case) blocked forever and froze
// the toggle back to the commander.
func TestDrainPTYReturnsOnIdleShell(t *testing.T) {
	sub := startInteractive(t, "bash", t.TempDir())
	pollUntil(t, 5*time.Second, "shell idle at start", func() bool { return !sub.Busy() })
	time.Sleep(200 * time.Millisecond) // let prompt output settle so the PTY is empty

	done := make(chan struct{})
	go func() {
		var buf bytes.Buffer
		drainPTYToWriter(sub.pty, &buf, 150*time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("drainPTYToWriter blocked on idle shell (Ctrl+O toggle-out freeze)")
	}
}

func TestBusyRefusesChdir(t *testing.T) {
	sub := startInteractive(t, "bash", t.TempDir())
	pollUntil(t, 5*time.Second, "shell idle at start", func() bool { return !sub.Busy() })

	if _, err := sub.WritePTY([]byte("sleep 1\n")); err != nil {
		t.Fatal(err)
	}
	pollUntil(t, 3*time.Second, "busy while sleep runs", sub.Busy)

	if err := sub.Chdir("/tmp"); !errors.Is(err, ErrBusy) {
		t.Fatalf("Chdir while busy = %v, want ErrBusy", err)
	}
	pollUntil(t, 5*time.Second, "idle after sleep", func() bool { return !sub.Busy() })
}
