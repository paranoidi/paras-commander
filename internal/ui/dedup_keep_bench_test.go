package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// BenchmarkDrawDedupAfterKeep is the first frame after a keep: MarksRev changed.
func BenchmarkDrawDedupAfterKeep(b *testing.B) {
	snap := dedupNestedBenchSnapshot(40000)
	view := DedupViewState{TreeDirs: true, IgnoreEmpty: true, Cache: &DedupCache{}, Marked: map[string]bool{}, Kept: map[string]bool{}}
	for g := 0; g < len(snap.Groups); g += 3 {
		view.Kept[snap.Groups[g].Files[0].Abs.String()] = true
		view.Marked[snap.Groups[g].Files[1].Abs.String()] = true
		view.Marked[snap.Groups[g].Files[2].Abs.String()] = true
	}
	ids := DedupExpandableIDs(snap, view)
	view.Main.Collapsed = DedupCollapsedSet(ids)
	list, _ := DedupRowsFromSnapshot(snap, view)
	copies := DedupCopyRows(snap, list[0], nil, view.Cache)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		b.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 40)
	layout := Layout{Primary: Rect{X: 0, Y: 1, Width: 60, Height: 36}, Secondary: Rect{X: 60, Y: 1, Width: 60, Height: 36}}
	styles := theme.Default()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		view.MarksRev++
		drawDedupView(screen, layout, view, snap, list, copies, styles, false, "", SplitHorizontal, nil, ListRowHover{})
	}
}

// BenchmarkDedupStripAfterKeep is the Selected strip rebuild after a keep (tree mode).
func BenchmarkDedupStripAfterKeep(b *testing.B) {
	snap := dedupNestedBenchSnapshot(40000)
	view := DedupViewState{Cache: &DedupCache{}, Marked: map[string]bool{}}
	for g := 0; g < len(snap.Groups); g += 3 {
		view.Marked[snap.Groups[g].Files[1].Abs.String()] = true
		view.Marked[snap.Groups[g].Files[2].Abs.String()] = true
	}
	b.ReportAllocs()
	for b.Loop() {
		view.MarksRev++
		DedupStripRows(snap, view, true)
	}
}
