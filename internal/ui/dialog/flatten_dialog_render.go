package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// DrawFlattenDialog paints the flatten confirmation modal from its widget spec (see FlattenView),
// then the destination's completion dropdown anchored under the input row.
func DrawFlattenDialog(screen tcell.Screen, layout Layout, state FlattenDialogState, styles theme.Theme, scrollbarStyle uiscrollbar.Style) {
	spec := FlattenView(state)
	dlg.Draw(screen, layout.Width, layout.Height, spec, state.Focus, styles)
	p, ok := dlg.Place(layout.Width, layout.Height, spec)
	if !ok {
		return
	}
	if in, ok := p.Item(FlattenDest); ok {
		drawPathCompletionDropdown(screen, in.Rect.X, in.Rect.Y+1, state.Destination.Scroll, state.Destination.Completion, scrollbarStyle, styles)
	}
}
