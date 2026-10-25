package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawConfigDialog paints the Configuration modal, or one of its Yes/No sub-confirms, from the
// widget specs (see ConfigDialogView, ConfigEditStubView, ConfigResetDefaultsView).
func DrawConfigDialog(screen tcell.Screen, layout Layout, state ConfigDialogState, styles theme.Theme) {
	switch {
	case state.EditStubConfirm:
		dlg.Draw(screen, layout.Width, layout.Height, ConfigEditStubView(), state.EditStubConfirmFocus, styles)
	case state.ResetDefaultsConfirm:
		dlg.Draw(screen, layout.Width, layout.Height, ConfigResetDefaultsView(), state.ResetDefaultsConfirmFocus, styles)
	default:
		dlg.Draw(screen, layout.Width, layout.Height, ConfigDialogView(state, layout.Width), state.Focus, styles)
	}
}
