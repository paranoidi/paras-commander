//go:build linux

package subshell

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func feedScreenText(f *PanelFeed) string {
	var b strings.Builder
	lastY := -1
	f.Draw(tcell.StyleDefault, func(x, y int, r rune, _ tcell.Style) {
		if y != lastY {
			if lastY >= 0 {
				b.WriteRune('\n')
			}
			lastY = y
		}
		if r != 0 {
			b.WriteRune(r)
		}
	})
	return b.String()
}

func waitFeedText(t *testing.T, f *PanelFeed, substr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(feedScreenText(f), substr) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %q in emulator screen:\n%s", substr, feedScreenText(f))
}

func TestPanelFeedRendersOutputAndColors(t *testing.T) {
	sub, err := Start(StartOptions{Shell: "/bin/sh", Command: `printf '\033[31mcrimson\033[0m plain\n'; echo READY; cat`})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	var wakes atomic.Int64
	feed, err := sub.StartPanelFeed(40, 6, func() { wakes.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	waitFeedText(t, feed, "READY")

	var crimsonStyle tcell.Style
	base := tcell.StyleDefault.Foreground(tcell.ColorGray).Background(tcell.ColorBlack)
	feed.Draw(base, func(x, y int, r rune, style tcell.Style) {
		if y == 0 && r == 'c' {
			crimsonStyle = style
		}
	})
	fg, _, _ := crimsonStyle.Decompose()
	if fg != tcell.PaletteColor(1) {
		t.Fatalf("SGR 31 fg = %v, want palette red", fg)
	}
	if wakes.Load() == 0 {
		t.Fatal("wake never fired")
	}
}

func TestPanelFeedResizeReflectsInDraw(t *testing.T) {
	sub, err := Start(StartOptions{Shell: "/bin/sh", Command: "echo READY; cat"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	feed, err := sub.StartPanelFeed(40, 6, func() {})
	if err != nil {
		t.Fatal(err)
	}
	waitFeedText(t, feed, "READY")

	feed.Resize(20, 4)
	maxX, maxY := 0, 0
	feed.Draw(tcell.StyleDefault, func(x, y int, _ rune, _ tcell.Style) {
		maxX, maxY = max(maxX, x), max(maxY, y)
	})
	if maxX != 19 || maxY != 3 {
		t.Fatalf("draw grid = %dx%d, want 20x4", maxX+1, maxY+1)
	}
	if ws, err := ptySizeCells(sub.ptyFD); err != nil || ws.cols != 20 || ws.rows != 4 {
		t.Fatalf("pty size = %+v (err %v), want 20x4", ws, err)
	}
}

func TestPanelFeedPauseHandsOffToVisibleFeed(t *testing.T) {
	sub, err := Start(StartOptions{Shell: "/bin/sh", Command: stubShell})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	feed, err := sub.StartPanelFeed(40, 6, func() {})
	if err != nil {
		t.Fatal(err)
	}
	waitFeedText(t, feed, "READY")

	// Park the reader, then run the visible feed loop as sole reader with the
	// emulator tee'd in — the RunVisible handoff, minus the /dev/tty parts.
	feed.Pause()
	out := io.MultiWriter(io.Discard, feed.teeWriter())
	toggled, err := sub.RunVisibleFeed(bytes.NewReader(append([]byte("PING\n"), ToggleKeyCtrlO)), out)
	if err != nil || !toggled {
		t.Fatalf("visible feed: toggled=%v err=%v", toggled, err)
	}
	if !strings.Contains(feedScreenText(feed), "PING") {
		t.Fatalf("tee missed the visible session's echo:\n%s", feedScreenText(feed))
	}

	// Reader resumes and picks up what the toggle-out drain left in the PTY buffer,
	// then keeps consuming fresh output (marker, not a replay).
	feed.Resume(40, 6)
	waitFeedText(t, feed, "PONG")
	if _, err := sub.WritePTY([]byte("marmalade\n")); err != nil {
		t.Fatal(err)
	}
	waitFeedText(t, feed, "line:marmalade")
}

func TestWriteScreenToReplaysColorsAndCursor(t *testing.T) {
	sub, err := Start(StartOptions{Shell: "/bin/sh", Command: `printf '\033[31mcrimson\033[0m\n'; echo READY; cat`})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	feed, err := sub.StartPanelFeed(40, 6, func() {})
	if err != nil {
		t.Fatal(err)
	}
	waitFeedText(t, feed, "READY")
	feed.Pause()

	var buf bytes.Buffer
	if err := feed.WriteScreenTo(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "\033[38;5;1;49mcrimson") {
		t.Fatalf("replay lost SGR color:\n%q", out)
	}
	cx, cy, _ := feed.Cursor()
	if want := fmt.Sprintf("\033[0m\033[%d;%dH", cy+1, cx+1); !strings.HasSuffix(out, want) {
		t.Fatalf("replay must end with cursor restore %q:\n%q", want, out)
	}
}

func TestPanelFeedExited(t *testing.T) {
	sub, err := Start(StartOptions{Shell: "/bin/sh", Command: "echo BYE"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	feed, err := sub.StartPanelFeed(40, 6, func() {})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !feed.Exited() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !feed.Exited() {
		t.Fatal("feed never reported child exit")
	}
}
