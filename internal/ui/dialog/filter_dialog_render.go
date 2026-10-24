package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawFilterDialog renders the panel Filter modal from its widget spec (see FilterDialogView).
func DrawFilterDialog(screen tcell.Screen, layout Layout, state FilterDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, FilterDialogView(state, layout, styles), state.Focus, styles)
}
