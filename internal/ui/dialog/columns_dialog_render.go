package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawColumnsDialog paints the Columns dialog from its widget spec (see ColumnsDialogView).
func DrawColumnsDialog(screen tcell.Screen, layout Layout, state ColumnsDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, ColumnsDialogView(state, layout.Height), state.Focus, styles)
}
