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
	if layout.Primary != (Rect{X: 0, Y: 1, Width: 50, Height: 28}) {
		t.Fatalf("Left = %+v", layout.Primary)
	}
	if layout.Secondary != (Rect{X: 50, Y: 1, Width: 50, Height: 28}) {
		t.Fatalf("Right = %+v", layout.Secondary)
	}
	if layout.Footer.Y != 29 {
		t.Fatalf("footer = %+v, want y=29", layout.Footer)
	}
}

func TestCalculateLayoutHandlesOddWidth(t *testing.T) {
	layout := CalculateLayout(101, 20, true, PanelWidthSplit{})

	if layout.Primary.Width != 50 {
		t.Fatalf("Left.Width = %d, want 50", layout.Primary.Width)
	}
	if layout.Secondary.X != 50 || layout.Secondary.Width != 51 {
		t.Fatalf("Right = %+v, want x=50 width=51", layout.Secondary)
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
	if layout.Primary != (Rect{X: 0, Y: 0, Width: 50, Height: 29}) {
		t.Fatalf("Left = %+v", layout.Primary)
	}
	if layout.Secondary != (Rect{X: 50, Y: 0, Width: 50, Height: 29}) {
		t.Fatalf("Right = %+v", layout.Secondary)
	}
	if layout.Footer.Y != 29 {
		t.Fatalf("footer = %+v, want y=29", layout.Footer)
	}
}

func TestCalculateLayoutZoomWidensActiveLeftColumn(t *testing.T) {
	layout := CalculateLayout(100, 30, true, PanelWidthSplit{
		Zoom: true, ActivePanel: 0, ActivePercent: 70, InactivePercent: 30,
	})
	if layout.Primary.Width != 70 || layout.Secondary.Width != 30 {
		t.Fatalf("Left=%+v Right=%+v want widths 70/30", layout.Primary, layout.Secondary)
	}
	if layout.Secondary.X != 70 {
		t.Fatalf("Right.X = %d, want 70", layout.Secondary.X)
	}
}

func TestCalculateLayoutHideInactivePanelGivesActiveFullWidth(t *testing.T) {
	layout := CalculateLayout(100, 30, true, PanelWidthSplit{
		HideInactivePanel: true,
		ActivePanel:       0,
	})
	if layout.Primary.Width != 100 || layout.Secondary.Width != 0 {
		t.Fatalf("Left=%+v Right=%+v want widths 100/0", layout.Primary, layout.Secondary)
	}
	layout = CalculateLayout(100, 30, true, PanelWidthSplit{
		HideInactivePanel: true,
		ActivePanel:       1,
	})
	if layout.Primary.Width != 0 || layout.Secondary.Width != 100 {
		t.Fatalf("Left=%+v Right=%+v want widths 0/100", layout.Primary, layout.Secondary)
	}
}

func TestCalculateLayoutZoomWidensActiveRightColumn(t *testing.T) {
	layout := CalculateLayout(100, 30, true, PanelWidthSplit{
		Zoom: true, ActivePanel: 1, ActivePercent: 70, InactivePercent: 30,
	})
	if layout.Primary.Width != 30 || layout.Secondary.Width != 70 {
		t.Fatalf("Left=%+v Right=%+v want widths 30/70", layout.Primary, layout.Secondary)
	}
	if layout.Secondary.X != 30 {
		t.Fatalf("Right.X = %d, want 30", layout.Secondary.X)
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

func TestSplitJobsSecondaryColumnSizesDetailToLineCount(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 28}
	detail, activity := SplitJobsSecondaryColumn(col, 10)
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

func TestSplitJobsSecondaryColumnReservesActivityMinimumWhenCramped(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 12}
	detail, activity := SplitJobsSecondaryColumn(col, 100)
	if activity.Height != jobsSubpanelMinFrameH {
		t.Fatalf("activity.Height = %d, want activity minimum %d", activity.Height, jobsSubpanelMinFrameH)
	}
	if detail.Height != col.Height-activity.Height {
		t.Fatalf("detail.Height = %d", detail.Height)
	}
}

func TestSplitJobsSecondaryColumnOmitsActivityWhenTooShort(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 6}
	detail, activity := SplitJobsSecondaryColumn(col, 3)
	if activity.Height != 0 {
		t.Fatalf("activity = %+v want omitted", activity)
	}
	if detail != col {
		t.Fatalf("detail = %+v want full column", detail)
	}
}

func TestSplitJobsSecondaryColumnFlexTopSizesBottomToLineCount(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 28}
	top, bottom := SplitJobsSecondaryColumnFlexTop(col, 10)
	wantBottomH := 10 + jobsDetailChromeRows
	if bottom.Height != wantBottomH {
		t.Fatalf("bottom.Height = %d, want %d (lines + chrome)", bottom.Height, wantBottomH)
	}
	if top.Height != col.Height-wantBottomH {
		t.Fatalf("top.Height = %d, want %d", top.Height, col.Height-wantBottomH)
	}
	if top.Height+bottom.Height != col.Height {
		t.Fatal("heights must fill column")
	}
}

