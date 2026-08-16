package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// TerminalDrawer paints the embedded terminal panel's live content and forwards input to it.
// Both the persistent Alt+P shell and a run-for-each PTY session implement it, so
// internal/app's panel plumbing needs no special-casing between the two. setCell coordinates
// are panel-content-local (0,0 = first content row, first column); drawTerminalPanel's caller
// offsets/clips into the panel rect. The returned cursor position is panel-content-local too;
// cursorVisible false hides the cursor.
type TerminalDrawer interface {
	DrawTo(setCell func(x, y int, r rune, style tcell.Style)) (cursorX, cursorY int, cursorVisible bool)
	// WriteInput sends encoded key bytes to the live session.
	WriteInput(b []byte) (int, error)
	// AppCursorMode reports DECCKM state (for subshell.EncodeKey).
	AppCursorMode() bool
	// Cursor returns the emulator cursor position/visibility without a full paint pass.
	Cursor() (x, y int, visible bool)
}

// TerminalPanelState is the embedded terminal panel's render/model state.
type TerminalPanelState struct {
	// Visible shows the panel strip above the footer (terminal.toggle-panel).
	Visible bool
	// Focused reports keyboard focus inside the panel.
	Focused bool
	// Rows is the requested content row count. The panel's actual on-screen rows come
	// from the computed layout rect (may be clamped smaller).
	Rows int
	// Drawer paints live content; nil paints blank cells (always nil in Phase 1).
	Drawer TerminalDrawer
}

// drawTerminalPanel paints the terminal panel strip: content rows (blank, or painted by
// state.Drawer when set).
func drawTerminalPanel(screen tcell.Screen, rect Rect, state TerminalPanelState, styles theme.Theme) {
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	textStyle := styles.TerminalTextStyle()
	primitive.Fill(screen, primitive.Rect{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}, ' ', textStyle)
	if state.Drawer == nil {
		return
	}
	setCell := func(x, y int, r rune, style tcell.Style) {
		if x < 0 || y < 0 || x >= rect.Width || y >= rect.Height {
			return
		}
		screen.SetContent(rect.X+x, rect.Y+y, r, nil, style)
	}
	// Cursor placement for the live PTY view is wired in a later phase.
	_, _, _ = state.Drawer.DrawTo(setCell)
}
