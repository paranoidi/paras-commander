package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawDebounceCalibrateDialog paints the calibrate dialog from its widget spec (see
// DebounceCalibrateView).
func DrawDebounceCalibrateDialog(screen tcell.Screen, layout Layout, state DebounceCalibrateDialogState, styles theme.Theme) {
	if !state.Open {
		return
	}
	dlg.Draw(screen, layout.Width, layout.Height, DebounceCalibrateView(state), state.Focus, styles)
}
