package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawCompareMergeDialog paints the compare merge modal from its widget spec (see CompareMergeView).
func DrawCompareMergeDialog(screen tcell.Screen, layout Layout, state CompareMergeDialogState, styles theme.Theme, userHomeDir string) {
	dlg.Draw(screen, layout.Width, layout.Height, CompareMergeView(state, userHomeDir), state.Focus, styles)
}
