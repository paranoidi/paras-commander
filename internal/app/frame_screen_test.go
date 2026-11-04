package app

import (
	"bytes"
	"io"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/terminfo"
	_ "github.com/gdamore/tcell/v2/terminfo/extended"
)

// recordingScreen counts the SetContent calls that reach the inner screen.
type recordingScreen struct {
	tcell.SimulationScreen
	calls []frameCell
}

func (r *recordingScreen) SetContent(x, y int, mainc rune, combc []rune, style tcell.Style) {
	r.calls = append(r.calls, frameCell{x: x, y: y, mainc: mainc, combc: combc, style: style})
	r.SimulationScreen.SetContent(x, y, mainc, combc, style)
}

func newRecordingFrame(t *testing.T, w, h int) (*frameScreen, *recordingScreen, tcell.SimulationScreen) {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sim.Fini)
	sim.SetSize(w, h)
	rec := &recordingScreen{SimulationScreen: sim}
	return newFrameScreen(rec), rec, sim
}

func TestFrameScreenCollapsesTransientWrites(t *testing.T) {
	f, rec, _ := newRecordingFrame(t, 10, 3)
	st := tcell.StyleDefault
	f.SetContent(2, 1, 'x', nil, st)
	f.SetContent(2, 1, ' ', nil, st)
	f.SetContent(2, 1, 'x', nil, st)
	if len(rec.calls) != 0 {
		t.Fatalf("writes reached inner before flush: %d", len(rec.calls))
	}
	f.Show()
	if len(rec.calls) != 1 || rec.calls[0].mainc != 'x' || rec.calls[0].x != 2 || rec.calls[0].y != 1 {
		t.Fatalf("calls = %+v, want one 'x' at (2,1)", rec.calls)
	}
	f.Show()
	if len(rec.calls) != 1 {
		t.Fatalf("second Show re-flushed: %d calls", len(rec.calls))
	}
}

func TestFrameScreenGetFlushesAndReturnsFinalValue(t *testing.T) {
	f, rec, _ := newRecordingFrame(t, 10, 3)
	f.SetContent(1, 1, 'a', nil, tcell.StyleDefault)
	f.SetContent(1, 1, 'b', nil, tcell.StyleDefault)
	str, _, _ := f.Get(1, 1)
	if str != "b" {
		t.Fatalf("Get = %q, want b", str)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("inner calls = %d, want 1", len(rec.calls))
	}
}

func TestFrameScreenLastStyleWins(t *testing.T) {
	f, _, sim := newRecordingFrame(t, 10, 3)
	red := tcell.StyleDefault.Foreground(tcell.ColorRed)
	blue := tcell.StyleDefault.Foreground(tcell.ColorBlue)
	f.SetContent(0, 0, 'x', nil, red)
	f.SetContent(0, 0, 'x', nil, blue)
	f.Show()
	_, st, _ := sim.Get(0, 0)
	if st != blue {
		t.Fatalf("style = %v, want blue", st)
	}
}

func TestFrameScreenClonesCombining(t *testing.T) {
	f, rec, _ := newRecordingFrame(t, 10, 3)
	comb := []rune{'́'}
	f.SetContent(0, 0, 'e', comb, tcell.StyleDefault)
	comb[0] = '̂'
	f.Show()
	if len(rec.calls) != 1 || len(rec.calls[0].combc) != 1 || rec.calls[0].combc[0] != '́' {
		t.Fatalf("calls = %+v, want combining U+0301", rec.calls)
	}
	f.SetContent(1, 0, 'e', nil, tcell.StyleDefault)
	f.SetContent(1, 0, 'e', []rune{}, tcell.StyleDefault)
	f.Show()
	if rec.calls[len(rec.calls)-1].combc != nil {
		t.Fatal("empty combc must be stored as nil")
	}
}

func TestFrameScreenOutOfBoundsDropped(t *testing.T) {
	f, rec, _ := newRecordingFrame(t, 4, 2)
	st := tcell.StyleDefault
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {4, 0}, {0, 2}, {100, 100}} {
		f.SetContent(p[0], p[1], 'x', nil, st)
	}
	f.Show()
	if len(rec.calls) != 0 {
		t.Fatalf("out-of-bounds writes reached inner: %+v", rec.calls)
	}
}

