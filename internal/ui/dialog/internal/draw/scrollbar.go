package draw

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// DrawDialogListScrollbar paints a themed vertical scroll indicator on a dialog's right
// border column for the row range [topY, topY+visible), when total exceeds visible (a no-op
// otherwise). borderStyle supplies the thumb-mode rail, and its background is what
// uiscrollbar.Draw paints the track/thumb on, so both read as part of the dialog border.
func DrawDialogListScrollbar(screen tcell.Screen, rect Rect, topY, visible, total, offset int, style uiscrollbar.Style, borderStyle tcell.Style, styles theme.Theme) {
	metrics, ok := uiscrollbar.ComputeMetrics(total, visible, offset)
	if !ok {
		return
	}
	uiscrollbar.Draw(uiscrollbar.DrawParams{
		Screen:     screen,
		X:          rect.X + rect.Width - 1,
		ListTopY:   topY,
		Visible:    visible,
		Metrics:    metrics,
		Style:      style,
		Active:     true,
		Blocked:    false,
		FrameStyle: borderStyle,
		Theme:      styles,
	})
}
