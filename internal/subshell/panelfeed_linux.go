//go:build linux

package subshell

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
	terminal "github.com/micro-editor/terminal"
	"golang.org/x/sys/unix"
)

// PanelFeed renders the subshell into an in-memory VT10x emulator so the app can
// paint it as an embedded panel. It owns PTY reads while active; RunVisible parks
// it (sole-reader invariant) and tees full-screen output back in so the emulator
// stays current across Ctrl+O round-trips.
type PanelFeed struct {
	sub   *Subshell
	state *terminal.State
	vt    *terminal.VT
	wake  func()

	mu     sync.Mutex // guards reader lifecycle + dims (held across Draw's state.Lock)
	stop   chan struct{}
	parked chan struct{}
	cols   int
	rows   int
	closed bool

	carryMu sync.Mutex
	carry   []byte // partial UTF-8 rune left over by vt.Write

	exited atomic.Bool
	muted  atomic.Bool // see Mute/Unmute
}

// StartPanelFeed sizes the PTY to the panel, attaches an emulator, and starts the
// PTY reader goroutine. wake fires after each output burst (coalesce app-side).
// Only one feed per Subshell; the previous one must be Closed first.
func (s *Subshell) StartPanelFeed(cols, rows int, wake func()) (*PanelFeed, error) {
	if !s.Alive() {
		return nil, ErrNotAlive
	}
	f := &PanelFeed{
		sub:   s,
		state: &terminal.State{},
		wake:  wake,
		cols:  cols,
		rows:  rows,
	}
	vt, err := terminal.Create(f.state, io.NopCloser(bytes.NewReader(nil)))
	if err != nil {
		return nil, err
	}
	f.vt = vt
	vt.Resize(cols, rows)
	_ = setPTYSizeCells(s.ptyFD, cols, rows)

	s.mu.Lock()
	s.feed = f
	s.mu.Unlock()

	f.startReader()
	return f, nil
}

func (s *Subshell) panelFeed() *PanelFeed {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.feed
}

// Resize moves the PTY winsize and the emulator together (they must never diverge:
// State.Cell has no bounds checks, so Draw dims must match the emulator's).
func (f *PanelFeed) Resize(cols, rows int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed || cols <= 0 || rows <= 0 {
		return
	}
	f.vt.Resize(cols, rows)
	f.cols, f.rows = cols, rows
	_ = setPTYSizeCells(f.sub.ptyFD, cols, rows)
}

// Pause parks the PTY reader (blocks until it acks). RunVisible calls this so its
// feed loop is the sole PTY reader during full-screen.
func (f *PanelFeed) Pause() {
	f.mu.Lock()
	stop, parked := f.stop, f.parked
	f.stop = nil
	f.mu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	<-parked
}

// Resume restarts the reader and restores panel dims after a full-screen session.
func (f *PanelFeed) Resume(cols, rows int) {
	f.Resize(cols, rows)
	f.startReader()
}

// Close stops the reader and detaches from the Subshell. The shell stays alive —
// the panel is a view of the persistent shell, never its owner.
func (f *PanelFeed) Close() {
	f.Pause()
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	f.sub.mu.Lock()
	if f.sub.feed == f {
		f.sub.feed = nil
	}
	f.sub.mu.Unlock()
}

// Exited reports that the shell child died while the feed was active.
func (f *PanelFeed) Exited() bool { return f.exited.Load() }

// Cursor returns the emulator cursor position and visibility (panel-local cells).
func (f *PanelFeed) Cursor() (x, y int, visible bool) {
	f.state.Lock()
	defer f.state.Unlock()
	x, y = f.state.Cursor()
	return x, y, f.state.CursorVisible()
}

// AppCursor reports DECCKM application-cursor mode (for EncodeKey).
func (f *PanelFeed) AppCursor() bool {
	f.state.Lock()
	defer f.state.Unlock()
	return f.state.Mode(terminal.ModeAppCursor)
}

