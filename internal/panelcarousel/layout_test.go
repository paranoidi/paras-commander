package panelcarousel

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

func TestLayoutFits(t *testing.T) {
	narrow := geom.Rect{X: 0, Y: 0, Width: 50, Height: 20}
	if LayoutFits(narrow, 0, 0) {
		t.Fatal("narrow panel should not fit carousel")
	}
	wide := geom.Rect{X: 0, Y: 0, Width: config.MinCarouselPanelInnerWidth + 4, Height: 20}
	if !LayoutFits(wide, 0, 0) {
		t.Fatal("wide panel should fit carousel")
	}
}

func TestSplitColumnsWidths(t *testing.T) {
	rect := geom.Rect{X: 1, Y: 2, Width: 92, Height: 18}
	cols := SplitColumns(rect, true)
	sum := cols[0].Width + cols[1].Width + cols[2].Width
	if sum != rect.Width-2 {
		t.Fatalf("column widths sum %d, want inner %d", sum, rect.Width-2)
	}
}

func TestSplitColumnsWidensCenterWithoutChild(t *testing.T) {
	rect := geom.Rect{X: 0, Y: 0, Width: 92, Height: 18}
	innerW := rect.Width - 2
	cols := SplitColumns(rect, false)
	if cols[0].Width != innerW/3 {
		t.Fatalf("parent width = %d, want %d", cols[0].Width, innerW/3)
	}
	if cols[1].Width != innerW-cols[0].Width {
		t.Fatalf("center width = %d, want %d", cols[1].Width, innerW-cols[0].Width)
	}
	if cols[2].Width != 0 {
		t.Fatalf("child width = %d, want 0", cols[2].Width)
	}
}

func TestMinActiveWidthPercent(t *testing.T) {
	if got := MinActiveWidthPercent(80); got < 90 {
		t.Fatalf("MinActiveWidthPercent(80) = %d, want wide active split on small terminal", got)
	}
	if got := MinActiveWidthPercent(200); got < 50 {
		t.Fatalf("MinActiveWidthPercent(200) = %d, want at least 50", got)
	}
}
