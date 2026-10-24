package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawQuitConfirmDialog paints the quit confirmation from its widget spec (see QuitConfirmView).
func DrawQuitConfirmDialog(screen tcell.Screen, layout Layout, state QuitConfirmState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, QuitConfirmView(state), state.Focus, styles)
}
