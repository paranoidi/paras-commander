package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawFilterDialog renders the panel Filter modal, or its picker / save prompt (Screen).
func DrawFilterDialog(screen tcell.Screen, layout Layout, state FilterDialogState, styles theme.Theme) {
	s := state.Screen(layout, styles)
	dlg.Draw(screen, layout.Width, layout.Height, s.Spec, *s.Focus, styles)
}
