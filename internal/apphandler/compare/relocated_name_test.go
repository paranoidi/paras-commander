package compare

import (
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestToggleRelocatedNameSetsFocusedSideAndClears(t *testing.T) {
	relocated := comparepkg.Row{Kind: comparepkg.KindRelocated, PrimaryRel: "harbor/compass.txt", SecondaryRel: "compass.txt", HashDone: true}
	only := comparepkg.Row{Kind: comparepkg.KindPrimaryOnly, PrimaryRel: "lantern.txt", HashDone: true}
	model := &ui.Model{
		CompareView: ui.CompareViewState{Filter: comparepkg.FilterAll, FocusColumn: ui.CompareColumnSecondary},
		CompareSnapshot: comparepkg.Snapshot{
			PrimaryRoot: pathloc.MustParse("/left"), SecondaryRoot: pathloc.MustParse("/right"),
			Rows: []comparepkg.Row{relocated, only},
		},
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})
	st := &model.CompareView
	key := relocated.Key()

	h.ToggleRelocatedName()
	if got := st.RelocatedNames[key]; got != comparepkg.NameSecondary {
		t.Fatalf("right column: name = %v, want NameSecondary", got)
	}
	h.ToggleColumnFocus()
	h.ToggleRelocatedName()
	if got := st.RelocatedNames[key]; got != comparepkg.NamePrimary {
		t.Fatalf("left column: name = %v, want NamePrimary", got)
	}
	h.ToggleRelocatedName()
	if _, ok := st.RelocatedNames[key]; ok {
		t.Fatal("same column again must clear the choice")
	}

	st.Selected = 1
	h.ToggleRelocatedName()
	if len(st.RelocatedNames) != 0 {
		t.Fatalf("non-relocated row got a name choice: %v", st.RelocatedNames)
	}
}
