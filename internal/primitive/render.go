package primitive

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// Rect describes a terminal region for low-level drawing.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Span applies a style to a half-open rune range.
type Span struct {
	Start int
	End   int
	Style tcell.Style
}

// BorderGlyphs is the set of box-drawing runes used to paint a frame's corners and edges.
type BorderGlyphs struct {
	TopLeft     rune
	TopRight    rune
	BottomLeft  rune
	BottomRight rune
	Horizontal  rune
	Vertical    rune
}

// SharpBorder is the square-corner box style used by file panels and most dialogs.
var SharpBorder = BorderGlyphs{TopLeft: '┌', TopRight: '┐', BottomLeft: '└', BottomRight: '┘', Horizontal: '─', Vertical: '│'}

// RoundedBorder is the rounded-corner box style (e.g. modal dialog frames).
var RoundedBorder = BorderGlyphs{TopLeft: '╭', TopRight: '╮', BottomLeft: '╰', BottomRight: '╯', Horizontal: '─', Vertical: '│'}

// Box draws a single-line box border inside rect using glyphs. Each cell is written once.
func Box(screen tcell.Screen, rect Rect, style tcell.Style, glyphs BorderGlyphs) {
	if rect.Width <= 1 || rect.Height <= 1 {
		Fill(screen, rect, ' ', style)
		return
	}
	// Top row
	screen.SetContent(rect.X, rect.Y, glyphs.TopLeft, nil, style)
	for x := rect.X + 1; x < rect.X+rect.Width-1; x++ {
		screen.SetContent(x, rect.Y, glyphs.Horizontal, nil, style)
	}
	screen.SetContent(rect.X+rect.Width-1, rect.Y, glyphs.TopRight, nil, style)
	// Side columns
	for y := rect.Y + 1; y < rect.Y+rect.Height-1; y++ {
		screen.SetContent(rect.X, y, glyphs.Vertical, nil, style)
		screen.SetContent(rect.X+rect.Width-1, y, glyphs.Vertical, nil, style)
	}
	// Bottom row
	screen.SetContent(rect.X, rect.Y+rect.Height-1, glyphs.BottomLeft, nil, style)
	for x := rect.X + 1; x < rect.X+rect.Width-1; x++ {
		screen.SetContent(x, rect.Y+rect.Height-1, glyphs.Horizontal, nil, style)
	}
	screen.SetContent(rect.X+rect.Width-1, rect.Y+rect.Height-1, glyphs.BottomRight, nil, style)
}

// Fill writes ch across the entire rect.
func Fill(screen tcell.Screen, rect Rect, ch rune, style tcell.Style) {
	for y := rect.Y; y < rect.Y+rect.Height; y++ {
		for x := rect.X; x < rect.X+rect.Width; x++ {
			screen.SetContent(x, y, ch, nil, style)
		}
	}
}

// Text writes a clipped, space-padded string at the target row.
func Text(screen tcell.Screen, x, y, width int, text string, style tcell.Style) {
	paintText(screen, x, y, width, text, func(int, int) tcell.Style { return style }, true)
}

// StyledText writes clipped, space-padded text with styled spans.
func StyledText(screen tcell.Screen, x, y, width int, text string, style tcell.Style, spans []Span) {
	paintText(screen, x, y, width, text, func(runeIdx, _ int) tcell.Style {
		return styleAt(runeIdx, style, spans)
	}, true)
}

// StyledTextCellwise writes clipped, space-padded text where each column's base style comes from cellStyle before span overlays.
func StyledTextCellwise(screen tcell.Screen, x, y, width int, text string, cellStyle func(column int) tcell.Style, spans []Span) {
	paintText(screen, x, y, width, text, func(runeIdx, cell int) tcell.Style {
		return styleAt(runeIdx, cellStyle(cell), spans)
	}, true)
}

// TextOverlay writes clipped text without clearing the remaining cells.
func TextOverlay(screen tcell.Screen, x, y, width int, text string, style tcell.Style) {
	paintText(screen, x, y, width, text, func(int, int) tcell.Style { return style }, false)
}

func paintText(screen tcell.Screen, x, y, width int, text string, styleFor func(runeIdx, cell int) tcell.Style, pad bool) {
	if width <= 0 {
		return
	}
	text = TruncateRight(text, width)
	cell := 0
	runeIdx := 0
	var pending rune
	var pendingComb []rune
	var pendingStyle tcell.Style
	flush := func() {
		if pending == 0 {
			return
		}
		rw := runewidth.RuneWidth(pending)
		if rw < 1 {
			rw = 1
		}
		if cell+rw > width {
			pending = 0
			pendingComb = nil
			return
		}
		screen.SetContent(x+cell, y, pending, pendingComb, pendingStyle)
		cell += rw
		pending = 0
		pendingComb = nil
	}
	for _, r := range text {
		if runewidth.RuneWidth(r) == 0 && pending != 0 {
			pendingComb = append(pendingComb, r)
			runeIdx++
			continue
		}
		flush()
		pending = r
		pendingStyle = styleFor(runeIdx, cell)
		runeIdx++
	}
	flush()
	if pad {
		for cell < width {
			screen.SetContent(x+cell, y, ' ', nil, styleFor(-1, cell))
			cell++
		}
	}
}

func styleAt(column int, fallback tcell.Style, spans []Span) tcell.Style {
	for _, span := range spans {
		if column >= span.Start && column < span.End {
			return span.Style
		}
	}
	return fallback
}

// Ellipsis is the single-cell overflow marker used when shortening display text.
const Ellipsis = '…'

// TruncateRight clips value to width terminal cells, using Ellipsis as an overflow marker.
func TruncateRight(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(value) <= width {
		return value
	}
	if width == 1 {
		return firstCell(value)
	}
	return runewidth.Truncate(value, width, string(Ellipsis))
}

// TruncateMiddle clips value to width terminal cells, preserving both ends when possible.
func TruncateMiddle(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(value) <= width {
		return value
	}
	if width <= 3 {
		return takeCells(value, width)
	}
	prefix := (width - 1) / 2
	suffix := width - prefix - 1
	return takeCells(value, prefix) + string(Ellipsis) + takeCellsRight(value, suffix)
}

func firstCell(value string) string {
	for _, r := range value {
		if runewidth.RuneWidth(r) == 0 {
			continue
		}
		if runewidth.RuneWidth(r) > 1 {
			return string(Ellipsis)
		}
		return string(r)
	}
	return string(Ellipsis)
}

func takeCells(value string, width int) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	for _, r := range value {
		rw := runewidth.RuneWidth(r)
		if rw == 0 {
			if used == 0 {
				continue
			}
			b.WriteRune(r)
			continue
		}
		if used+rw > width {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String()
}

func takeCellsRight(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	used := 0
	start := len(runes)
	for i := len(runes) - 1; i >= 0; i-- {
		rw := runewidth.RuneWidth(runes[i])
		if rw == 0 {
			start = i
			continue
		}
		if used+rw > width {
			break
		}
		used += rw
		start = i
	}
	return string(runes[start:])
}

func Repeat(value string, count int) string {
	if count <= 0 {
		return ""
	}
	return strings.Repeat(value, count)
}