func TestSplitJobsSecondaryColumnFlexTopReservesBottomMinimumWhenCramped(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 12}
	top, bottom := SplitJobsSecondaryColumnFlexTop(col, 100)
	if bottom.Height != jobsSubpanelMinFrameH {
		t.Fatalf("bottom.Height = %d, want minimum %d", bottom.Height, jobsSubpanelMinFrameH)
	}
	if top.Height != col.Height-bottom.Height {
		t.Fatalf("top.Height = %d", top.Height)
	}
}

func TestSplitJobsSecondaryPanelsReservesConflictAboveDetail(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 30}
	conflict, detail, activity := SplitJobsSecondaryPanels(col, true, 8)
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

func TestSplitJobsSecondaryPanelsNoConflictMatchesLegacySplit(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 50, Height: 28}
	wantD, wantA := SplitJobsSecondaryColumn(col, 10)
	conflict, detail, activity := SplitJobsSecondaryPanels(col, false, 10)
	if conflict.Height != 0 {
		t.Fatalf("conflict = %+v want height 0", conflict)
	}
	if detail != wantD || activity != wantA {
		t.Fatalf("detail=%+v activity=%+v want detail=%+v activity=%+v", detail, activity, wantD, wantA)
	}
}

func TestCalculateLayoutStackedSplitsHeight(t *testing.T) {
	layout := CalculateLayoutWithOrientation(LayoutInput{
		Width: 100, Height: 30, ShowMenuBar: true, Split: PanelPaneSplit{},
		Orientation: SplitVertical,
	})
	if layout.TooSmall {
		t.Fatal("TooSmall = true, want false")
	}
	if layout.Primary != (Rect{X: 0, Y: 1, Width: 100, Height: 14}) {
		t.Fatalf("Primary = %+v", layout.Primary)
	}
	if layout.Secondary != (Rect{X: 0, Y: 15, Width: 100, Height: 14}) {
		t.Fatalf("Secondary = %+v", layout.Secondary)
	}
}

func TestCalculateLayoutStackedHandlesOddHeight(t *testing.T) {
	layout := CalculateLayoutWithOrientation(LayoutInput{
		Width: 80, Height: 17, ShowMenuBar: true, Split: PanelPaneSplit{},
		Orientation: SplitVertical,
	})
	if layout.Primary.Height != 7 || layout.Secondary.Height != 8 {
		t.Fatalf("Primary.Height=%d Secondary.Height=%d want 7/8", layout.Primary.Height, layout.Secondary.Height)
	}
	if layout.Secondary.Y != layout.Primary.Y+layout.Primary.Height {
		t.Fatalf("Secondary.Y = %d want %d", layout.Secondary.Y, layout.Primary.Y+layout.Primary.Height)
	}
}

