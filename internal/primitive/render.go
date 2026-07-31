package primitive

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
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
	if width <= 0 {
		return
	}

	text = TruncateRight(text, width)
	column := 0
	for _, r := range text {
		if column >= width {
			break
		}
		screen.SetContent(x+column, y, r, nil, style)
		column++
	}
	for column < width {
		screen.SetContent(x+column, y, ' ', nil, style)
		column++
	}
}

// StyledText writes clipped, space-padded text with styled spans.
func StyledText(screen tcell.Screen, x, y, width int, text string, style tcell.Style, spans []Span) {
	if width <= 0 {
		return
	}

	text = TruncateRight(text, width)
	runes := []rune(text)
	column := 0
	for ; column < len(runes) && column < width; column++ {
		screen.SetContent(x+column, y, runes[column], nil, styleAt(column, style, spans))
	}
	for column < width {
		screen.SetContent(x+column, y, ' ', nil, style)
		column++
	}
}

// StyledTextCellwise writes clipped, space-padded text where each column's base style comes from cellStyle before span overlays.
func StyledTextCellwise(screen tcell.Screen, x, y, width int, text string, cellStyle func(column int) tcell.Style, spans []Span) {
	if width <= 0 {
		return
	}

	text = TruncateRight(text, width)
	runes := []rune(text)
	column := 0
	for ; column < len(runes) && column < width; column++ {
		base := cellStyle(column)
		screen.SetContent(x+column, y, runes[column], nil, styleAt(column, base, spans))
	}
	for column < width {
		screen.SetContent(x+column, y, ' ', nil, cellStyle(column))
		column++
	}
}

// TextOverlay writes clipped text without clearing the remaining cells.
func TextOverlay(screen tcell.Screen, x, y, width int, text string, style tcell.Style) {
	if width <= 0 {
		return
	}

	text = TruncateRight(text, width)
	column := 0
	for _, r := range text {
		if column >= width {
			break
		}
		screen.SetContent(x+column, y, r, nil, style)
		column++
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

// TruncateRight clips value to width runes, using Ellipsis as an overflow marker.
func TruncateRight(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= width {
		return value
	}
	runes := []rune(value)
	if width == 1 {
		return string(runes[:1])
	}
	return string(runes[:width-1]) + string(Ellipsis)
}

// TruncateMiddle clips value to width runes, preserving both ends when possible.
func TruncateMiddle(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= width {
		return value
	}
	runes := []rune(value)
	if width <= 3 {
		return string(runes[:width])
	}
	prefix := (width - 1) / 2
	suffix := width - prefix - 1
	return string(runes[:prefix]) + string(Ellipsis) + string(runes[len(runes)-suffix:])
}

func Repeat(value string, count int) string {
	if count <= 0 {
		return ""
	}
	return strings.Repeat(value, count)
}
