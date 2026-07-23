package panelcarousel

import (
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

// LayoutFits reports whether rect has enough interior width for carousel layout.
func LayoutFits(rect geom.Rect, layout Layout, showChild bool) bool {
	inner := rect.Width - 2
	return inner >= layout.MinInnerWidth(showChild)
}

// MinActiveWidthPercent returns the minimum active-column width share (1–100) so the
// active panel interior fits three carousel columns at the given total terminal width.
func MinActiveWidthPercent(totalWidth int, layout Layout) int {
	if totalWidth <= 0 {
		return config.DefaultPanelZoomActivePercent
	}
	need := layout.MinInnerWidth(true) + 2 // panel column width including frame sides
	pct := (need*100 + totalWidth - 1) / totalWidth
	if pct < 50 {
		pct = 50
	}
	if pct > 95 {
		pct = 95
	}
	return pct
}

// SplitColumns divides the panel interior (below title + header rows) into carousel panes.
func SplitColumns(rect geom.Rect, showChild bool, layout Layout, measuredFitWidth [3]int) [3]geom.Rect {
	var cols [3]geom.Rect
	innerX := rect.X + 1
	innerW := rect.Width - 2
	if innerW < 2 {
		return cols
	}
	listY := rect.Y + 2
	listH := geom.PanelListRows(rect)
	widths := layout.ResolveMeasured(innerW, showChild, measuredFitWidth)
	x := innerX
	for i := 0; i < 3; i++ {
		if widths[i] <= 0 {
			continue
		}
		cols[i] = geom.Rect{X: x, Y: listY, Width: widths[i], Height: listH}
		x += widths[i]
	}
	return cols
}

// ChildColumnWidth returns the inner width of the carousel child column.
//
// ponytail: uses Layout.Resolve (unmeasured) — this is a structural pre-render check (used to
// decide whether file preview is eligible before the frame is painted) with no live listing
// entries available to measure fit-mode columns against.
func ChildColumnWidth(rect geom.Rect, layout Layout) int {
	innerW := rect.Width - 2
	if innerW < 1 {
		return 0
	}
	return layout.Resolve(innerW, true)[2]
}

// FilePreviewEligible reports whether the carousel child column is wide enough for file preview,
// or the inactive twin panel is hidden so the active panel has full width.
func FilePreviewEligible(rect geom.Rect, hideInactive bool, layout Layout) bool {
	if hideInactive {
		return true
	}
	return ChildColumnWidth(rect, layout) >= config.MinCarouselFilePreviewColumnWidth
}

// ChildPreviewPaintRect returns the rect for embedded file preview in the child column (header + list rows).
func ChildPreviewPaintRect(frame geom.Rect, showChild bool, layout Layout, measuredFitWidth [3]int) (geom.Rect, bool) {
	if !showChild {
		return geom.Rect{}, false
	}
	cols := SplitColumns(frame, true, layout, measuredFitWidth)
	if cols[2].Width <= 0 {
		return geom.Rect{}, false
	}
	return geom.Rect{
		X:      cols[2].X,
		Y:      frame.Y + 1,
		Width:  cols[2].Width,
		Height: cols[2].Height + 1,
	}, true
}
