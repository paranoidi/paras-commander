package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawHostKeyDialog paints the host key prompt from its widget spec (see HostKeyView).
func DrawHostKeyDialog(screen tcell.Screen, layout Layout, state HostKeyDialogState, styles theme.Theme) {
	if !state.Open {
		return
	}
	dlg.Draw(screen, layout.Width, layout.Height, HostKeyView(state), state.Focus, styles)
}
