package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawGroupSelectDialog renders the Select / Unselect group modal from its widget spec (see
// GroupSelectView).
func DrawGroupSelectDialog(screen tcell.Screen, layout Layout, state GroupSelectState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, GroupSelectView(state, layout, styles), state.Focus, styles)
}
