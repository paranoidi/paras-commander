package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawConfigDialog paints the Configuration modal, or one of its Yes/No sub-confirms (Screen).
func DrawConfigDialog(screen tcell.Screen, layout Layout, state ConfigDialogState, styles theme.Theme) {
	s := state.Screen(layout.Width)
	dlg.Draw(screen, layout.Width, layout.Height, s.Spec, *s.Focus, styles)
}

// Screen is the Configuration dialog's screen on top on a screen screenW wide: the "generate
// config.toml?" or "reset to defaults?" Yes/No confirm with its own focus, else the dialog.
func (st *ConfigDialogState) Screen(screenW int) DialogScreen {
	switch {
	case st.EditStubConfirm:
		return DialogScreen{ConfigEditStubView(), &st.EditStubConfirmFocus}
	case st.ResetDefaultsConfirm:
		return DialogScreen{ConfigResetDefaultsView(), &st.ResetDefaultsConfirmFocus}
	}
	return DialogScreen{ConfigDialogView(*st, screenW), &st.Focus}
}
