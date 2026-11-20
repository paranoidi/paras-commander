package dedup

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
)

// BenchmarkKeepDirRow is dedup.mark-keep on a top-level directory row in dirs mode
// (toggling the keep on and off), with a tenth of the files marked.
func BenchmarkKeepDirRow(b *testing.B) {
	snap := benchSnapshot(40000)
	view := ui.DedupViewState{TreeDirs: true, IgnoreEmpty: true, Cache: &ui.DedupCache{}, Marked: map[string]bool{}, Kept: map[string]bool{},
		DirsCollapsePending: true, GroupsCollapsePending: true}
	model := &ui.Model{ViewMode: ui.ViewDedup, DedupSnapshot: snap, DedupView: view}
	h := New(Deps{Host: &dedupHandlerHost{}, Model: model})
	h.syncDedupList()
	r, ok := h.selectedRow()
	if !ok || r.Value.Kind != ui.DedupRowDir {
		b.Fatal("cursor row not a dir")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		model.DedupView.Main.Selected = 0
		h.KeepSelection()
	}
}
