package compare

import (
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type rangeHost struct {
	compareHandlerHost
	selected map[string]bool
}

func (h *rangeHost) SetPanelSelection(_ int, path string, on bool) bool {
	if h.selected[path] == on {
		return false
	}
	h.selected[path] = on
	return true
}

func TestSelectColumnRange(t *testing.T) {
	names := []string{"anchor", "bramble", "cobalt", "dune"}
	var rows []comparepkg.Row
	for _, n := range names {
		rows = append(rows, comparepkg.Row{PrimaryRel: n, SecondaryRel: n, Kind: comparepkg.KindContentDiff, HashDone: true})
	}
	model := &ui.Model{
		CompareView: ui.CompareViewState{Filter: comparepkg.FilterAll, FocusColumn: ui.CompareColumnSecondary},
		CompareSnapshot: comparepkg.Snapshot{
			PrimaryRoot: pathloc.MustParse("/left"), SecondaryRoot: pathloc.MustParse("/right"), Rows: rows,
		},
	}
	host := &rangeHost{selected: map[string]bool{}}
	h := New(Deps{Host: host, Model: model})

	st := &model.CompareView
	st.Selected = 1
	if res, _ := h.SelectColumnRange(); res != primitive.RangeStarted || st.RangeAnchor != "/right/bramble" {
		t.Fatalf("first press = %v anchor %q", res, st.RangeAnchor)
	}
	h.ToggleColumnFocus()
	if st.RangeAnchor != "" {
		t.Fatal("column change must clear the anchor")
	}
	h.ToggleColumnFocus()
	h.SelectColumnRange() // re-anchor at row 1
	st.Selected = 3
	res, added := h.SelectColumnRange()
	if res != primitive.RangeDone || added != 3 {
		t.Fatalf("second press = %v added %d, want Done/3", res, added)
	}
	if host.selected["/right/anchor"] || !host.selected["/right/cobalt"] || host.selected["/left/cobalt"] {
		t.Fatalf("selection = %v, want only right-side bramble..dune", host.selected)
	}
}
