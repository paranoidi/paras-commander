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
func drawFilePreviewSearchBar(screen tcell.Screen, rect Rect, field dialog.FileDialogField, styles theme.Theme) {
	if rect.Height < 1 || rect.Width < 1 {
		return
	}
	rowStyle, _ := styles.DialogInputPair(true)
	for col := rect.X; col < rect.X+rect.Width; col++ {
		screen.SetContent(col, rect.Y, ' ', nil, rowStyle)
	}
	labelW := runewidth.StringWidth(filePreviewSearchLabel)
	if labelW > rect.Width {
		labelW = rect.Width
	}
	_, rowBG, _ := rowStyle.Decompose()
	labelStyle := styles.DialogText.Background(rowBG)
	primitive.TextOverlay(screen, rect.X, rect.Y, labelW, filePreviewSearchLabel, labelStyle)
	inputX := rect.X + labelW
	inputW := rect.Width - labelW
	if inputW > 0 {
		dialog.DrawInputField(screen, inputX, rect.Y, inputW, field, true, styles)
	}
}
