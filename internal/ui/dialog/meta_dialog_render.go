package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawMetaDialog paints the meta column picker from its widget spec (see MetaDialogView).
func DrawMetaDialog(screen tcell.Screen, layout Layout, state MetaDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, MetaDialogView(state), state.Focus, styles)
}
