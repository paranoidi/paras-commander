package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestListViewsHonorEdgeMargin(t *testing.T) {
	SetListScroll(panel.ScrollModeEdge, 2)
	t.Cleanup(func() { SetListScroll(panel.ScrollModeEdge, 0) })
	const total, vis = 100, 10

	var pane DedupPane
	for sel := 0; sel < 20; sel++ {
		pane.Selected = sel
		pane.EnsureSelectionVisible(total, vis)
		if pos := pane.Selected - pane.ListScroll; pos > vis-1-2 {
			t.Fatalf("dedup: selected %d scroll %d leaves %d rows below, want >= 2", sel, pane.ListScroll, vis-1-pos)
		}
	}
	var cmp CompareViewState
	cmp.Selected = 9
	cmp.EnsureSelectionVisible(total, vis)
	if cmp.ListScroll != 2 {
		t.Fatalf("compare: scroll = %d, want 2 (margin 2 below row 9)", cmp.ListScroll)
	}
	SetListScroll(panel.ScrollModeCenter, 2)
	cmp.Selected = 50
	cmp.EnsureSelectionVisible(total, vis)
	if cmp.ListScroll != 45 {
		t.Fatalf("center: scroll = %d, want 45", cmp.ListScroll)
	}
	cmp.EnsureSelectionVisible(total, 0)
	if cmp.ListScroll != 45 {
		t.Fatal("visibleRows=0 must only clamp")
	}
}
