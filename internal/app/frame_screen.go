package app

import (
	"slices"

	"github.com/gdamore/tcell/v2"
)

type frameCell struct {
	x, y  int
	mainc rune
	combc []rune
	style tcell.Style
}

// frameScreen hands tcell only each cell's final value for the frame.
//
// tcell's CellBuffer.Put marks a cell dirty (clearing its lastStr) whenever the cell's string
// changes, even if it changes back before Show. Paint code that blanks an area with Fill and then
// draws text over it therefore leaves every non-blank cell permanently dirty, and tcell re-emits
// all of them on every frame. frameScreen collects SetContent calls and writes the last value of
// each cell to the inner screen at flush time (Show, Sync and every read or direct write), so
// tcell's diff sees real changes only and paint code may freely Fill then repaint.
//
// No ColorNone handling is needed: tcell's Put merges ColorNone styles with the cell's previous
// style, and this repo never uses tcell.ColorNone.
//
// Only the UI goroutine paints, so there is no mutex. Other goroutines only call PostEvent, which
// the embedded screen serves directly.
//
// ponytail: reads and direct writes (Get, GetContent, Put, ...) flush mid-frame instead of being
// served from the buffer. Get+Put is only used by the panel title hover (panel_render.go) and the
// dialog hover (dlg/mouse.go), so those frames can re-emit cells transiently overwritten before
// the flush. Serve reads from pending if those paths ever become hot.
type frameScreen struct {
	tcell.Screen
	pending []frameCell
	index   []int32 // w*h entries; 0 = no pending write, else position in pending + 1
	w, h    int
}

func newFrameScreen(inner tcell.Screen) *frameScreen {
	return &frameScreen{Screen: inner}
}

func (f *frameScreen) SetContent(x, y int, mainc rune, combc []rune, style tcell.Style) {
	if x < 0 || y < 0 {
		return
	}
	if x >= f.w || y >= f.h {
		f.flush()
		if x >= f.w || y >= f.h {
			return
		}
	}
	if len(combc) > 0 {
		combc = slices.Clone(combc)
	} else {
		combc = nil
	}
	c := frameCell{x: x, y: y, mainc: mainc, combc: combc, style: style}
	i := y*f.w + x
	if p := f.index[i]; p != 0 {
		f.pending[p-1] = c
		return
	}
	f.pending = append(f.pending, c)
	f.index[i] = int32(len(f.pending))
}

// flush writes the pending cells to the inner screen, then refreshes the cached size.
func (f *frameScreen) flush() {
	f.writePending()
	if w, h := f.Size(); w != f.w || h != f.h {
		f.w, f.h = w, h
		f.index = make([]int32, w*h)
	}
}

// writePending is the cheap path for reads: no pending cells, no work.
func (f *frameScreen) writePending() {
	if len(f.pending) == 0 {
		return
	}
	for _, c := range f.pending {
		f.Screen.SetContent(c.x, c.y, c.mainc, c.combc, c.style)
		f.index[c.y*f.w+c.x] = 0
	}
	f.pending = f.pending[:0]
}

func (f *frameScreen) Show() { f.flush(); f.Screen.Show() }
func (f *frameScreen) Sync() { f.flush(); f.Screen.Sync() }

func (f *frameScreen) Get(x, y int) (string, tcell.Style, int) {
	f.writePending()
	return f.Screen.Get(x, y)
}

func (f *frameScreen) GetContent(x, y int) (rune, []rune, tcell.Style, int) {
	f.writePending()
	return f.Screen.GetContent(x, y) //nolint:staticcheck // SA1019: forwarding the deprecated interface method
}

func (f *frameScreen) Put(x, y int, str string, style tcell.Style) (string, int) {
	f.writePending()
	return f.Screen.Put(x, y, str, style)
}

func (f *frameScreen) PutStr(x, y int, str string) {
	f.writePending()
	f.Screen.PutStr(x, y, str)
}

func (f *frameScreen) PutStrStyled(x, y int, str string, style tcell.Style) {
	f.writePending()
	f.Screen.PutStrStyled(x, y, str, style)
}

func (f *frameScreen) SetCell(x, y int, style tcell.Style, ch ...rune) {
	f.writePending()
	f.Screen.SetCell(x, y, style, ch...)
}

func (f *frameScreen) Fill(r rune, style tcell.Style) {
	f.writePending()
	f.Screen.Fill(r, style)
}

func (f *frameScreen) Clear() {
	f.writePending()
	f.Screen.Clear()
}
