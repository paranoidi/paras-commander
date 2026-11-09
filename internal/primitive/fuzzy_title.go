package primitive

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

// FuzzyQueryTitle paints "> query" in inputStyle on a panel's or dialog's top border, with a
// reversed caret at rune offset cursor when caret is set.
func FuzzyQueryTitle(screen tcell.Screen, x, y, w int, query string, cursor int, caret bool, inputStyle tcell.Style) {
	display := "> " + query
	if !caret {
		Text(screen, x, y, w, display, inputStyle)
		return
	}
	cursorCol := 2 + min(max(cursor, 0), utf8.RuneCountInString(query))
	if cursorCol >= utf8.RuneCountInString(display) {
		// Widen past end-of-text so the caret span lands on a real cell
		// (StyledText's trailing pad columns don't apply spans).
		display += " "
	}
	StyledText(screen, x, y, w, display, inputStyle, []Span{
		{Start: cursorCol, End: cursorCol + 1, Style: inputStyle.Reverse(true)},
	})
}
