package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/testutil"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func BenchmarkDrawDedupViewLargeList(b *testing.B) {
	const groups = 45464
	snap := testutil.SyntheticDedupSnapshot(groups)
	list, _ := DedupRowsFromSnapshot(snap, DedupViewState{IgnoreEmpty: true})
	if len(list) != groups*2 {
		b.Fatalf("list len = %d, want %d", len(list), groups*2)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		b.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 40)

	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 60, Height: 36},
		Secondary: Rect{X: 60, Y: 1, Width: 60, Height: 36},
	}
	view := DedupViewState{Main: DedupPane{Selected: 1}, Marked: map[string]bool{}}
	copies := DedupCopyRows(snap, list[1], nil)
	styles := theme.Default()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		drawDedupView(screen, layout, view, snap, list, copies, styles, false, "", SplitHorizontal, nil)
	}
}
