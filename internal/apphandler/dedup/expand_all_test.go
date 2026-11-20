package dedup

import (
	"maps"
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// TestExpandAllCapMatchesFullFlatten: on a tree past the row cap, ExpandAll collapses
// exactly the nodes a depth/row-limit scan over the fully flattened tree would.
func TestExpandAllCapMatchesFullFlatten(t *testing.T) {
	for _, treeDirs := range []bool{true, false} {
		snap := benchSnapshot(12000)
		view := ui.DedupViewState{TreeDirs: treeDirs, IgnoreEmpty: true}
		full, _ := ui.DedupRowsFromSnapshot(snap, view)
		limit := panel.MaxExpandAllShallowDepth
		for ; limit > 0; limit-- {
			n := 0
			for _, r := range full {
				if r.Depth <= limit {
					n++
				}
			}
			if n <= panel.MaxExpandAllTotalRows {
				break
			}
		}
		want := map[string]bool{}
		for _, r := range full {
			if r.HasChildren && r.Depth >= limit {
				want[r.ID] = true
			}
		}
		if limit >= panel.MaxExpandAllShallowDepth || len(want) == 0 {
			t.Fatalf("treeDirs=%v: tree does not exceed the row cap (limit %d)", treeDirs, limit)
		}

		model := &ui.Model{ViewMode: ui.ViewDedup, DedupSnapshot: snap, DedupView: ui.DedupViewState{TreeDirs: treeDirs, IgnoreEmpty: true, Cache: &ui.DedupCache{}}}
		h := New(Deps{Host: &dedupHandlerHost{}, Model: model})
		h.syncDedupList()
		h.ExpandAll()
		if got := model.DedupView.Main.Collapsed; !maps.Equal(got, want) {
			t.Fatalf("treeDirs=%v: collapsed %d ids, want %d", treeDirs, len(got), len(want))
		}
	}
}
