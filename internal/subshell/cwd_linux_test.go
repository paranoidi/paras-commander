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
	// The precmd pipe reports logical pwd (target as given); /proc fallback reports resolved.
	pollUntil(t, 5*time.Second, "cwd to follow Chdir", func() bool {
		cwd, err := sub.Cwd()
		return err == nil && (cwd == resolved || cwd == target)
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

// The precmd pipe reports the logical $PWD: cd through a symlink must surface the symlinked
// path, which the /proc readlink fallback can never produce.
func testCwdKeepsSymlinkPath(t *testing.T, shell string) {
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "aspen link")
	if err := os.Symlink(filepath.Join(base, "meadow"), link); err != nil {
		t.Fatal(err)
	}

	sub := startInteractive(t, shell, base)
	pollUntil(t, 5*time.Second, "shell idle at start", func() bool { return !sub.Busy() })

	if err := sub.Chdir(link); err != nil {
		t.Fatal(err)
	}
	pollUntil(t, 5*time.Second, "cwd to keep the symlink path", func() bool {
		cwd, err := sub.Cwd()
		return err == nil && cwd == link
	})
}

func TestCwdKeepsSymlinkPathBash(t *testing.T) { testCwdKeepsSymlinkPath(t, "bash") }
func TestCwdKeepsSymlinkPathFish(t *testing.T) { testCwdKeepsSymlinkPath(t, "fish") }

// Shells without a precmd hook keep working through the /proc fallback (resolved paths).
func TestCwdFallbackWithoutPrecmd(t *testing.T) {
	base := t.TempDir()
	resolved, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	sub := startInteractive(t, "sh", "/")
	pollUntil(t, 5*time.Second, "shell idle at start", func() bool { return !sub.Busy() })

	if _, err := sub.WritePTY([]byte(" cd " + QuoteArg(base) + "\n")); err != nil {
		t.Fatal(err)
	}
	pollUntil(t, 5*time.Second, "cwd via /proc fallback", func() bool {
		cwd, err := sub.Cwd()
		return err == nil && cwd == resolved
	})
}

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
	if err := sub.InsertText("'/tmp/file'"); !errors.Is(err, ErrBusy) {
		t.Fatalf("InsertText while busy = %v, want ErrBusy", err)
	}
	pollUntil(t, 5*time.Second, "idle after sleep", func() bool { return !sub.Busy() })
}

// InsertText leaves the text in the input buffer: the stub only sees the line (and echoes
// line:<text>) once a newline is written afterwards.
func TestInsertTextStaysInInputBuffer(t *testing.T) {
	sub, err := Start(StartOptions{Shell: "/bin/sh", Command: stubShell})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	readUntil(t, sub.pty, "READY", 3*time.Second)

	if err := sub.InsertText("'/tmp/alpha beta'"); err != nil {
		t.Fatal(err)
	}
	if _, err := sub.WritePTY([]byte("\n")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, sub.pty, "line:'/tmp/alpha beta'", 3*time.Second)
}
