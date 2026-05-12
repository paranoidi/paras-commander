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
func drawSimpleDialogInput(screen tcell.Screen, x, y, width int, value string, focused, invalid bool, styles theme.Theme) {
	if width <= 0 {
		return
	}
	style := styles.DialogInputBaseStyle(focused, invalid)
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

// EnsureScrollInputVisible adjusts scroll so that cursor is within [scroll, scroll+width-1]
// for an input rendered with width cells. cursor and length are rune counts.
// Returns the (possibly clamped) cursor and updated scroll values.
func EnsureScrollInputVisible(length, cursor, scroll, width int) (int, int) {
	if width < 1 {
		width = 1
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > length {
		cursor = length
	}
	if scroll < 0 {
		scroll = 0
	}
	maxScroll := length - width + 1
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if cursor < scroll {
		scroll = cursor
	} else if cursor >= scroll+width {
		scroll = cursor - width + 1
	}
	if scroll < 0 {
		scroll = 0
	}
	return cursor, scroll
}

// drawScrollingDialogInput paints a dialog input row with horizontal scrolling.
// The input renders the slice value[scroll : scroll+width] (rune-wise) and shows
// a reversed cell at cursor-scroll when focused. Overflow markers ('◀'/'▶') are
// drawn on the edge cells when content is hidden in that direction.
// Callers should ensure (cursor, scroll) are kept in range via EnsureScrollInputVisible.
func drawScrollingDialogInput(screen tcell.Screen, x, y, width int, value string, cursor, scroll int, focused, invalid bool, styles theme.Theme) {
	if width <= 0 {
		return
	}
	style := styles.DialogInputBaseStyle(focused, invalid)
	runes := []rune(value)
	length := len(runes)

	cursor, scroll = EnsureScrollInputVisible(length, cursor, scroll, width)

	for i := 0; i < width; i++ {
		idx := scroll + i
		ch := ' '
		if idx < length {
			ch = runes[idx]
		}
		st := style
		if focused && idx == cursor {
			st = style.Reverse(true)
		}
		screen.SetContent(x+i, y, ch, nil, st)
	}

	// Overflow markers: only when not hiding the cursor cell and there's hidden content.
	if scroll > 0 && (!focused || cursor != scroll) {
		screen.SetContent(x, y, '◀', nil, style)
	}
	if scroll+width < length && (!focused || cursor != scroll+width-1) {
		screen.SetContent(x+width-1, y, '▶', nil, style)
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
