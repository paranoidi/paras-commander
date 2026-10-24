package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawStashRestoreDialog paints the stash restore dialog from its widget spec (see
// StashRestoreView).
func DrawStashRestoreDialog(screen tcell.Screen, layout Layout, state StashRestoreDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, StashRestoreView(state), state.Focus, styles)
}
