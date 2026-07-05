package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// PaintFileDialog repaints any file dialog (rename, mkdir, copy/move, mass rename,
// delete, …) without touching panels or the footer. It draws through the same
// dialog.DrawFileDialog path as the full render, which dispatches by DialogType,
// so one helper covers every file-dialog type. The delete row-icon painter is
// only invoked for the delete list and is harmless for other types.
func PaintFileDialog(
	screen tcell.Screen,
	layout Layout,
	state dialog.FileDialogState,
	styles theme.Theme,
	showIcons bool,
) {
	if !state.Open {
		return
	}
	dialog.DrawFileDialog(screen, layout, state, styles, showIcons, DialogListIconLeadingWidth(showIcons), PaintDeleteDialogRowIcon)
}
