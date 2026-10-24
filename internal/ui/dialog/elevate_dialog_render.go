package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawElevateDialog paints the "retry as root" prompt from its widget spec (see ElevateView).
func DrawElevateDialog(screen tcell.Screen, layout Layout, state ElevateDialogState, styles theme.Theme) {
	if !state.Open {
		return
	}
	dlg.Draw(screen, layout.Width, layout.Height, ElevateView(state), state.Focus, styles)
}
