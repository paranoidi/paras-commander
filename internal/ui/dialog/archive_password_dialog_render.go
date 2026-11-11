package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawArchivePasswordDialog paints the encrypted-archive password prompt from its widget spec
// (see ArchivePasswordView).
func DrawArchivePasswordDialog(screen tcell.Screen, layout Layout, state ArchivePasswordDialogState, styles theme.Theme) {
	if !state.Open {
		return
	}
	dlg.Draw(screen, layout.Width, layout.Height, ArchivePasswordView(state), state.Focus, styles)
}
