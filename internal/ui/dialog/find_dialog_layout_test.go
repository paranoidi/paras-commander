package dialog

import "testing"

func TestFindDialogMetricsListRowsFitDialogRect(t *testing.T) {
	layout := Layout{Width: 80, Height: 24}
	width, height, listH, ok := FindDialogMetrics(layout, true)
	if !ok {
		t.Fatal("FindDialogMetrics: want ok")
	}
	rect := centeredDialogRectForTest(layout, width, height)
	checkboxRows := 2
	listTop := rect.Y + 5 + checkboxRows + 1
	buttonY := rect.Y + height - 2
	if listTop+listH >= buttonY {
		t.Fatalf("list rows spill into button row: listTop=%d listH=%d buttonY=%d", listTop, listH, buttonY)
	}
	footerY := layout.Height - 1
	if listTop+listH > footerY {
		t.Fatalf("list rows spill into footer: bottom=%d footerY=%d", listTop+listH, footerY)
	}
}

// centeredDialogRectForTest mirrors draw.CenteredDialogRect without importing draw in test-only helper.
func centeredDialogRectForTest(layout Layout, width, height int) Rect {
	x := (layout.Width - width) / 2
	y := (layout.Height - height) / 2
	return Rect{X: x, Y: y, Width: width, Height: height}
}
