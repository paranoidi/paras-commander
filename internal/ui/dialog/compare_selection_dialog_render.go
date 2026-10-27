package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawCompareSelectionDialog paints the compare selection dialog from its widget spec (see
// CompareSelectionView).
func DrawCompareSelectionDialog(screen tcell.Screen, layout Layout, state CompareSelectionDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, CompareSelectionView(state), state.Focus, styles)
}
