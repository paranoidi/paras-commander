package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawDedupReturnDialog paints the "kept results" modal from its widget spec (see
// DedupReturnView).
func DrawDedupReturnDialog(screen tcell.Screen, layout Layout, state DedupReturnDialogState, styles theme.Theme, spec dlg.Spec) {
	if !state.Open {
		return
	}
	dlg.Draw(screen, layout.Width, layout.Height, spec, state.Focus, styles)
}
