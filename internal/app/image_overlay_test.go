package app

import (
	"bytes"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func stubNativeSixel(t *testing.T, native bool) {
	t.Helper()
	orig := preview.TmuxSupportsNativeSixel
	t.Cleanup(func() { preview.TmuxSupportsNativeSixel = orig })
	preview.TmuxSupportsNativeSixel = func(func(string) string) bool { return native }
}

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

// TestNativeSixelTransportIgnoresEnvOverride pins the production transport to
// preview.TmuxSupportsNativeSixel only: PC_SIXEL_TRANSPORT must not force native
// when capability detection says no (fake TMUX cannot reach a real server).
func TestNativeSixelTransportIgnoresEnvOverride(t *testing.T) {
	t.Setenv("TMUX", fakeTmuxEnv)
	t.Setenv("PC_SIXEL_TRANSPORT", "native")
	a := &App{}
	if a.nativeSixelTransport(previewpanel.ImageProtocolSixel) {
		t.Fatal("PC_SIXEL_TRANSPORT=native must not force native when TmuxSupportsNativeSixel is false")
	}
	t.Setenv("PC_SIXEL_TRANSPORT", "passthrough")
	if a.nativeSixelTransport(previewpanel.ImageProtocolSixel) {
		t.Fatal("PC_SIXEL_TRANSPORT=passthrough must not change a false capability result")
	}
	if a.nativeSixelTransport(previewpanel.ImageProtocolKitty) {
		t.Fatal("Kitty protocol is never native Sixel transport")
	}
}

func TestWriteImagePayloadOutsideTmuxIsUnwrapped(t *testing.T) {
	t.Setenv("TMUX", "")
	payload := "\x1bPq...sixel-data...\x1b\\"
	var buf bytes.Buffer
	writeImagePayload(&buf, payload, previewpanel.ImageProtocolSixel, false)
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
	writeImagePayload(&buf, payload, previewpanel.ImageProtocolSixel, false)
	want := tmuxPassthroughWrap(payload)
	if buf.String() != want {
		t.Fatalf("writeImagePayload() = %q, want single wrap %q", buf.String(), want)
	}
}

func TestWriteImagePayloadUnderTmuxWrapsKittyChunksSeparately(t *testing.T) {
	t.Setenv("TMUX", fakeTmuxEnv)
	payload := "\x1b_Ga=T,m=1;AAAA\x1b\\\x1b_Gm=0;BBBB\x1b\\"
	var buf bytes.Buffer
	writeImagePayload(&buf, payload, previewpanel.ImageProtocolKitty, false)
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

func TestResetImageOverlayForResizeMarksNativeSixelLost(t *testing.T) {
	stubNativeSixel(t, true)
	t.Setenv("TMUX", fakeTmuxEnv)
	const payload = "\x1bPqtest\x1b\\"
	a := &App{
		image: imageOverlay{
			last: previewpanel.ImagePlacement{
				Payload:  payload,
				Protocol: previewpanel.ImageProtocolSixel,
				X:        2,
				Y:        3,
			},
			lastSet:  true,
			lastCols: 10,
			lastRows: 5,
		},
	}

	a.resetImageOverlayForResize()
	if !a.image.lastSet {
		t.Fatal("resetImageOverlayForResize cleared the native placement, want cached")
	}
	if a.image.last.Payload != payload {
		t.Fatalf("payload = %q, want cached sixel", a.image.last.Payload)
	}
	if !a.image.pendingEmit {
		t.Fatal("pendingEmit = false after tmux resize, want true so the cached payload is retransmitted once")
	}
}

func TestResetImageOverlayForResizeClearsNonNative(t *testing.T) {
	stubNativeSixel(t, false)
	t.Setenv("TMUX", fakeTmuxEnv)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	a := &App{
		screen: screen,
		image: imageOverlay{
			last: previewpanel.ImagePlacement{
				Payload:  "\x1bPqtest\x1b\\",
				Protocol: previewpanel.ImageProtocolSixel,
			},
			lastSet: true,
		},
	}

	a.resetImageOverlayForResize()
	if a.image.lastSet || a.image.pendingEmit {
		t.Fatalf("non-native resize left lastSet=%v pendingEmit=%v, want full reset", a.image.lastSet, a.image.pendingEmit)
	}
}

func TestReconcileImageBeforeShowRetransmitsNativeSixelAfterResize(t *testing.T) {
	stubNativeSixel(t, true)
	t.Setenv("TMUX", fakeTmuxEnv)
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("screen.Init() error = %v", err)
	}
	defer sim.Fini()
	sim.SetSize(80, 24)
	screen := &screenWithTty{Screen: sim, tty: &fakeTty{}}
	a := &App{screen: screen}

	plan := &previewpanel.ImagePlacement{
		Payload:  "\x1bPqtest\x1b\\",
		Path:     "/tmp/a.png",
		Protocol: previewpanel.ImageProtocolSixel,
		X:        2,
		Y:        3,
		PxW:      20,
		PxH:      40,
		MaxCols:  40,
		MaxRows:  20,
	}
	if force := a.reconcileImageBeforeShow(plan); !force {
		t.Fatal("first reconcile = false, want true")
	}
	a.emitImageAfterShow()
	if a.image.pendingEmit {
		t.Fatal("pendingEmit still set after first emit")
	}

	if force := a.reconcileImageBeforeShow(plan); force {
		t.Fatal("unchanged plan forced a retransmit before resize")
	}

	a.resetImageOverlayForResize()
	if !a.image.pendingEmit {
		t.Fatal("pendingEmit = false after resize, want true")
	}
	if force := a.reconcileImageBeforeShow(plan); !force {
		t.Fatal("unchanged plan after resize = false, want forceShow so the cached payload is retransmitted")
	}
	if !a.image.pendingEmit {
		t.Fatal("pendingEmit cleared during reconcile; emitImageAfterShow would skip the retransmit")
	}

	screen.tty.Reset()
	a.emitImageAfterShow()
	if screen.tty.Len() == 0 {
		t.Fatal("resize retransmit did not write the cached payload")
	}
	if a.image.pendingEmit {
		t.Fatal("pendingEmit still set after resize retransmit")
	}
	if force := a.reconcileImageBeforeShow(plan); force {
		t.Fatal("second unchanged reconcile after resize retransmit forced another send")
	}
}
