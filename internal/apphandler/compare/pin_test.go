package compare

import (
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestSelectedColumnPinTargetPrimaryAndSecondary(t *testing.T) {
	model := &ui.Model{
		CompareView: ui.CompareViewState{Selected: 0, FocusColumn: ui.CompareColumnPrimary},
		CompareSnapshot: comparepkg.Snapshot{
			PrimaryRoot:   pathloc.MustParse("/primary"),
			SecondaryRoot: pathloc.MustParse("/secondary"),
			Rows: []comparepkg.Row{
				{PrimaryRel: "a.txt", SecondaryRel: "b.txt", Kind: comparepkg.KindContentDiff, HashDone: true},
			},
		},
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})

	path, ok := h.SelectedColumnPinTarget()
	if !ok {
		t.Fatal("SelectedColumnPinTarget: ok = false, want true for primary column")
	}
	if want := "/primary/a.txt"; path != want {
		t.Fatalf("primary column path = %q, want %q", path, want)
	}

	model.CompareView.FocusColumn = ui.CompareColumnSecondary
	path, ok = h.SelectedColumnPinTarget()
	if !ok {
		t.Fatal("SelectedColumnPinTarget: ok = false, want true for secondary column")
	}
	if want := "/secondary/b.txt"; path != want {
		t.Fatalf("secondary column path = %q, want %q", path, want)
	}
}

func TestSelectedColumnPinTargetNoCounterpart(t *testing.T) {
	model := &ui.Model{
		CompareView: ui.CompareViewState{Selected: 0, FocusColumn: ui.CompareColumnSecondary},
		CompareSnapshot: comparepkg.Snapshot{
			PrimaryRoot:   pathloc.MustParse("/primary"),
			SecondaryRoot: pathloc.MustParse("/secondary"),
			Rows: []comparepkg.Row{
				{PrimaryRel: "only-here.txt", Kind: comparepkg.KindPrimaryOnly},
			},
		},
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})

	if _, ok := h.SelectedColumnPinTarget(); ok {
		t.Fatal("SelectedColumnPinTarget: ok = true, want false when the focused side has no entry")
	}
}

func TestSelectedColumnPinTargetNoSelection(t *testing.T) {
	model := &ui.Model{
		CompareView:     ui.CompareViewState{Selected: 0},
		CompareSnapshot: comparepkg.Snapshot{},
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})

	if _, ok := h.SelectedColumnPinTarget(); ok {
		t.Fatal("SelectedColumnPinTarget: ok = true, want false with an empty row list")
	}
}
