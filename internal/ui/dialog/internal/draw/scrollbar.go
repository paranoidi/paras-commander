package draw

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// DrawDialogListScrollbar paints a themed vertical scroll indicator on a dialog's right
// border column for the row range [topY, topY+visible), when total exceeds visible (a no-op
// otherwise). It forces the track/thumb background to the dialog's own DialogSurface color:
// panel.scrollbar.track/thumb only define a foreground in the theme, so left as-is they'd
// leak the terminal default background instead of matching the dialog. borderStyle supplies
// the thumb-mode rail so untouched cells still read as the dialog border.
func DrawDialogListScrollbar(screen tcell.Screen, rect Rect, topY, visible, total, offset int, style uiscrollbar.Style, borderStyle tcell.Style, styles theme.Theme) {
	metrics, ok := uiscrollbar.ComputeMetrics(total, visible, offset)
	if !ok {
		return
	}
	_, dbg, _ := styles.DialogSurface.Decompose()
	scrollbarTheme := styles
	scrollbarTheme.PanelScrollbarTrack = styles.PanelScrollbarTrack.Background(dbg)
	scrollbarTheme.PanelScrollbarThumb = styles.PanelScrollbarThumb.Background(dbg)
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
		Theme:      scrollbarTheme,
	})
}
