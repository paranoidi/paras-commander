//go:build linux

package subshell

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// stubShell echoes READY then answers PING with PONG until exit.
const stubShell = `
echo READY
while IFS= read -r line; do
  case "$line" in
    PING) echo PONG ;;
    exit) exit 0 ;;
    *) echo "line:$line" ;;
  esac
done
`

func readUntil(t *testing.T, r io.Reader, substr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var buf bytes.Buffer
	tmp := make([]byte, 256)
	for time.Now().Before(deadline) {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			if strings.Contains(buf.String(), substr) {
				return
			}
		}
		if err != nil {
			if err == io.EOF && strings.Contains(buf.String(), substr) {
				return
			}
			if err != io.EOF {
				t.Fatalf("read: %v (buf=%q)", err, buf.String())
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %q in %q", substr, buf.String())
}

func TestSpikeStubShellSurvivesTwoToggles(t *testing.T) {
	sub, err := Start(StartOptions{Shell: "/bin/sh", Command: stubShell})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	readUntil(t, sub.pty, "READY", 3*time.Second)

	// First visible session: send a line, toggle back with Ctrl+O.
	toggled, err := sub.RunVisibleFeed(
		bytes.NewReader(append([]byte("alpha\n"), ToggleKeyCtrlO)),
		io.Discard,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !toggled {
		t.Fatal("expected toggle back")
	}
	if !sub.Alive() {
		t.Fatal("shell should survive first toggle")
	}

	// Talk to shell while commander would be visible.
	if _, err := sub.WritePTY([]byte("PING\n")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, sub.pty, "PONG", 2*time.Second)

	// Second visible session: toggle immediately.
	toggled, err = sub.RunVisibleFeed(bytes.NewReader([]byte{ToggleKeyCtrlO}), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !toggled {
		t.Fatal("expected second toggle back")
	}
	if !sub.Alive() {
		t.Fatal("shell should survive second toggle")
	}

	// Exit in shell ends the child.
	if _, err := sub.WritePTY([]byte("exit\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for sub.Alive() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if sub.Alive() {
		t.Fatal("shell should exit after exit command")
	}
}

func TestForceFullRedrawDeliversWinch(t *testing.T) {
	sub, err := Start(StartOptions{
		Shell:   "/bin/sh",
		Command: "trap 'echo WINCHED' WINCH; echo READY; while :; do sleep 0.1; done",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	// Headless runs leave the PTY at 0x0; forceFullRedraw needs a real size to jiggle.
	if err := pty.Setsize(sub.pty, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		t.Fatal(err)
	}
	readUntil(t, sub.pty, "READY", 3*time.Second)

	forceFullRedraw(sub.pty)
	readUntil(t, sub.pty, "WINCHED", 3*time.Second)
}

func TestSpikeStartRequiresShell(t *testing.T) {
	sub, err := Start(StartOptions{Shell: "/bin/sh", Command: "echo sub-ok"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	readUntil(t, sub.pty, "sub-ok", 2*time.Second)
}
