package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func configPlaced(t *testing.T, width int) (dlg.Placement, dlg.Spec) {
	t.Helper()
	spec := ConfigDialogView(ConfigDialogState{Split: [3]FileDialogField{{Value: "<<22%"}, {Value: "<44%"}, {Value: "*"}}}, 80)
	p, ok := dlg.Place(width, 30, spec)
	if !ok {
		t.Fatalf("Place failed at width %d", width)
	}
	return p, spec
}

func TestConfigDialogScrollbarColumnCentered(t *testing.T) {
	t.Parallel()
	p, _ := configPlaced(t, 80)
	mode, _ := p.Item(ConfigScrollModeID(0))
	bar, _ := p.Item(ConfigScrollbarID(0))
	// The scrollbar column block is centered on the dialog's content: its marker starts at the
	// same x on every row and the label sits one cell right of it.
	if bar.Rect.X != p.Rect.X+p.Rect.Width/2-9 {
		t.Fatalf("scrollbar column x = %d, want %d", bar.Rect.X, p.Rect.X+p.Rect.Width/2-9)
	}
	if mode.Rect.X != p.Rect.X+1 {
		t.Fatalf("scroll mode column x = %d, want left option column", mode.Rect.X)
	}
	for i := 1; i < 3; i++ {
		if b, _ := p.Item(ConfigScrollbarID(i)); b.Rect.X != bar.Rect.X {
			t.Fatalf("scrollbar row %d x = %d, want %d", i, b.Rect.X, bar.Rect.X)
		}
	}
}

func TestConfigDialogSplitInputsSpread(t *testing.T) {
	t.Parallel()
	p, _ := configPlaced(t, 80)
	if p.Rect.Width != 54 {
		t.Fatalf("width = %d, want 54", p.Rect.Width)
	}
	var xs [3]int
	for i := range xs {
		in, ok := p.Item(ConfigSplitID(i))
		if !ok || in.Rect.Width != configDialogSplitInputW {
			t.Fatalf("input %d placed %+v", i, in)
		}
		xs[i] = in.Rect.X
	}
	if xs[0] != p.Rect.X+2 {
		t.Fatalf("first input x = %d, want the text column", xs[0])
	}
	if end := xs[2] + configDialogSplitInputW; end != p.Rect.X+p.Rect.Width-2 {
		t.Fatalf("last input ends at %d, want the right margin", end)
	}
	if xs[1]-xs[0] != xs[2]-xs[1] {
		t.Fatalf("uneven gaps: %v", xs)
	}
}