// Draw paints the emulator grid via setCell (panel-local coordinates). Colors
// mapped to the tcell palette; emulator default fg/bg take defaultStyle's.
// Returns the cursor position and visibility.
func (f *PanelFeed) Draw(defaultStyle tcell.Style, setCell func(x, y int, r rune, style tcell.Style)) (cursorX, cursorY int, cursorVisible bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	defFG, defBG, _ := defaultStyle.Decompose()
	f.state.Lock()
	defer f.state.Unlock()
	for y := 0; y < f.rows; y++ {
		for x := 0; x < f.cols; x++ {
			ch, fg, bg := f.state.Cell(x, y)
			setCell(x, y, ch, tcell.StyleDefault.
				Foreground(mapVTColor(fg, defFG)).
				Background(mapVTColor(bg, defBG)))
		}
	}
	cursorX, cursorY = f.state.Cursor()
	return cursorX, cursorY, f.state.CursorVisible()
}

// WriteScreenTo dumps the current emulator cell content to w as ANSI-escaped
// text at the current emulator dimensions, leaving the terminal cursor at the
// emulator's cursor position (an idle shell's next echo must land at the
// prompt). Called after syncVTToPTY while the feed is paused (no concurrent
// reader) — the emulator already has the content the session should show.
func (f *PanelFeed) WriteScreenTo(w io.Writer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.Lock()
	defer f.state.Unlock()

	if _, err := io.WriteString(w, "\033[0m\033[H\033[J"); err != nil {
		return err
	}
	curFG, curBG := terminal.DefaultFG, terminal.DefaultBG
	for y := 0; y < f.rows; y++ {
		if _, err := fmt.Fprintf(w, "\033[%dH", y+1); err != nil {
			return err
		}
		for x := 0; x < f.cols; x++ {
			ch, fg, bg := f.state.Cell(x, y)
			if ch == 0 {
				ch = ' '
			}
			if fg != curFG || bg != curBG {
				if _, err := io.WriteString(w, sgrColors(fg, bg)); err != nil {
					return err
				}
				curFG, curBG = fg, bg
			}
			if _, err := fmt.Fprintf(w, "%c", ch); err != nil {
				return err
			}
		}
	}
	cx, cy := f.state.Cursor()
	_, err := fmt.Fprintf(w, "\033[0m\033[%d;%dH", cy+1, cx+1)
	return err
}

// sgrColors emits 256-color SGR for a cell (defaults as 39/49).
// ponytail: State.Cell exposes no attributes — colors only, no bold/underline.
func sgrColors(fg, bg terminal.Color) string {
	f, b := "39", "49"
	if fg < 256 {
		f = fmt.Sprintf("38;5;%d", fg)
	}
	if bg < 256 {
		b = fmt.Sprintf("48;5;%d", bg)
	}
	return "\033[" + f + ";" + b + "m"
}

// mapVTColor converts a vt10x color (ANSI 0-15 + xterm 256) to tcell.
// ponytail: truecolor SGR (38;2;…) is not representable in vt10x's uint16 — falls
// back to def; upgrade means swapping emulators, not mapping harder.
func mapVTColor(c terminal.Color, def tcell.Color) tcell.Color {
	if c < 256 {
		return tcell.PaletteColor(int(c))
	}
	return def
}

// teeWriter feeds bytes into the emulator and never errors (safe for MultiWriter
// with os.Stdout in the visible feed loop).
func (f *PanelFeed) teeWriter() io.Writer { return vtTee{f} }

type vtTee struct{ f *PanelFeed }

func (w vtTee) Write(p []byte) (int, error) {
	w.f.feedBytes(p)
	return len(p), nil
}

// syncVTToPTY resizes the emulator to the PTY's current winsize (RunVisible calls
// this after syncPTYSize so the tee parses against full-screen dims).
func (f *PanelFeed) syncVTToPTY() {
	ws, err := ptySizeCells(f.sub.ptyFD)
	if err != nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	f.vt.Resize(ws.cols, ws.rows)
	f.cols, f.rows = ws.cols, ws.rows
}

