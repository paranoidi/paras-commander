package dialog

import "testing"

func TestFindDialogMetricsListRowsFitDialogRect(t *testing.T) {
	layout := Layout{Width: 80, Height: 24}
	width, height, listH, ok := FindDialogMetrics(layout, &FindDialogState{ShowSearchSelectionsOption: true})
	if !ok {
		t.Fatal("FindDialogMetrics: want ok")
	}
	rect := centeredDialogRectForTest(layout, width, height)
	checkboxRows := 2
	listTop := rect.Y + 3 + checkboxRows + 1
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

func TestFindDialogAdvancedKeepsFrameHeight(t *testing.T) {
	for _, size := range [][2]int{{130, 40}, {125, 40}, {100, 30}, {80, 24}, {70, 40}, {80, 30}} {
		layout := Layout{Width: size[0], Height: size[1]}
		closed := &FindDialogState{}
		_, closedH, closedList, ok := FindDialogMetrics(layout, closed)
		if !ok {
			t.Fatalf("%v: closed dialog does not fit", size)
		}
		open := &FindDialogState{Advanced: FindAdvancedState{Expanded: true}}
		_, openH, openList, ok := FindDialogMetrics(layout, open)
		if !ok {
			t.Fatalf("%v: open dialog does not fit", size)
		}
		if openList > 1 && openH != closedH {
			t.Errorf("%v: frame height closed %d, open %d; want equal", size, closedH, openH)
		}
		if openList >= closedList {
			t.Errorf("%v: list rows closed %d, open %d; want fewer when open", size, closedList, openList)
		}
	}
}

func TestFindDialogNoMatchesOnlyWhenIndexComplete(t *testing.T) {
	st := &FindDialogState{Query: "zzq", IndexDone: true}
	if !st.NoMatches() {
		t.Fatal("finished index, no results: want NoMatches")
	}
	for name, mut := range map[string]func(*FindDialogState){
		"indexing":     func(s *FindDialogState) { s.Indexing, s.IndexDone = true, false },
		"rank pending": func(s *FindDialogState) { s.RankPending = true },
		"has results":  func(s *FindDialogState) { s.Ranked = []int{0} },
		"empty query":  func(s *FindDialogState) { s.Query = "  " },
	} {
		s := *st
		mut(&s)
		if s.NoMatches() {
			t.Errorf("%s: NoMatches = true, want false", name)
		}
	}
	idx := &IndexFindDialogState{Query: "zzq"}
	if !idx.NoMatches() {
		t.Fatal("index find: finished query without results: want NoMatches")
	}
	idx.Searching = true
	if idx.NoMatches() {
		t.Fatal("index find: searching: want false")
	}
}