func TestFrameScreenResize(t *testing.T) {
	f, _, sim := newRecordingFrame(t, 4, 2)
	f.SetContent(1, 1, 'a', nil, tcell.StyleDefault)
	f.Show()
	sim.SetSize(8, 5)
	f.SetContent(7, 4, 'z', nil, tcell.StyleDefault)
	f.SetContent(1, 1, 'b', nil, tcell.StyleDefault)
	f.Show()
	if str, _, _ := sim.Get(7, 4); str != "z" {
		t.Fatalf("cell (7,4) = %q, want z", str)
	}
	if str, _, _ := sim.Get(1, 1); str != "b" {
		t.Fatalf("cell (1,1) = %q, want b", str)
	}
	sim.SetSize(2, 2)
	f.SetContent(1, 1, 'c', nil, tcell.StyleDefault)
	f.SetContent(7, 4, 'q', nil, tcell.StyleDefault) // stale-size bounds: dropped by inner, no panic
	f.Show()
	if str, _, _ := sim.Get(1, 1); str != "c" {
		t.Fatalf("cell (1,1) = %q, want c", str)
	}
	f.SetContent(5, 5, 'x', nil, tcell.StyleDefault) // now out of bounds after refresh
	f.Show()
}

// captureTty is a Tty that captures everything written to it.
type captureTty struct {
	mu     sync.Mutex
	out    bytes.Buffer
	closed chan struct{}
	once   sync.Once
}

func newCaptureTty() *captureTty { return &captureTty{closed: make(chan struct{})} }

func (t *captureTty) Start() error        { return nil }
func (t *captureTty) Stop() error         { return nil }
func (t *captureTty) Drain() error        { return t.Close() } // wakes the blocked Read so Fini can finish
func (t *captureTty) NotifyResize(func()) {}
func (t *captureTty) WindowSize() (tcell.WindowSize, error) {
	return tcell.WindowSize{Width: 40, Height: 5}, nil
}
func (t *captureTty) Read([]byte) (int, error) { <-t.closed; return 0, io.EOF }
func (t *captureTty) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.out.Write(p)
}
func (t *captureTty) Close() error { t.once.Do(func() { close(t.closed) }); return nil }

func (t *captureTty) take() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	b := bytes.Clone(t.out.Bytes())
	t.out.Reset()
	return b
}

func newTerminfoScreen(t *testing.T) (tcell.Screen, *captureTty) {
	t.Helper()
	ti, err := terminfo.LookupTerminfo("xterm-256color")
	if err != nil {
		t.Fatal(err)
	}
	tty := newCaptureTty()
	s, err := tcell.NewTerminfoScreenFromTtyTerminfo(tty, ti)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Fini)
	return s, tty
}

// paintFrame is the paint pattern the UI uses: blank the row, then draw text over it.
func paintFrame(s tcell.Screen, word string) {
	st := tcell.StyleDefault.Foreground(tcell.ColorGreen)
	for x := 0; x < 40; x++ {
		s.SetContent(x, 2, ' ', nil, st)
	}
	for i, r := range word {
		s.SetContent(3+i, 2, r, nil, st)
	}
}

// TestFrameScreenIdenticalFrameEmitsNothing is the regression test for tcell re-emitting cells
// that were blanked and repainted with the same value within one frame.
func TestFrameScreenIdenticalFrameEmitsNothing(t *testing.T) {
	const word = "quietly"

	raw, rawTty := newTerminfoScreen(t)
	paintFrame(raw, word)
	raw.Show()
	rawTty.take()
	paintFrame(raw, word)
	raw.Show()
	if !bytes.Contains(rawTty.take(), []byte(word)) {
		t.Skip("raw tcell no longer re-emits blank-then-repaint cells; wrapper is unnecessary")
	}

	inner, tty := newTerminfoScreen(t)
	f := newFrameScreen(inner)
	paintFrame(f, word)
	f.Show()
	if !bytes.Contains(tty.take(), []byte(word)) {
		t.Fatal("first frame did not emit the word")
	}
	paintFrame(f, word)
	f.Show()
	if out := tty.take(); bytes.Contains(out, []byte(word)) {
		t.Fatalf("identical second frame re-emitted the word: %q", out)
	}
}
