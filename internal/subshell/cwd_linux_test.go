//go:build linux

package subshell

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// Chdir must mute the injected cd from the terminal panel emulator (MC's QUIETLY feed mode):
// the literal "cd '...'" line and its echo must never render, and normal output must resume
// once the shell settles back at a prompt (proving Unmute actually fires).
func testChdirMutesInjectedCommand(t *testing.T, shell string) {
	if _, err := exec.LookPath(shell); err != nil {
		t.Skipf("%s not installed", shell)
	}
	base := t.TempDir()
	target := filepath.Join(base, "willow grove")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	sub, err := Start(StartOptions{Shell: shell, Dir: base})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	feed, err := sub.StartPanelFeed(80, 24, func() {})
	if err != nil {
		t.Fatal(err)
	}
	pollUntil(t, 5*time.Second, "shell idle at start", func() bool { return !sub.Busy() })

	if err := sub.Chdir(target); err != nil {
		t.Fatal(err)
	}
	pollUntil(t, 5*time.Second, "cwd to follow Chdir", func() bool {
		cwd, err := sub.Cwd()
		return err == nil && cwd == target
	})
	// Chdir's own synthetic Enter must force a fresh prompt render showing the new
	// directory — the mute/settle window can otherwise swallow the shell's real
	// post-cd prompt redraw too, leaving a stale-looking prompt. Matched against the
	// screen with row breaks stripped: t.TempDir()'s long paths can push the prompt
	// past 80 columns, wrapping the directory name itself across two emulator rows.
	pollUntil(t, 3*time.Second, "prompt to redraw with the new directory", func() bool {
		return strings.Contains(strings.ReplaceAll(feedScreenText(feed), "\n", ""), filepath.Base(target))
	})

	if _, err := sub.WritePTY([]byte("echo INJECTION-OK\n")); err != nil {
		t.Fatal(err)
	}
	waitFeedText(t, feed, "INJECTION-OK")
	if strings.Contains(feedScreenText(feed), "cd '") {
		t.Fatalf("injected cd leaked into the emulator:\n%s", feedScreenText(feed))
	}
}

func TestChdirMutesInjectedCommandBash(t *testing.T) { testChdirMutesInjectedCommand(t, "bash") }
func TestChdirMutesInjectedCommandFish(t *testing.T) { testChdirMutesInjectedCommand(t, "fish") }

// Hookless shells (no precmd signal) fall back to a fixed settle window but must still mute.
func TestChdirMutesInjectedCommandHookless(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "cedar")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}

	sub, err := Start(StartOptions{Shell: "sh", Dir: base})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	feed, err := sub.StartPanelFeed(80, 24, func() {})
	if err != nil {
		t.Fatal(err)
	}
	pollUntil(t, 5*time.Second, "shell idle at start", func() bool { return !sub.Busy() })

	if err := sub.Chdir(target); err != nil {
		t.Fatal(err)
	}
	pollUntil(t, 5*time.Second, "cwd via /proc fallback", func() bool {
		cwd, err := sub.Cwd()
		return err == nil && cwd == resolved
	})

	if _, err := sub.WritePTY([]byte("echo INJECTION-OK\n")); err != nil {
		t.Fatal(err)
	}
	waitFeedText(t, feed, "INJECTION-OK")
	if strings.Contains(feedScreenText(feed), "cd '") {
		t.Fatalf("injected cd leaked into the emulator:\n%s", feedScreenText(feed))
	}
}

// Regression: hasPrecmdHook must mirror Start's exact hook-install gate (init != "" &&
// opts.Command == ""), not just precmdInit(shell) != "". A shell named "bash" run in
// Command (stub) mode never gets the hook written, so Chdir must not block for the full
// sync timeout waiting on a cwdGen bump that will never come.
func TestChdirHookGateSkipsCommandMode(t *testing.T) {
	sub, err := Start(StartOptions{Shell: "bash", Command: stubShell})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	readUntil(t, sub.pty, "READY", 3*time.Second)

	start := time.Now()
	if err := sub.Chdir("/tmp"); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Chdir took %v with no real precmd hook installed, want well under the 2s sync timeout", elapsed)
	}
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
