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
