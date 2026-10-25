package dlg

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// BarFilledCols is how many of width cells a meter at frac fills: rounded, clamped, and at least
// one once started (frac > 0) so a running bar is never completely empty.
func BarFilledCols(width int, frac float64) int {
	if width <= 0 || frac <= 0 {
		return 0
	}
	filled := int(min(frac, 1)*float64(width) + 0.5)
	return min(max(filled, 1), width)
}

// drawBar paints a KindBar: fill / track carried by the cell background with the already fitted
// label overlaid (blank label cells stay plain bar).
func drawBar(screen tcell.Screen, r Rect, label string, frac float64, styles theme.Theme) {
	filled := BarFilledCols(r.Width, frac)
	labelRunes := []rune(label)
	for col := 0; col < r.Width; col++ {
		onFill := col < filled
		st := styles.DialogProgressTrack
		if onFill {
			st = styles.DialogProgressFill
		}
		ch := ' '
		if col < len(labelRunes) {
			ch = labelRunes[col]
			if ch != ' ' {
				st = styles.DialogProgressLabelOnBar(onFill)
			}
		}
		screen.SetContent(r.X+col, r.Y, ch, nil, st)
	}
}
