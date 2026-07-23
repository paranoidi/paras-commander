package panelcarousel

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

func TestLayoutFits(t *testing.T) {
	layout := DefaultLayout()
	narrow := geom.Rect{X: 0, Y: 0, Width: 50, Height: 20}
	if LayoutFits(narrow, layout, true) {
		t.Fatal("narrow panel should not fit carousel")
	}
	wide := geom.Rect{X: 0, Y: 0, Width: config.MinCarouselPanelInnerWidth + 4, Height: 20}
	if !LayoutFits(wide, layout, true) {
		t.Fatal("wide panel should fit carousel")
	}
}

func TestSplitColumnsWidths(t *testing.T) {
	layout := DefaultLayout()
	rect := geom.Rect{X: 1, Y: 2, Width: 92, Height: 18}
	cols := SplitColumns(rect, true, layout, [3]int{})
	sum := cols[0].Width + cols[1].Width + cols[2].Width
	if sum != rect.Width-2 {
		t.Fatalf("column widths sum %d, want inner %d", sum, rect.Width-2)
	}
}

func TestSplitColumnsWidensCenterWithoutChild(t *testing.T) {
	layout, err := ParseLayout([]string{"*", "*", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rect := geom.Rect{X: 0, Y: 0, Width: 92, Height: 18}
	innerW := rect.Width - 2
	cols := SplitColumns(rect, false, layout, [3]int{})
	widths := layout.Resolve(innerW, false)
	if cols[0].Width != widths[0] {
		t.Fatalf("parent width = %d, want %d", cols[0].Width, widths[0])
	}
	if cols[1].Width != widths[1] {
		t.Fatalf("center width = %d, want %d", cols[1].Width, widths[1])
	}
	if cols[2].Width != 0 {
		t.Fatalf("child width = %d, want 0", cols[2].Width)
	}
}

func TestMinActiveWidthPercent(t *testing.T) {
	layout := DefaultLayout()
	if got := MinActiveWidthPercent(80, layout); got < 90 {
		t.Fatalf("MinActiveWidthPercent(80) = %d, want wide active split on small terminal", got)
	}
	if got := MinActiveWidthPercent(200, layout); got < 50 {
		t.Fatalf("MinActiveWidthPercent(200) = %d, want at least 50", got)
	}
}

func TestFilePreviewEligible(t *testing.T) {
	layout := DefaultLayout()
	minCarousel := geom.Rect{X: 0, Y: 0, Width: config.MinCarouselPanelInnerWidth + 2, Height: 20}
	if FilePreviewEligible(minCarousel, false, layout) {
		t.Fatal("bare-minimum carousel width should not enable file preview")
	}
	if !FilePreviewEligible(minCarousel, true, layout) {
		t.Fatal("hide inactive should enable file preview at minimum carousel width")
	}
	wide := geom.Rect{X: 0, Y: 0, Width: config.MinCarouselFilePreviewColumnWidth*3 + 2, Height: 20}
	if !FilePreviewEligible(wide, false, layout) {
		t.Fatal("wide child column should enable file preview")
	}
}

func TestSplitColumnsAndChildPreviewPaintRectAgreeOnMeasuredFitWidth(t *testing.T) {
	t.Parallel()
	layout, err := ParseLayout([]string{"<16", "*", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rect := geom.Rect{X: 0, Y: 0, Width: 92, Height: 18}
	measured := [3]int{9, 0, 0}

	cols := SplitColumns(rect, true, layout, measured)
	previewRect, ok := ChildPreviewPaintRect(rect, true, layout, measured)
	if !ok {
		t.Fatal("ChildPreviewPaintRect should succeed for a wide-enough child column")
	}
	if previewRect.X != cols[2].X || previewRect.Width != cols[2].Width {
		t.Fatalf("ChildPreviewPaintRect = {X:%d Width:%d}, want to match SplitColumns col[2] = {X:%d Width:%d}",
			previewRect.X, previewRect.Width, cols[2].X, cols[2].Width)
	}
	// The parent column shrank to the measured width (under its 16-cell cap), which changes
	// column 2's X — both call sites must agree given the same measuredFitWidth.
	if cols[0].Width != 9 {
		t.Fatalf("parent column width = %d, want 9 (measured under cap)", cols[0].Width)
	}
}

func TestSplitColumnsCustomLayout(t *testing.T) {
	layout, err := ParseLayout([]string{"20%", "30%", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rect := geom.Rect{X: 0, Y: 0, Width: 92, Height: 18}
	cols := SplitColumns(rect, true, layout, [3]int{})
	want := layout.Resolve(rect.Width-2, true)
	if cols[0].Width != want[0] || cols[1].Width != want[1] || cols[2].Width != want[2] {
		t.Fatalf("cols = [%d %d %d], want %v", cols[0].Width, cols[1].Width, cols[2].Width, want)
	}
}