func TestCalculateLayoutStackedZoomWidensActivePrimary(t *testing.T) {
	layout := CalculateLayoutWithOrientation(LayoutInput{
		Width: 80, Height: 40, ShowMenuBar: true,
		Split:       PanelPaneSplit{Zoom: true, ActivePanel: 0, ActivePercent: 70, InactivePercent: 30},
		Orientation: SplitVertical,
	})
	panelH := 38
	wantPrimaryH := (panelH * 70) / 100
	if layout.Primary.Height != wantPrimaryH {
		t.Fatalf("Primary.Height = %d want %d", layout.Primary.Height, wantPrimaryH)
	}
	if layout.Secondary.Height != panelH-layout.Primary.Height {
		t.Fatalf("Secondary.Height = %d want %d", layout.Secondary.Height, panelH-layout.Primary.Height)
	}
}

func TestCalculateLayoutStackedHideInactiveGivesActiveFullHeight(t *testing.T) {
	layout := CalculateLayoutWithOrientation(LayoutInput{
		Width: 80, Height: 30, ShowMenuBar: true,
		Split:       PanelPaneSplit{HideInactivePanel: true, ActivePanel: 0},
		Orientation: SplitVertical,
	})
	if layout.Primary.Height != 28 || layout.Secondary.Height != 0 {
		t.Fatalf("Primary=%+v Secondary=%+v want heights 28/0", layout.Primary, layout.Secondary)
	}
}

func TestCalculateLayoutStackedMarksShortTerminal(t *testing.T) {
	layout := CalculateLayoutWithOrientation(LayoutInput{
		Width: 80, Height: 15, ShowMenuBar: true, Split: PanelPaneSplit{},
		Orientation: SplitVertical,
	})
	if !layout.TooSmall {
		t.Fatal("TooSmall = false, want true for height < minStackedHeight")
	}
}

func TestCalculateLayoutReservesTerminalPanelSideBySide(t *testing.T) {
	layout := CalculateLayoutWithOrientation(LayoutInput{
		Width: 100, Height: 30, ShowMenuBar: true, Split: PanelPaneSplit{},
		Orientation: SplitHorizontal, TerminalRows: 5,
	})
	if layout.TooSmall {
		t.Fatal("TooSmall = true, want false")
	}
	wantTerminal := Rect{X: 0, Y: 24, Width: 100, Height: 5}
	if layout.Terminal != wantTerminal {
		t.Fatalf("Terminal = %+v, want %+v", layout.Terminal, wantTerminal)
	}
	if layout.Primary.Height != 23 || layout.Secondary.Height != 23 {
		t.Fatalf("Primary=%+v Secondary=%+v want height 23 (panel area reduced by terminal)", layout.Primary, layout.Secondary)
	}
	if layout.Terminal.Y != layout.Footer.Y-layout.Terminal.Height {
		t.Fatalf("Terminal.Y = %d, want directly above footer (footerY=%d, height=%d)", layout.Terminal.Y, layout.Footer.Y, layout.Terminal.Height)
	}
}

func TestCalculateLayoutReservesTerminalPanelStacked(t *testing.T) {
	layout := CalculateLayoutWithOrientation(LayoutInput{
		Width: 80, Height: 40, ShowMenuBar: true, Split: PanelPaneSplit{},
		Orientation: SplitVertical, TerminalRows: 5,
	})
	if layout.TooSmall {
		t.Fatal("TooSmall = true, want false")
	}
	wantTerminal := Rect{X: 0, Y: 34, Width: 80, Height: 5}
	if layout.Terminal != wantTerminal {
		t.Fatalf("Terminal = %+v, want %+v", layout.Terminal, wantTerminal)
	}
	if layout.Primary.Height+layout.Secondary.Height != 33 {
		t.Fatalf("Primary=%+v Secondary=%+v want combined height 33 (panel area reduced by terminal)", layout.Primary, layout.Secondary)
	}
	if layout.Secondary.Y+layout.Secondary.Height != layout.Terminal.Y {
		t.Fatalf("Secondary bottom (%d) should meet Terminal.Y (%d)", layout.Secondary.Y+layout.Secondary.Height, layout.Terminal.Y)
	}
}

