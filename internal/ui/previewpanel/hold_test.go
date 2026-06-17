package previewpanel

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestHoldableInternalCells(t *testing.T) {
	t.Parallel()
	base := tcell.StyleDefault
	st := State{
		Open:             true,
		Phase:            PhaseDone,
		Source:           SourceInternalHighlighted,
		HighlightedCells: []AnsiCell{{R: 'x', St: base}},
	}
	if !st.Holdable() {
		t.Fatal("internal highlighted preview should be holdable")
	}
}

func TestMergeDrawWithHoldKeepsInternalCells(t *testing.T) {
	t.Parallel()
	base := tcell.StyleDefault
	hold := State{
		Open: true, Phase: PhaseDone, Source: SourceInternalHighlighted,
		HighlightedCells: []AnsiCell{{R: 's', St: base}, {R: 't', St: base}},
	}
	hold.EnsureWrappedLines(10, base)
	live := State{
		Open: true, Phase: PhasePending, Path: "/tmp/new.go", TitleBase: "new.go",
	}
	draw := MergeDrawWithHold(live, hold)
	if !draw.BodyHeld {
		t.Fatal("BodyHeld = false, want true")
	}
	if draw.Source != SourceInternalHighlighted {
		t.Fatalf("Source = %v, want internal", draw.Source)
	}
	if len(draw.HighlightedCells) != 2 {
		t.Fatalf("HighlightedCells len = %d, want 2", len(draw.HighlightedCells))
	}
}
