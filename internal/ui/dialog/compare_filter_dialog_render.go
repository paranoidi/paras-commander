package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawCompareFilterDialog paints the compare category filter picker modal from its widget spec
// (see CompareFilterView).
func DrawCompareFilterDialog(screen tcell.Screen, layout Layout, state CompareFilterDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, CompareFilterView(state), state.Focus, styles)
}