// Mute discards future PTY bytes instead of feeding them to the emulator — used by
// [Subshell.Chdir] so an injected cd's echo never reaches the screen (MC's QUIETLY
// feed mode). Drops any pending partial-UTF8 rune synchronously: those bytes belong
// to the pre-mute stream and will never be completed now, and clearing them here
// (rather than in Unmute) avoids a race with the reader goroutine calling feedBytes
// the instant Unmute flips the flag back.
func (f *PanelFeed) Mute() {
	f.carryMu.Lock()
	f.carry = nil
	f.carryMu.Unlock()
	f.muted.Store(true)
}

// Unmute resumes feeding PTY bytes to the emulator.
func (f *PanelFeed) Unmute() {
	f.muted.Store(false)
}

// feedBytes parses a PTY chunk into the emulator, carrying partial UTF-8 runes
// (vt.Write reports a short count when the chunk ends mid-rune).
func (f *PanelFeed) feedBytes(p []byte) {
	if f.muted.Load() {
		return
	}
	f.carryMu.Lock()
	defer f.carryMu.Unlock()
	buf := p
	if len(f.carry) > 0 {
		buf = append(f.carry, p...)
	}
	n, err := f.vt.Write(buf)
	if err != nil || n >= len(buf) {
		f.carry = nil
		return
	}
	f.carry = append([]byte(nil), buf[n:]...)
}

func (f *PanelFeed) startReader() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stop != nil || f.closed {
		return
	}
	f.stop = make(chan struct{})
	f.parked = make(chan struct{})
	go f.run(f.stop, f.parked)
}

// run is the PTY reader: same poll + re-arm-SetNonblock pattern as runVisiblePoll
// (any File.Fd() call flips the fd back to blocking; re-arm every wakeup).
func (f *PanelFeed) run(stop, parked chan struct{}) {
	defer close(parked)
	ptyFD := f.sub.ptyFD
	buf := make([]byte, 4096)
	pfd := []unix.PollFd{{Fd: int32(ptyFD), Events: unix.POLLIN}}
	for {
		select {
		case <-stop:
			return
		case <-f.sub.dead:
			f.exited.Store(true)
			f.wake()
			return
		default:
		}

		pfd[0].Revents = 0
		_, err := unix.Poll(pfd, 100)
		if err != nil && !errors.Is(err, unix.EINTR) {
			return
		}
		_ = unix.SetNonblock(ptyFD, true)
		if pfd[0].Revents&(unix.POLLIN|unix.POLLHUP) == 0 {
			continue
		}

		got := false
		hup := false
		for {
			n, rerr := unix.Read(ptyFD, buf)
			if n > 0 {
				f.feedBytes(buf[:n])
				got = true
			}
			if errors.Is(rerr, unix.EAGAIN) {
				break
			}
			if n <= 0 {
				hup = true
				break
			}
		}
		if got {
			f.wake()
		}
		if hup {
			// Slave side gone (child exiting); wait for the definitive signal.
			select {
			case <-stop:
			case <-f.sub.dead:
				f.exited.Store(true)
				f.wake()
			}
			return
		}
	}
}

type cellSize struct{ cols, rows int }

// setPTYSizeCells and ptySizeCells take the cached raw fd, not *os.File: creack/pty's own
// Setsize/GetsizeFull call .Fd() internally, which flips the fd back to blocking mode (their
// own tests acknowledge the resulting race — "Potential in (*os.File).Fd()") and can race
// PanelFeed.run's reader goroutine mid-read, hanging it exactly like the Busy()/.Fd() bug
// fixed earlier in Subshell.ptyFD.
func setPTYSizeCells(ptyFD int, cols, rows int) error {
	return unix.IoctlSetWinsize(ptyFD, unix.TIOCSWINSZ, &unix.Winsize{Col: uint16(cols), Row: uint16(rows)})
}

func ptySizeCells(ptyFD int) (cellSize, error) {
	ws, err := unix.IoctlGetWinsize(ptyFD, unix.TIOCGWINSZ)
	if err != nil {
		return cellSize{}, err
	}
	return cellSize{cols: int(ws.Col), rows: int(ws.Row)}, nil
}
