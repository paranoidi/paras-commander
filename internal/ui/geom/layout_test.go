package geom

import "testing"

func TestCalculateLayoutSplitsScreenIntoExpectedRegions(t *testing.T) {
	layout := CalculateLayout(100, 30, true, PanelWidthSplit{})

	if layout.TooSmall {
		t.Fatal("TooSmall = true, want false")
	}
	if layout.Menu != (Rect{X: 0, Y: 0, Width: 100, Height: 1}) {
		t.Fatalf("Menu = %+v", layout.Menu)
	}
	if layout.Left != (Rect{X: 0, Y: 1, Width: 50, Height: 28}) {
		t.Fatalf("Left = %+v", layout.Left)
	}
	if layout.Right != (Rect{X: 50, Y: 1, Width: 50, Height: 28}) {
		t.Fatalf("Right = %+v", layout.Right)
	}
	if layout.Footer.Y != 29 {
		t.Fatalf("footer = %+v, want y=29", layout.Footer)
	}
}

func TestCalculateLayoutHandlesOddWidth(t *testing.T) {
	layout := CalculateLayout(101, 20, true, PanelWidthSplit{})

	if layout.Left.Width != 50 {
		t.Fatalf("Left.Width = %d, want 50", layout.Left.Width)
	}
	if layout.Right.X != 50 || layout.Right.Width != 51 {
		t.Fatalf("Right = %+v, want x=50 width=51", layout.Right)
	}
}

func TestCalculateLayoutMarksSmallTerminal(t *testing.T) {
	layout := CalculateLayout(39, 8, true, PanelWidthSplit{})
	if !layout.TooSmall {
		t.Fatal("TooSmall = false, want true")
	}

	layout = CalculateLayout(40, 7, true, PanelWidthSplit{})
	if !layout.TooSmall {
		t.Fatal("TooSmall = false, want true")
	}
}

func TestCalculateLayoutOmitsMenuRowWhenShowMenuBarFalse(t *testing.T) {
	layout := CalculateLayout(100, 30, false, PanelWidthSplit{})

	if layout.TooSmall {
		t.Fatal("TooSmall = true, want false")
	}
	if layout.Menu != (Rect{}) {
		t.Fatalf("Menu = %+v, want empty", layout.Menu)
	}
	if layout.Left != (Rect{X: 0, Y: 0, Width: 50, Height: 29}) {
		t.Fatalf("Left = %+v", layout.Left)
	}
	if layout.Right != (Rect{X: 50, Y: 0, Width: 50, Height: 29}) {
		t.Fatalf("Right = %+v", layout.Right)
	}
	if layout.Footer.Y != 29 {
		t.Fatalf("footer = %+v, want y=29", layout.Footer)
	}
}

func TestCalculateLayoutZoomWidensActiveLeftColumn(t *testing.T) {
	layout := CalculateLayout(100, 30, true, PanelWidthSplit{
		Zoom: true, ActivePanel: 0, ActivePercent: 70, InactivePercent: 30,
	})
	if layout.Left.Width != 70 || layout.Right.Width != 30 {
		t.Fatalf("Left=%+v Right=%+v want widths 70/30", layout.Left, layout.Right)
	}
	if layout.Right.X != 70 {
		t.Fatalf("Right.X = %d, want 70", layout.Right.X)
	}
}

func TestCalculateLayoutZoomWidensActiveRightColumn(t *testing.T) {
	layout := CalculateLayout(100, 30, true, PanelWidthSplit{
		Zoom: true, ActivePanel: 1, ActivePercent: 70, InactivePercent: 30,
	})
	if layout.Left.Width != 30 || layout.Right.Width != 70 {
		t.Fatalf("Left=%+v Right=%+v want widths 30/70", layout.Left, layout.Right)
	}
	if layout.Right.X != 30 {
		t.Fatalf("Right.X = %d, want 30", layout.Right.X)
	}
}

func TestPanelListRows(t *testing.T) {
	rows := PanelListRows(Rect{Width: 50, Height: 12})
	if rows != 9 {
		t.Fatalf("PanelListRows() = %d, want 9", rows)
	}

	rows = PanelListRows(Rect{Width: 7, Height: 12})
	if rows != 0 {
		t.Fatalf("PanelListRows() = %d, want 0 for narrow panel", rows)
	}
}

