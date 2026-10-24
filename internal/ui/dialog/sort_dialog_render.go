package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawSortDialog paints the sort dialog from its widget spec (see SortDialogView).
func DrawSortDialog(screen tcell.Screen, layout Layout, state SortDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, SortDialogView(state), state.Focus, styles)
}
