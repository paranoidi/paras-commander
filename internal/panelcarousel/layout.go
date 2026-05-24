package panelcarousel

import (
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

// LayoutFits reports whether rect has enough interior width for three carousel columns.
func LayoutFits(rect geom.Rect, minPanelInner, minCol int) bool {
	if minPanelInner <= 0 {
		minPanelInner = config.MinCarouselPanelInnerWidth
	}
	if minCol <= 0 {
		minCol = config.MinCarouselColumnWidth
	}
	inner := rect.Width - 2
	if inner < minPanelInner {
		return false
	}
	return inner >= 3*minCol
}

// MinActiveWidthPercent returns the minimum active-column width share (1–100) so the
// active panel interior fits three carousel columns at the given total terminal width.
func MinActiveWidthPercent(totalWidth int) int {
	if totalWidth <= 0 {
		return config.DefaultPanelZoomActivePercent
	}
	need := config.MinCarouselPanelInnerWidth + 2 // panel column width including frame sides
	pct := (need*100 + totalWidth - 1) / totalWidth
	if pct < 50 {
		pct = 50
	}
	if pct > 95 {
		pct = 95
	}
	return pct
}

// SplitColumns divides the panel interior (below title + header rows) into three list columns.
func SplitColumns(rect geom.Rect) [3]geom.Rect {
	var cols [3]geom.Rect
	innerX := rect.X + 1
	innerW := rect.Width - 2
	if innerW < 3 {
		return cols
	}
	w0 := innerW / 3
	w1 := innerW / 3
	w2 := innerW - w0 - w1
	listY := rect.Y + 2
	listH := geom.PanelListRows(rect)
	cols[0] = geom.Rect{X: innerX, Y: listY, Width: w0, Height: listH}
	cols[1] = geom.Rect{X: innerX + w0, Y: listY, Width: w1, Height: listH}
	cols[2] = geom.Rect{X: innerX + w0 + w1, Y: listY, Width: w2, Height: listH}
	return cols
}
