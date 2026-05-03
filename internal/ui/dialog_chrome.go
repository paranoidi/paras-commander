package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// drawCenteredDialogTitle overwrites the top border row between corners with '─'
// fillers and the title centered, per AGENTS.md.
func drawCenteredDialogTitle(screen tcell.Screen, rect Rect, title string, titleStyle tcell.Style, edgeStyle tcell.Style) {
	innerWidth := rect.Width - 2
	if innerWidth < 1 {
		return
	}
	innerLeft := rect.X + 1
	topY := rect.Y

	tr := strings.TrimSpace(title)
	var titleRunes []rune
	if tr != "" {
		titleRunes = append(append(titleRunes, ' '), []rune(tr)...)
		titleRunes = append(titleRunes, ' ')
	}
	tlen := len(titleRunes)
	if tlen > innerWidth {
		titleRunes = titleRunes[:innerWidth]
		tlen = innerWidth
	}
	leftPad := (innerWidth - tlen) / 2

	col := innerLeft
	for i := range innerWidth {
		var r rune
		var st tcell.Style
		switch {
		case i < leftPad || i >= leftPad+tlen:
			r = '─'
			st = edgeStyle
		default:
			r = titleRunes[i-leftPad]
			st = titleStyle
		}
		screen.SetContent(col, topY, r, nil, st)
		col++
	}
}

// drawDialogHSeparator draws ├ ─ ─ ┤ between left and right border at y.
func drawDialogHSeparator(screen tcell.Screen, rect Rect, y int, borderStyle tcell.Style) {
	screen.SetContent(rect.X, y, '├', nil, borderStyle)
	for x := rect.X + 1; x < rect.X+rect.Width-1; x++ {
		screen.SetContent(x, y, '─', nil, borderStyle)
	}
	screen.SetContent(rect.X+rect.Width-1, y, '┤', nil, borderStyle)
}

// drawSimpleDialogInput paints a full-width input row with dialog input styles (no DialogSurface override)
// and shows focus with a reversed cell at the logical cursor (end of value), per AGENTS.md.
func drawSimpleDialogInput(screen tcell.Screen, x, y, width int, value string, focused bool, styles theme.Theme) {
	if width <= 0 {
		return
	}
	style, _ := styles.DialogInputPair(focused)
	display := primitive.TruncateRight(value, width)
	runes := []rune(display)

	cursorPos := utf8.RuneCountInString(value)
	if cursorPos > width-1 {
		cursorPos = width - 1
	}
	if cursorPos < 0 {
		cursorPos = 0
	}

	for i := 0; i < width; i++ {
		ch := ' '
		if i < len(runes) {
			ch = runes[i]
		}
		st := style
		if focused && i == cursorPos {
			st = style.Reverse(true)
		}
		screen.SetContent(x+i, y, ch, nil, st)
	}
}

// DialogButtonSpec describes one rendered dialog button (label, Alt shortcut, focus).
type DialogButtonSpec struct {
	Label    string
	Shortcut rune
	Focused  bool
}

// centeredDialogRect returns a rectangle of the given size centered in the layout.
// Width and height are clamped to the layout; coordinates are clamped to non-negative.
func centeredDialogRect(layout Layout, width, height int) Rect {
	if width > layout.Width {
		width = layout.Width
	}
	if height > layout.Height {
		height = layout.Height
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	x := (layout.Width - width) / 2
	y := (layout.Height - height) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return Rect{X: x, Y: y, Width: width, Height: height}
}

// drawDialogFrame clears the rect with dialog background, draws a border using
// DialogFrame on that background, and paints the title row per AGENTS.md.
// Returns borderStyle (for horizontal rules inside the dialog).
func drawDialogFrame(screen tcell.Screen, rect Rect, title string, styles theme.Theme) tcell.Style {
	primitive.Fill(screen, primitive.Rect(rect), ' ', styles.DialogSurface)
	_, dbg, _ := styles.DialogSurface.Decompose()
	borderStyle := styles.DialogFrame.Background(dbg)
	primitive.Box(screen, primitive.Rect(rect), borderStyle)

	bfg, _, _ := styles.DialogFrame.Decompose()
	titleStyle := styles.DialogTitle.Foreground(bfg).Background(dbg)
	drawCenteredDialogTitle(screen, rect, title, titleStyle, borderStyle)
	return borderStyle
}

// drawDialogButtonRowCentered draws a row of buttons with fixed gap, centered in rect at row y.
func drawDialogButtonRowCentered(screen tcell.Screen, rect Rect, y int, buttons []DialogButtonSpec, styles theme.Theme) {
	if len(buttons) == 0 {
		return
	}
	const gap = 2
	totalWidth := 0
	for i, b := range buttons {
		totalWidth += dialogButtonWidth(b.Label)
		if i > 0 {
			totalWidth += gap
		}
	}
	startX := rect.X + (rect.Width-totalWidth)/2
	if startX < rect.X+1 {
		startX = rect.X + 1
	}
	x := startX
	for i, b := range buttons {
		if i > 0 {
			x += gap
		}
		x += drawDialogButton(screen, x, y, b.Label, b.Shortcut, b.Focused, styles)
	}
}
