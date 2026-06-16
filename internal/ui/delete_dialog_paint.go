package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// PaintDeleteDialog repaints the delete confirmation dialog without touching panels or the footer.
func PaintDeleteDialog(
	screen tcell.Screen,
	layout Layout,
	state FileDialogState,
	styles theme.Theme,
	showIcons bool,
) {
	if !state.Open || state.DialogType != dialog.FileDialogDelete {
		return
	}
	dialog.DrawFileDialog(screen, layout, state, styles, showIcons, DialogListIconLeadingWidth(showIcons), PaintDeleteDialogRowIcon)
}
