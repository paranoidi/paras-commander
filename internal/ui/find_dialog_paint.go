package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// FindDialogListRows returns the visible find-dialog list row count (matches DrawFindDialog).
func FindDialogListRows(layout Layout, showSearchSelectionsOption bool) int {
	_, _, listH, ok := dialog.FindDialogMetrics(layout, showSearchSelectionsOption)
	if !ok {
		return 4
	}
	return listH
}

// PaintFindDialog repaints the find dialog overlay without touching panels or the footer.
func PaintFindDialog(screen tcell.Screen, layout Layout, state FindDialogState, styles theme.Theme, showIcons bool) {
	if !state.Open {
		return
	}
	dialog.DrawFindDialog(screen, layout, state, styles, showIcons, FindListIconLeadingWidth(showIcons), PaintFindDialogRowIcon)
}