func TestSelectionsStripListRows(t *testing.T) {
	// One more list line than file panel at same height (no column header row).
	if n := SelectionsStripListRows(Rect{Width: 50, Height: 12}); n != 10 {
		t.Fatalf("SelectionsStripListRows = %d, want 10", n)
	}
	if SelectionsStripListRows(Rect{Width: 7, Height: 12}) != 0 {
		t.Fatal("narrow strip should yield 0 rows")
	}
}

func TestSplitPanelColumnAllocatesStripBelowFilePanel(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 28}
	file, strip := SplitPanelColumn(col, 3, 5, 3)
	if file.Y != 0 || strip.Y <= file.Y {
		t.Fatalf("file=%+v strip=%+v", file, strip)
	}
	if file.Height+strip.Height != col.Height {
		t.Fatalf("heights sum %d+%d want %d", file.Height, strip.Height, col.Height)
	}
	if SelectionsStripListRows(strip) < 1 {
		t.Fatalf("strip list rows = %d", SelectionsStripListRows(strip))
	}
	if PanelListRows(file) < 3 {
		t.Fatalf("file list rows = %d want >=3", PanelListRows(file))
	}
}

func TestSplitPanelColumnHidesStripWhenNoItems(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 28}
	file, strip := SplitPanelColumn(col, 0, 5, 3)
	if strip.Height != 0 {
		t.Fatalf("strip = %+v want height 0", strip)
	}
	if file != col {
		t.Fatalf("file = %+v want full column %+v", file, col)
	}
}

func TestSplitJobsRightColumnSizesDetailToLineCount(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 28}
	detail, activity := SplitJobsRightColumn(col, 10)
	wantDetailH := 10 + jobsDetailChromeRows
	if detail.Height != wantDetailH {
		t.Fatalf("detail.Height = %d, want %d (lines + chrome)", detail.Height, wantDetailH)
	}
	if activity.Height != col.Height-wantDetailH {
		t.Fatalf("activity.Height = %d, want %d", activity.Height, col.Height-wantDetailH)
	}
	if detail.Height+activity.Height != col.Height {
		t.Fatal("heights must fill column")
	}
}

func TestSplitJobsRightColumnReservesActivityMinimumWhenCramped(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 12}
	detail, activity := SplitJobsRightColumn(col, 100)
	if activity.Height != jobsSubpanelMinFrameH {
		t.Fatalf("activity.Height = %d, want activity minimum %d", activity.Height, jobsSubpanelMinFrameH)
	}
	if detail.Height != col.Height-activity.Height {
		t.Fatalf("detail.Height = %d", detail.Height)
	}
}

func TestSplitJobsRightColumnOmitsActivityWhenTooShort(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 6}
	detail, activity := SplitJobsRightColumn(col, 3)
	if activity.Height != 0 {
		t.Fatalf("activity = %+v want omitted", activity)
	}
	if detail != col {
		t.Fatalf("detail = %+v want full column", detail)
	}
}

func TestSplitJobsRightPanelsReservesConflictAboveDetail(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 30}
	conflict, detail, activity := SplitJobsRightPanels(col, true, 8)
	if conflict.Height != jobsConflictPanelMinFrameH {
		t.Fatalf("conflict.Height = %d, want %d", conflict.Height, jobsConflictPanelMinFrameH)
	}
	if conflict.Y != col.Y {
		t.Fatalf("conflict.Y = %d, want 0", conflict.Y)
	}
	if detail.Y != col.Y+conflict.Height {
		t.Fatalf("detail should sit below conflict, got detail=%+v conflictH=%d", detail, conflict.Height)
	}
	if detail.Height+activity.Height+conflict.Height != col.Height {
		t.Fatalf("sum of panel heights = %d+%d+%d want %d", conflict.Height, detail.Height, activity.Height, col.Height)
	}
}

func TestSplitJobsRightPanelsNoConflictMatchesLegacySplit(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 28}
	wantD, wantA := SplitJobsRightColumn(col, 10)
	conflict, detail, activity := SplitJobsRightPanels(col, false, 10)
	if conflict.Height != 0 {
		t.Fatalf("conflict = %+v want height 0", conflict)
	}
	if detail != wantD || activity != wantA {
		t.Fatalf("detail=%+v activity=%+v want detail=%+v activity=%+v", detail, activity, wantD, wantA)
	}
}