func TestCalculateLayoutClampsTerminalRowsToMinimumThree(t *testing.T) {
	layout := CalculateLayoutWithOrientation(LayoutInput{
		Width: 100, Height: 30, ShowMenuBar: true, Split: PanelPaneSplit{},
		Orientation: SplitHorizontal, TerminalRows: 1,
	})
	if layout.Terminal.Height != 3 {
		t.Fatalf("Terminal.Height = %d, want 3 (minimum content rows)", layout.Terminal.Height)
	}
}

func TestCalculateLayoutShrinksTerminalWhenPanelAreaWouldStarve(t *testing.T) {
	layout := CalculateLayoutWithOrientation(LayoutInput{
		Width: 100, Height: 13, ShowMenuBar: true, Split: PanelPaneSplit{},
		Orientation: SplitHorizontal, TerminalRows: 10,
	})
	if layout.TooSmall {
		t.Fatal("TooSmall = true, want false")
	}
	if layout.Terminal.Height != 5 {
		t.Fatalf("Terminal.Height = %d, want 5 (shrunk to keep panel area at its minimum)", layout.Terminal.Height)
	}
	if layout.Primary.Height != 6 {
		t.Fatalf("Primary.Height = %d, want 6 (panelAreaMin preserved)", layout.Primary.Height)
	}
}

func TestCalculateLayoutOmitsTerminalWhenPanelAreaAlreadyAtMinimum(t *testing.T) {
	layout := CalculateLayoutWithOrientation(LayoutInput{
		Width: 100, Height: 8, ShowMenuBar: true, Split: PanelPaneSplit{},
		Orientation: SplitHorizontal, TerminalRows: 5,
	})
	if layout.TooSmall {
		t.Fatal("TooSmall = true, want false")
	}
	if layout.Terminal != (Rect{}) {
		t.Fatalf("Terminal = %+v, want omitted (zero rect)", layout.Terminal)
	}
	if layout.Primary.Height != 6 || layout.Secondary.Height != 6 {
		t.Fatalf("Primary=%+v Secondary=%+v want height 6 (unaffected by omitted terminal)", layout.Primary, layout.Secondary)
	}
}

func TestCalculateLayoutTerminalRowsIgnoredWhenTooSmall(t *testing.T) {
	layout := CalculateLayoutWithOrientation(LayoutInput{
		Width: 39, Height: 8, ShowMenuBar: true, Split: PanelPaneSplit{},
		Orientation: SplitHorizontal, TerminalRows: 5,
	})
	if !layout.TooSmall {
		t.Fatal("TooSmall = false, want true")
	}
	if layout.Terminal != (Rect{}) {
		t.Fatalf("Terminal = %+v, want zero rect when TooSmall", layout.Terminal)
	}
}

func TestMergePaneRectsSideBySide(t *testing.T) {
	primary := Rect{X: 0, Y: 1, Width: 50, Height: 28}
	secondary := Rect{X: 50, Y: 1, Width: 50, Height: 28}
	got := MergePaneRects(primary, secondary, SplitHorizontal)
	want := Rect{X: 0, Y: 1, Width: 100, Height: 28}
	if got != want {
		t.Fatalf("MergePaneRects = %+v want %+v", got, want)
	}
}

func TestMergePaneRectsStacked(t *testing.T) {
	primary := Rect{X: 0, Y: 1, Width: 100, Height: 14}
	secondary := Rect{X: 0, Y: 15, Width: 100, Height: 14}
	got := MergePaneRects(primary, secondary, SplitVertical)
	want := Rect{X: 0, Y: 1, Width: 100, Height: 28}
	if got != want {
		t.Fatalf("MergePaneRects = %+v want %+v", got, want)
	}
}
