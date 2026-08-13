package app

import (
	"bytes"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// fakeTmuxEnv is a TMUX value that can never resolve to a real tmux socket (unlike e.g.
// "/tmp/tmux-1000/default,..." which collides with the actual default socket a developer
// machine may have live, making preview.TmuxSupportsNativeSixel's subprocess call return real
// data instead of failing closed).
const fakeTmuxEnv = "/nonexistent-tmux-test-socket,1234,0"

func TestSplitTerminatedSequences(t *testing.T) {
	payload := "\x1b_Ga=T,m=1;AAAA\x1b\\\x1b_Gm=0;BBBB\x1b\\"
	got := splitTerminatedSequences(payload)
	want := []string{
		"\x1b_Ga=T,m=1;AAAA\x1b\\",
		"\x1b_Gm=0;BBBB\x1b\\",
	}
	if len(got) != len(want) {
		t.Fatalf("splitTerminatedSequences() = %d chunks, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("chunk %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTmuxPassthroughWrapDoublesEmbeddedEscapes(t *testing.T) {
	seq := "\x1b_Ga=d,d=I,i=1\x1b\\"
	got := tmuxPassthroughWrap(seq)
	want := "\x1bPtmux;\x1b\x1b_Ga=d,d=I,i=1\x1b\x1b\\\x1b\\"
	if got != want {
		t.Fatalf("tmuxPassthroughWrap() = %q, want %q", got, want)
	}
}

func TestWriteKittyDeleteOutsideTmux(t *testing.T) {
	t.Setenv("TMUX", "")
	var buf bytes.Buffer
	writeKittyDelete(&buf)
	want := "\x1b_Ga=d,d=I,i=1\x1b\\"
	if buf.String() != want {
		t.Fatalf("writeKittyDelete() = %q, want unwrapped %q", buf.String(), want)
	}
}

func TestWriteKittyDeleteUnderTmuxIsWrapped(t *testing.T) {
	t.Setenv("TMUX", fakeTmuxEnv)
	var buf bytes.Buffer
	writeKittyDelete(&buf)
	want := tmuxPassthroughWrap("\x1b_Ga=d,d=I,i=1\x1b\\")
	if buf.String() != want {
		t.Fatalf("writeKittyDelete() = %q, want wrapped %q", buf.String(), want)
	}
}

func TestWriteImagePayloadOutsideTmuxIsUnwrapped(t *testing.T) {
	t.Setenv("TMUX", "")
	payload := "\x1bPq...sixel-data...\x1b\\"
	var buf bytes.Buffer
	writeImagePayload(&buf, payload, previewpanel.ImageProtocolSixel)
	if buf.String() != payload {
		t.Fatalf("writeImagePayload() = %q, want unwrapped %q", buf.String(), payload)
	}
}

// TestWriteImagePayloadUnderTmuxWrapsSixelAsOnePiece covers a Sixel payload (a single DCS
// sequence with only its own leading/trailing ESC, no internal ones) when tmux's attached
// outer terminal isn't confirmed to support sixel (preview.TmuxSupportsNativeSixel is false
// here since fakeTmuxEnv can't reach a real tmux server): it must still get
// passthrough-wrapped — tmux has no native understanding of anything sent through passthrough,
// so leaving it unwrapped without confirmed native support meant nothing rendered under tmux
// at all. Splitting a single-terminator payload must yield exactly one chunk, wrapped once —
// not split mid-sequence.
func TestWriteImagePayloadUnderTmuxWrapsSixelAsOnePiece(t *testing.T) {
	t.Setenv("TMUX", fakeTmuxEnv)
	payload := "\x1bPq\"1;1;10;10#0;2;0;0;0#0~~~~$-\x1b\\"
	var buf bytes.Buffer
	writeImagePayload(&buf, payload, previewpanel.ImageProtocolSixel)
	want := tmuxPassthroughWrap(payload)
	if buf.String() != want {
		t.Fatalf("writeImagePayload() = %q, want single wrap %q", buf.String(), want)
	}
}

func TestWriteImagePayloadUnderTmuxWrapsKittyChunksSeparately(t *testing.T) {
	t.Setenv("TMUX", fakeTmuxEnv)
	payload := "\x1b_Ga=T,m=1;AAAA\x1b\\\x1b_Gm=0;BBBB\x1b\\"
	var buf bytes.Buffer
	writeImagePayload(&buf, payload, previewpanel.ImageProtocolKitty)
	want := tmuxPassthroughWrap("\x1b_Ga=T,m=1;AAAA\x1b\\") + tmuxPassthroughWrap("\x1b_Gm=0;BBBB\x1b\\")
	if buf.String() != want {
		t.Fatalf("writeImagePayload() = %q, want %q", buf.String(), want)
	}
}

// TestReconcilePlaceholderImageNilIsNoopOnEmptyState guards the early-out in
// reconcilePlaceholderImage: clearing already-empty state must not touch the tty (verified
// indirectly — SimulationScreen has no tty at all, so any attempted write would panic/error
// rather than silently no-op, and this test would fail loudly if the early return were removed).
func TestReconcilePlaceholderImageNilIsNoopOnEmptyState(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	a := &App{screen: screen}

	a.reconcilePlaceholderImage(nil)
	if a.placeholderImg.sent {
		t.Fatal("reconcilePlaceholderImage(nil) on empty state left sent=true")
	}
}

// fakeTty is a minimal tcell.Tty backed by a buffer, so tests can exercise
// reconcilePlaceholderImage's transmit path (which needs a.screen.Tty() to succeed) without a
// real terminal.
type fakeTty struct {
	bytes.Buffer
}

func (*fakeTty) Start() error                          { return nil }
func (*fakeTty) Stop() error                           { return nil }
func (*fakeTty) Drain() error                          { return nil }
func (*fakeTty) Close() error                          { return nil }
func (*fakeTty) NotifyResize(func())                   {}
func (*fakeTty) WindowSize() (tcell.WindowSize, error) { return tcell.WindowSize{}, nil }

// screenWithTty wraps a tcell.Screen and reports a fakeTty as available, since
// tcell.SimulationScreen.Tty() always returns (nil, false).
type screenWithTty struct {
	tcell.Screen
	tty *fakeTty
}

func (s *screenWithTty) Tty() (tcell.Tty, bool) { return s.tty, true }

// TestReconcileImageBeforeShowForcesShowOnPlaceholderPayloadChange pins down the fix for the
// live-reported Kitty+tmux disappearing-image bug: the Unicode-placeholder grid's cell bytes
// (rune + diacritics + color) encode only row, column, and the fixed KittyGraphicsImageID —
// never which image currently backs that id — so two different images at the same on-screen
// grid size produce byte-for-byte identical cell content and the render hash-cache can't see
// the change. reconcileImageBeforeShow must report forceShow=true whenever
// reconcilePlaceholderImage actually transmits new data, or Show() (and so the terminal redraw
// Kitty needs to notice the new data) gets skipped even though fresh image bytes just went out.
func TestReconcileImageBeforeShowForcesShowOnPlaceholderPayloadChange(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("screen.Init() error = %v", err)
	}
	defer sim.Fini()
	sim.SetSize(80, 24)
	screen := &screenWithTty{Screen: sim, tty: &fakeTty{}}
	a := &App{screen: screen}

	planA := &previewpanel.ImagePlacement{
		Payload:            "\x1b_Ga=T,f=100,i=1,q=2,U=1,m=0;AAAA\x1b\\",
		Path:               "/tmp/a.png",
		Protocol:           previewpanel.ImageProtocolKitty,
		UnicodePlaceholder: true,
	}
	if force := a.reconcileImageBeforeShow(planA); !force {
		t.Fatal("reconcileImageBeforeShow() = false on first placeholder transmit, want true")
	}
	if screen.tty.Len() == 0 {
		t.Fatal("first transmit did not write to tty")
	}

	screen.tty.Reset()
	if force := a.reconcileImageBeforeShow(planA); force {
		t.Fatal("reconcileImageBeforeShow() = true for unchanged placeholder payload, want false")
	}
	if screen.tty.Len() != 0 {
		t.Fatal("unchanged payload retransmitted to tty, want no-op")
	}

	// planB has the same on-screen grid geometry as planA (Draw would emit identical cell
	// bytes for both), only the transmitted image data differs — exactly the case tcell's own
	// diffing and the app's render hash-cache both can't detect on their own.
	planB := &previewpanel.ImagePlacement{
		Payload:            "\x1b_Ga=T,f=100,i=1,q=2,U=1,m=0;BBBB\x1b\\",
		Path:               "/tmp/b.png",
		Protocol:           previewpanel.ImageProtocolKitty,
		UnicodePlaceholder: true,
	}
	if force := a.reconcileImageBeforeShow(planB); !force {
		t.Fatal("reconcileImageBeforeShow() = false when placeholder payload changed, want true")
	}
	if screen.tty.Len() == 0 {
		t.Fatal("changed payload was not retransmitted to tty")
	}
}

// TestResetImageOverlayClearsPlaceholderState ensures Suspend/Resume / Sync paths force a
// re-transmit on the next render: if placeholderImg were left with sent=true and the same
// payload, reconcilePlaceholderImage would skip the transmit after the terminal's graphics
// registry was wiped.
func TestResetImageOverlayClearsPlaceholderState(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	a := &App{
		screen: screen,
		placeholderImg: placeholderImage{
			sent:    true,
			payload: "\x1b_Ga=T,f=100,i=1,q=2,U=1,m=0;AAAA\x1b\\",
		},
	}

	a.resetImageOverlay()
	if a.placeholderImg.sent || a.placeholderImg.payload != "" {
		t.Fatalf("resetImageOverlay() left placeholderImg = %+v, want zero value", a.placeholderImg)
	}
}
