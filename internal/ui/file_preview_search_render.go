package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

const filePreviewSearchLabel = "search: "

// drawFilePreviewSearchBar paints the incremental search query row directly above the footer.
// noMatch styles the whole row with styles.FuzzyInputNomatch, matching the file-list fuzzy
// filter's no-results styling, when the query has no matches in the previewed text.
func drawFilePreviewSearchBar(screen tcell.Screen, rect Rect, field dialog.FileDialogField, noMatch bool, styles theme.Theme) {
	if rect.Height < 1 || rect.Width < 1 {
		return
	}
	rowStyle, _ := styles.DialogInputPair(true)
	if noMatch {
		rowStyle = styles.FuzzyInputNomatch
	}
	for col := rect.X; col < rect.X+rect.Width; col++ {
		screen.SetContent(col, rect.Y, ' ', nil, rowStyle)
	}
	labelW := min(runewidth.StringWidth(filePreviewSearchLabel), rect.Width)
	labelStyle := rowStyle
	if !noMatch {
		_, rowBG, _ := rowStyle.Decompose()
		labelStyle = styles.DialogText.Background(rowBG)
	}
	primitive.TextOverlay(screen, rect.X, rect.Y, labelW, filePreviewSearchLabel, labelStyle)
	inputX := rect.X + labelW
	inputW := rect.Width - labelW
	if inputW <= 0 {
		return
	}
	if !noMatch {
		dialog.DrawInputField(screen, inputX, rect.Y, inputW, field, true, styles)
		return
	}
	display := field.Value
	cursorCol := field.Cursor
	if cursorCol >= len([]rune(display)) {
		display += " "
	}
	primitive.StyledText(screen, inputX, rect.Y, inputW, display, rowStyle, []primitive.Span{
		{Start: cursorCol, End: cursorCol + 1, Style: rowStyle.Reverse(true)},
	})
}
