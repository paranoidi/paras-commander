package draw

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/lineedit"
)

// DrawCenteredDialogTitle overwrites the top border row between corners with '─'
// fillers and the title centered, per AGENTS.md.
func DrawCenteredDialogTitle(screen tcell.Screen, rect Rect, title string, titleStyle tcell.Style, edgeStyle tcell.Style) {
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

// DrawDialogHSeparator draws ├ ─ ─ ┤ between left and right border at y.
func DrawDialogHSeparator(screen tcell.Screen, rect Rect, y int, borderStyle tcell.Style) {
	screen.SetContent(rect.X, y, '├', nil, borderStyle)
	for x := rect.X + 1; x < rect.X+rect.Width-1; x++ {
		screen.SetContent(x, y, '─', nil, borderStyle)
	}
	screen.SetContent(rect.X+rect.Width-1, y, '┤', nil, borderStyle)
}

// DrawSimpleDialogInput paints a full-width input row with dialog input styles (no DialogSurface override)
// and shows focus with a reversed cell at the logical cursor (end of value), per AGENTS.md.
func DrawSimpleDialogInput(screen tcell.Screen, x, y, width int, value string, focused, invalid bool, styles theme.Theme) {
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

// ScrollingInputLayout reserves screen columns for ◀/▶ overflow markers.
type ScrollingInputLayout struct {
	TextCols int // rune columns painted between markers
	LeftPad  int // screen columns before text (0 or 1)
	RightPad int // screen columns after text (0 or 1)
}

// ScrollContentLen is the horizontal extent for overflow markers and scroll limits.
// When the caret is after the last rune (cursor == valueLen), one extra column is
// counted so trailing text is not hidden behind the empty caret cell.
func ScrollContentLen(valueLen, cursor int) int {
	if cursor > valueLen {
		cursor = valueLen
	}
	if valueLen > 0 && cursor == valueLen {
		return valueLen + 1
	}
	return valueLen
}

// ScrollingInputLayoutFor computes text column reservation for contentLen runes at scroll.
func ScrollingInputLayoutFor(scroll, width, contentLen int) ScrollingInputLayout {
	if width < 1 {
		width = 1
	}
	leftPad := 0
	if scroll > 0 {
		leftPad = 1
	}
	tentative := width - leftPad
	rightPad := 0
	if scroll+tentative < contentLen {
		rightPad = 1
	}
	textCols := width - leftPad - rightPad
	if textCols < 1 {
		textCols = 1
	}
	return ScrollingInputLayout{TextCols: textCols, LeftPad: leftPad, RightPad: rightPad}
}

func ensureScrollInputVisible(valueLen, cursor, scroll, width, contentLen int) (int, int) {
	if width < 1 {
		width = 1
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > valueLen {
		cursor = valueLen
	}
	if scroll < 0 {
		scroll = 0
	}
	lay := ScrollingInputLayoutFor(scroll, width, contentLen)
	maxScroll := contentLen - lay.TextCols
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if cursor < scroll {
		scroll = cursor
	} else if cursor >= scroll+lay.TextCols {
		scroll = cursor - lay.TextCols + 1
	}
	if scroll < 0 {
		scroll = 0
	}
	return cursor, scroll
}

// EnsureScrollInputVisible adjusts scroll so the caret stays in the text area between
// overflow markers. cursor and length are rune counts.
func EnsureScrollInputVisible(length, cursor, scroll, width int) (int, int) {
	return ensureScrollInputVisible(length, cursor, scroll, width, ScrollContentLen(length, cursor))
}

// AdjustScrollForCompletion updates scroll so the caret stays visible. When suffixLen > 0
// and the ghost suffix does not fit, scroll increases by suffixLen (viewport shifts left).
// When suffixLen is 0, scroll is not decreased so ignoring a suggestion does not snap back.
func AdjustScrollForCompletion(valueLen, cursor, scroll, width, suffixLen int) (int, int) {
	if width < 1 {
		width = 1
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > valueLen {
		cursor = valueLen
	}
	if scroll < 0 {
		scroll = 0
	}
	if suffixLen > 0 {
		if valueLen <= width {
			return cursor, 0
		}
		combinedLen := valueLen + suffixLen
		cursor, scroll = ensureScrollInputVisible(valueLen, cursor, scroll, width, combinedLen)
		lay := ScrollingInputLayoutFor(scroll, width, combinedLen)
		suffixEnd := cursor + suffixLen
		if suffixEnd > scroll+lay.TextCols {
			scroll += suffixLen
			maxScroll := combinedLen - lay.TextCols
			if maxScroll < 0 {
				maxScroll = 0
			}
			if scroll > maxScroll {
				scroll = maxScroll
			}
		}
		return cursor, scroll
	}
	if cursor < scroll {
		scroll = cursor
	}
	lay := ScrollingInputLayoutFor(scroll, width, ScrollContentLen(valueLen, cursor))
	if cursor >= scroll+lay.TextCols {
		scroll = cursor - lay.TextCols + 1
		if scroll < 0 {
			scroll = 0
		}
	}
	return cursor, scroll
}

// EnsurePathInputScroll keeps the caret visible in a path-shaped input row.
// When the committed value fits in width, scroll is always 0 (ghost text may clip on the right).
func EnsurePathInputScroll(valueLen, cursor, scroll, width, suffixLen int) (int, int) {
	if width < 1 {
		width = 1
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > valueLen {
		cursor = valueLen
	}
	if ScrollContentLen(valueLen, cursor) <= width {
		return cursor, 0
	}
	return AdjustScrollForCompletion(valueLen, cursor, scroll, width, suffixLen)
}

// ShouldPreemptiveScrollRevealOnErase reports whether the next backspace/delete removes the
// rightmost visible rune in the viewport (value plus ghost completion at the caret).
func ShouldPreemptiveScrollRevealOnErase(valueLen, cursor, scroll, width, suffixLen int, isBackspace bool) bool {
	if width < 1 || scroll < 0 {
		return false
	}
	var deleteIdx int
	if isBackspace {
		if cursor <= 0 {
			return false
		}
		deleteIdx = cursor - 1
		if deleteIdx < scroll {
			return true
		}
	} else {
		if cursor >= valueLen {
			return false
		}
		deleteIdx = cursor
		if deleteIdx < scroll {
			return true
		}
	}
	combinedLen := valueLen + suffixLen
	if combinedLen == 0 || valueLen == 0 {
		return false
	}
	windowEnd := scroll + width - 1
	if windowEnd >= combinedLen {
		windowEnd = combinedLen - 1
	}
	lastValueVisible := windowEnd
	if lastValueVisible >= valueLen {
		lastValueVisible = valueLen - 1
	}
	if lastValueVisible < scroll {
		return false
	}
	return deleteIdx == lastValueVisible
}

// AdjustScrollRevealOnErase moves the viewport right (decreases scroll) after deletions in a
// scrolled field. When value plus completion suffix fits in width, scroll becomes 0.
// Otherwise scroll steps right by one readline word boundary at the first visible index.
func AdjustScrollRevealOnErase(value string, cursor, scroll, width, suffixLen int) (int, int) {
	if width < 1 {
		width = 1
	}
	valueRunes := []rune(value)
	valueLen := len(valueRunes)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > valueLen {
		cursor = valueLen
	}
	if scroll <= 0 {
		return cursor, 0
	}
	if ScrollContentLen(valueLen, cursor) <= width {
		return cursor, 0
	}
	newScroll := lineedit.BackwardWordIndex(valueRunes, scroll)
	if newScroll >= scroll {
		newScroll = scroll - 1
	}
	if newScroll < 0 {
		newScroll = 0
	}
	lay := ScrollingInputLayoutFor(newScroll, width, ScrollContentLen(valueLen, cursor))
	maxScroll := ScrollContentLen(valueLen, cursor) - lay.TextCols
	if maxScroll < 0 {
		maxScroll = 0
	}
	if newScroll > maxScroll {
		newScroll = maxScroll
	}
	return cursor, newScroll
}

// PaintScrollingInputContent paints a horizontally scrolling input slice with optional
// ghost completion at the caret (value[:cursor]+suffix+value[cursor:]).
// Ghost text always uses dialog input placeholder styling, not error styles.
// textFocused controls caret reverse-video highlighting.
func PaintScrollingInputContent(
	screen tcell.Screen, x, y, width int,
	value, completionSuffix string,
	cursor, scroll int,
	textFocused, invalid, focused bool,
	styles theme.Theme,
) (int, int) {
	if width <= 0 {
		return cursor, scroll
	}
	committedStyle := styles.DialogInputBaseStyle(focused, invalid)
	markerStyle := styles.DialogInputBaseStyle(focused, false)
	_, ghostStyle := styles.DialogInputPair(focused)
	valueRunes := []rune(value)
	suffixRunes := []rune(completionSuffix)
	valueLen := len(valueRunes)
	combinedLen := valueLen + len(suffixRunes)

	layoutLen := combinedLen
	if extra := ScrollContentLen(valueLen, cursor); extra > layoutLen {
		layoutLen = extra
	}
	if layoutLen <= width {
		scroll = 0
	} else {
		cursor, scroll = ensureScrollInputVisible(valueLen, cursor, scroll, width, layoutLen)
	}
	lay := ScrollingInputLayoutFor(scroll, width, layoutLen)

	for i := 0; i < lay.TextCols; i++ {
		idx := scroll + i
		ch := ' '
		ghost := false
		if idx < combinedLen {
			switch {
			case idx < cursor:
				ch = valueRunes[idx]
			case idx < cursor+len(suffixRunes):
				ch = suffixRunes[idx-cursor]
				ghost = true
			default:
				ch = valueRunes[cursor+(idx-cursor-len(suffixRunes))]
			}
		}
		st := committedStyle
		if ghost {
			st = ghostStyle
		}
		if textFocused && idx == cursor {
			if ghost {
				st = ghostStyle.Reverse(true)
			} else {
				st = committedStyle.Reverse(true)
			}
		}
		screen.SetContent(x+lay.LeftPad+i, y, ch, nil, st)
	}

	if lay.LeftPad > 0 {
		screen.SetContent(x, y, '◀', nil, markerStyle)
	}
	if lay.RightPad > 0 {
		screen.SetContent(x+width-1, y, '▶', nil, markerStyle)
	}
	return cursor, scroll
}

// DrawScrollingDialogInput paints a dialog input row with horizontal scrolling.
func DrawScrollingDialogInput(screen tcell.Screen, x, y, width int, value string, cursor, scroll int, completionSuffix string, focused, invalid bool, styles theme.Theme) {
	if width <= 0 {
		return
	}
	PaintScrollingInputContent(screen, x, y, width, value, completionSuffix, cursor, scroll, focused, invalid, focused, styles)
}

// DialogButtonSpec describes one rendered dialog button (label, Alt shortcut, focus).
type DialogButtonSpec struct {
	Label    string
	Shortcut rune
	Focused  bool
}

// CenteredDialogRect returns a rectangle of the given size centered in the layout.
// Width and height are clamped to the layout; coordinates are clamped to non-negative.
func CenteredDialogRect(layout Layout, width, height int) Rect {
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

// DrawDialogFrame clears the rect with dialog background, draws a border using
// DialogFrame on that background, and paints the title row per AGENTS.md.
// Returns borderStyle (for horizontal rules inside the dialog).
func DrawDialogFrame(screen tcell.Screen, rect Rect, title string, styles theme.Theme) tcell.Style {
	primitive.Fill(screen, primitive.Rect{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}, ' ', styles.DialogSurface)
	_, dbg, _ := styles.DialogSurface.Decompose()
	borderStyle := styles.DialogFrame.Background(dbg)
	primitive.Box(screen, primitive.Rect{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}, borderStyle)

	bfg, _, _ := styles.DialogFrame.Decompose()
	titleStyle := styles.DialogTitle.Foreground(bfg).Background(dbg)
	DrawCenteredDialogTitle(screen, rect, title, titleStyle, borderStyle)
	return borderStyle
}

// DrawDialogButtonRowCentered draws a row of buttons with fixed gap, centered in rect at row y.
func DrawDialogButtonRowCentered(screen tcell.Screen, rect Rect, y int, buttons []DialogButtonSpec, styles theme.Theme) {
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
		x += DrawDialogButton(screen, x, y, b.Label, b.Shortcut, b.Focused, styles)
	}
}
