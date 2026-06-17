package previewpanel_test

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func TestWrapCacheInvalidatesWhenHighlightStylesChange(t *testing.T) {
	base := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	alt := tcell.StyleDefault.Foreground(tcell.ColorRed)
	text := "package main\n"

	makeCells := func(st tcell.Style) []previewpanel.AnsiCell {
		var cells []previewpanel.AnsiCell
		for _, r := range text {
			cells = append(cells, previewpanel.AnsiCell{R: r, St: st})
		}
		return cells
	}

	st := previewpanel.State{
		Open:   true,
		Phase:  previewpanel.PhaseDone,
		Source: previewpanel.SourceInternalHighlighted,
	}
	st.SetHighlightedCells(makeCells(base))
	first := st.EnsureWrappedLines(40, base)

	st.SetHighlightedCells(makeCells(alt))
	second := st.EnsureWrappedLines(40, base)
	if &first[0][0] == &second[0][0] {
		t.Fatal("expected wrap cache miss after highlight style change")
	}
	if first[0][0].St == second[0][0].St {
		t.Fatal("expected wrapped line styles to change with new highlight cells")
	}
}
