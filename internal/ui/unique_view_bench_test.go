package ui

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// uniqueBenchState is files unique files under a five-deep word-named tree, every directory
// collapsed but the first top-level one (the state after opening one directory).
func uniqueBenchState(files int) (*UniqueViewState, []string) {
	words := []string{"amber", "birch", "cedar", "dune", "ember", "fjord", "grove", "harbor", "iris", "juniper", "kettle", "lantern"}
	st := &UniqueViewState{Root: "/scan/root", Scope: []string{"/scan/root"}, Marked: map[string]bool{}}
	for i := range files {
		rel := fmt.Sprintf("%s/%s/%s/%s/%s-%d.bin", words[i%12], words[(i/12)%12], words[(i/144)%12], words[(i/7)%12], words[(i/3)%12], i)
		st.Files = append(st.Files, UniqueFile{Path: "/scan/root/" + rel, Size: int64(4096 + i), Unverified: i%5 == 0})
	}
	ids := UniqueExpandableIDs(st)
	st.Pane.Collapsed = DedupCollapsedSet(ids)
	st.Loaded = true
	return st, ids
}

// BenchmarkUniqueRowsToggle is one expand/collapse of a top-level directory.
func BenchmarkUniqueRowsToggle(b *testing.B) {
	st, ids := uniqueBenchState(120000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st.Pane.SetCollapsed(ids[0], i%2 == 0)
		UniqueRows(st)
	}
}

// BenchmarkUniqueRowsExpandAll is the flatten of a fully open tree.
func BenchmarkUniqueRowsExpandAll(b *testing.B) {
	st, _ := uniqueBenchState(120000)
	st.Pane.Collapsed = nil
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		UniqueRows(st)
	}
}

// BenchmarkDrawUniqueViewExpanded is one frame of a fully open tree with some marks.
func BenchmarkDrawUniqueViewExpanded(b *testing.B) {
	st, _ := uniqueBenchState(120000)
	st.Pane.Collapsed = nil
	for i := 0; i < len(st.Files); i += 10 {
		st.SetMark(st.Files[i], true)
	}
	rows := UniqueRows(st)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		b.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 40)
	layout := Layout{Primary: Rect{X: 0, Y: 1, Width: 60, Height: 36}, Secondary: Rect{X: 60, Y: 1, Width: 60, Height: 36}}
	model := Model{UniqueView: *st, UniqueList: rows}
	styles := theme.Default()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		drawUniqueView(screen, layout, model, styles, false)
	}
}
