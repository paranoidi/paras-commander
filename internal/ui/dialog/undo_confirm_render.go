package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawUndoConfirmDialog paints the undo confirmation from its widget spec (see UndoConfirmView).
func DrawUndoConfirmDialog(screen tcell.Screen, layout Layout, state UndoConfirmState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, UndoConfirmView(state), state.Focus, styles)
}
