package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// BenchmarkDrawDedupAfterKeep is the first frame after a keep that went through
// NoteMarkChange (the handler's path): the counters move by one file.
func BenchmarkDrawDedupAfterKeep(b *testing.B) {
	benchDrawDedupAfterKeep(b, true)
}

// BenchmarkDrawDedupAfterRecount is the first frame after a MarksRev bump nobody
// noted (bulk resets): the counters are recounted from the marks.
func BenchmarkDrawDedupAfterRecount(b *testing.B) {
	benchDrawDedupAfterKeep(b, false)
}

func benchDrawDedupAfterKeep(b *testing.B, noted bool) {
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
	keepAbs := snap.Groups[1].Files[0].Abs.String()
	for b.Loop() {
		if noted {
			on := !view.Kept[keepAbs]
			if on {
				view.Kept[keepAbs] = true
			} else {
				delete(view.Kept, keepAbs)
			}
			view.NoteMarkChange(snap, keepAbs, true, on)
		} else {
			view.MarksRev++
		}
		drawDedupView(screen, layout, view, snap, list, copies, styles, false, "", SplitHorizontal, nil, ListRowHover{})
	}
}

// dedupStripBenchView marks two files of every third group of the nested snapshot and builds
// the strip once, so a benchmark loop starts from a warm cache.
func dedupStripBenchView(snap comparepkg.DedupSnapshot) DedupViewState {
	view := DedupViewState{Cache: &DedupCache{}, Marked: map[string]bool{}}
	for g := 0; g < len(snap.Groups); g += 3 {
		view.Marked[snap.Groups[g].Files[1].Abs.String()] = true
		view.Marked[snap.Groups[g].Files[2].Abs.String()] = true
	}
	DedupStripRows(snap, view, true)
	return view
}

// BenchmarkDedupStripFullRebuild is the Selected strip rebuild after a MarksRev bump nobody
// noted (bulk resets): every mark is sorted, labelled and put into a new tree.
func BenchmarkDedupStripFullRebuild(b *testing.B) {
	snap := dedupNestedBenchSnapshot(40000)
	view := dedupStripBenchView(snap)
	b.ReportAllocs()
	for b.Loop() {
		view.MarksRev++
		DedupStripRows(snap, view, true)
	}
}

// BenchmarkDedupStripOneMark is the strip rebuild after one mark toggled through
// NoteMarkChange (the handler's path), tree mode.
func BenchmarkDedupStripOneMark(b *testing.B) {
	benchDedupStripMarks(b, 1)
}

// BenchmarkDedupStripBatch is the strip rebuild after 1000 files toggled through NoteMarkChange.
func BenchmarkDedupStripBatch(b *testing.B) {
	benchDedupStripMarks(b, 1000)
}

func benchDedupStripMarks(b *testing.B, n int) {
	snap := dedupNestedBenchSnapshot(40000)
	view := dedupStripBenchView(snap)
	var abs []string
	for g := 1; len(abs) < n; g += 3 {
		abs = append(abs, snap.Groups[g].Files[0].Abs.String())
	}
	b.ReportAllocs()
	for b.Loop() {
		for _, p := range abs {
			on := !view.Marked[p]
			if on {
				view.Marked[p] = true
			} else {
				delete(view.Marked, p)
			}
			view.NoteMarkChange(snap, p, false, on)
		}
		DedupStripRows(snap, view, true)
	}
}
