package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// drawPanelBottomSelectionSize paints the selection count/size label centered on the file panel bottom row.
func drawPanelBottomSelectionSize(
	screen tcell.Screen,
	rect Rect,
	_ int,
	ctx PanelBottomIndicatorContext,
) {
	if ctx.SelectionSizeLabel == "" || ctx.SelectionSizeWidth <= 0 || ctx.SelectionSizeCenterStart <= 0 {
		return
	}
	y := rect.Y + rect.Height - 1
	style := ctx.Styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelectionSize, ctx.FileListActive, ctx.ChromeBlocked)
	primitive.TextOverlay(screen, ctx.SelectionSizeCenterStart, y, ctx.SelectionSizeWidth, ctx.SelectionSizeLabel, style)
}
