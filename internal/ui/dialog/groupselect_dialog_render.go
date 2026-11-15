package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawGroupSelectDialog renders the Select / Unselect group modal, or its saved-pattern picker /
// save prompt (Screen).
func DrawGroupSelectDialog(screen tcell.Screen, layout Layout, state GroupSelectState, styles theme.Theme) {
	s := state.Screen(layout, styles)
	dlg.Draw(screen, layout.Width, layout.Height, s.Spec, *s.Focus, styles)
}
