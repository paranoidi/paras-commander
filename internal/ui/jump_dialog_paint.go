package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// PaintJumpDialog paints the jump-to-directory dialog of model (the full render and the partial
// repaint share it) and returns the area it painted (zero when nothing was).
func PaintJumpDialog(screen tcell.Screen, layout Layout, model *Model, styles theme.Theme) dialog.PaintedArea {
	st := &model.JumpDialog
	if !st.Open {
		return dialog.PaintedArea{}
	}
	marks := rowMarksResolver(model.PinnedItems, model.JobPathMarks)
	return dialog.DrawJumpDialog(screen, layout, *st, dialogRenderContext(*model, styles), PaintDirRowIcon, func(r dialog.JumpResult) dialog.RowMarks {
		return marks(st.PanelPath(r))
	})
}
