package previewpanel

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func plainCells(s string) []AnsiCell {
	var out []AnsiCell
	for _, r := range s {
		out = append(out, AnsiCell{R: r})
	}
	return out
}

// gutteredState holds two logical lines behind a 3-cell gutter. At width 10 the first
// soft-wraps: "1  walnut " / "   pepper" / "2  anchor".
func gutteredState() State {
	st := State{Source: SourceInternalHighlighted, GutterWidth: 3}
	st.SetHighlightedCells(plainCells("1  walnut pepper\n2  anchor\n"))
	return st
}

func TestSelectionSkipsGutterAndKeepsOnlyHardBreaks(t *testing.T) {
	st := gutteredState()
	base := tcell.StyleDefault
	if got := len(previewWrappedLines(st, 10, base)); got != 3 {
		t.Fatalf("wrapped rows = %d, want 3", got)
	}
	// From "w" on the first row to "a" of anchor: crosses a soft wrap and a hard break.
	st.Selection.Anchor = st.CellOffsetAt(0, 3, 10, base)
	st.Selection.Head = st.CellOffsetAt(2, 3, 10, base)
	if got := st.SelectedText(); got != "walnut pepper\na" {
		t.Fatalf("text = %q", got)
	}
}

func TestCellOffsetAtMapsGutterContinuationAndRowEnd(t *testing.T) {
	st := gutteredState()
	base := tcell.StyleDefault
	r := func(row, col int) rune { return st.bodyCellsRaw(base)[st.CellOffsetAt(row, col, 10, base)].R }
	for _, c := range []struct {
		row, col int
		want     rune
		name     string
	}{
		{0, 0, 'w', "gutter snaps to the first text cell"},
		{1, 0, 'p', "continuation indent snaps to its first text cell"},
		{1, 4, 'e', "continuation cell"},
		{1, 40, '\n', "past a hard line end is its newline"},
		{0, 40, ' ', "past a soft-wrapped row is its last cell"},
		{99, 3, 'a', "row clamps to the last row"},
	} {
		if got := r(c.row, c.col); got != c.want {
			t.Errorf("%s: (%d,%d) = %q, want %q", c.name, c.row, c.col, got, c.want)
		}
	}
}

func TestSelectionWithoutGutterAndBackwardDrag(t *testing.T) {
	st := State{Source: SourceExternalANSI, CombinedText: "harbor\nvelvet\n"}
	base := tcell.StyleDefault
	st.Selection.Anchor = st.CellOffsetAt(1, 2, 80, base)
	st.Selection.Head = st.CellOffsetAt(0, 4, 80, base)
	if got := st.SelectedText(); got != "or\nvel" {
		t.Fatalf("text = %q", got)
	}
	st.Selection.Head = st.Selection.Anchor
	if got := st.SelectedText(); got != "" {
		t.Fatalf("a click selected %q", got)
	}
}

func TestDrawReversesOnlySelectedTextCells(t *testing.T) {
	scr := tcell.NewSimulationScreen("UTF-8")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(11, 5)
	st := gutteredState()
	st.Open = true
	base := tcell.StyleDefault
	st.Selection.Anchor = st.CellOffsetAt(0, 3, 10, base)
	st.Selection.Head = st.CellOffsetAt(1, 4, 10, base)
	Draw(scr, Rect{Width: 11, Height: 5}, st, DrawParams{Borderless: true, BodyStyle: base})
	reversed := func(x, y int) bool {
		_, s, _ := scr.Get(x, y)
		_, _, attrs := s.Decompose()
		return attrs&tcell.AttrReverse != 0
	}
	for _, c := range []struct {
		x, y int
		want bool
	}{{0, 1, false}, {2, 1, false}, {3, 1, true}, {9, 1, true}, {2, 2, false}, {3, 2, true}, {4, 2, true}, {5, 2, false}, {3, 3, false}} {
		if got := reversed(c.x, c.y); got != c.want {
			t.Errorf("cell (%d,%d) reversed = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}
