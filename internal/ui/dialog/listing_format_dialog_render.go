package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawListingFormatDialog paints the listing format dialog from its widget spec (see
// ListingFormatView).
func DrawListingFormatDialog(screen tcell.Screen, layout Layout, state ListingFormatDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, ListingFormatView(state), state.Focus, styles)
}
