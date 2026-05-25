//go:build linux

package subshell

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
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
	spike, err := StartSpike(StartOptions{Shell: "/bin/sh", Command: stubShell})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = spike.Close() })

	readUntil(t, spike.pty, "READY", 3*time.Second)

	// First visible session: send a line, toggle back with Ctrl+O.
	toggled, err := spike.RunVisibleFeed(
		bytes.NewReader(append([]byte("alpha\n"), ToggleKeyCtrlO)),
		io.Discard,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !toggled {
		t.Fatal("expected toggle back")
	}
	if !spike.Alive() {
		t.Fatal("shell should survive first toggle")
	}

	// Talk to shell while commander would be visible.
	if _, err := spike.WritePTY([]byte("PING\n")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, spike.pty, "PONG", 2*time.Second)

	// Second visible session: toggle immediately.
	toggled, err = spike.RunVisibleFeed(bytes.NewReader([]byte{ToggleKeyCtrlO}), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !toggled {
		t.Fatal("expected second toggle back")
	}
	if !spike.Alive() {
		t.Fatal("shell should survive second toggle")
	}

	// Exit in shell ends the child.
	if _, err := spike.WritePTY([]byte("exit\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for spike.Alive() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if spike.Alive() {
		t.Fatal("shell should exit after exit command")
	}
}

func TestSpikeStartRequiresShell(t *testing.T) {
	spike, err := StartSpike(StartOptions{Shell: "/bin/sh", Command: "echo spike-ok"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = spike.Close() })
	readUntil(t, spike.pty, "spike-ok", 2*time.Second)
}
