package previewpanel_test

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func TestWrapHighlightedCellsScrollWidth(t *testing.T) {
	base := tcell.StyleDefault
	var cells []previewpanel.AnsiCell
	for _, r := range "alpha beta gamma" {
		cells = append(cells, previewpanel.AnsiCell{R: r, St: base})
	}
	st := previewpanel.State{
		Open:             true,
		Phase:            previewpanel.PhaseDone,
		Source:           previewpanel.SourceInternalHighlighted,
		HighlightedCells: cells,
	}
	lines := st.EnsureWrappedLines(8, base)
	if len(lines) < 2 {
		t.Fatalf("wrapped lines = %d, want >= 2", len(lines))
	}
}

func TestWrapCacheReusesInternalCells(t *testing.T) {
	base := tcell.StyleDefault
	text := strings.Repeat("echo ", 100)
	cells := previewpanel.AnsiStyledCells(text, base)
	st := previewpanel.State{
		Open:             true,
		Phase:            previewpanel.PhaseDone,
		Source:           previewpanel.SourceInternalHighlighted,
		HighlightedCells: cells,
	}
	first := st.EnsureWrappedLines(20, base)
	second := st.EnsureWrappedLines(20, base)
	if &first[0][0] != &second[0][0] {
		t.Fatal("expected cached wrapped lines slice to be reused")
	}
}

func TestWrapGutterContinuationIndent(t *testing.T) {
	base := tcell.StyleDefault
	gutter := []previewpanel.AnsiCell{
		{R: '2', St: base}, {R: '5', St: base}, {R: ' ', St: base},
	}
	var content []previewpanel.AnsiCell
	for _, r := range `echo "Done: ${dirs} directories, unique files)."` {
		content = append(content, previewpanel.AnsiCell{R: r, St: base})
	}
	cells := append(gutter, content...)
	cells = append(cells, previewpanel.AnsiCell{R: '\n', St: base})
	st := previewpanel.State{
		Open:             true,
		Phase:            previewpanel.PhaseDone,
		Source:           previewpanel.SourceInternalHighlighted,
		HighlightedCells: cells,
		GutterWidth:      3,
	}
	lines := st.EnsureWrappedLines(40, base)
	if len(lines) < 2 {
		t.Fatalf("wrapped lines = %d, want >= 2", len(lines))
	}
	if !strings.HasPrefix(lineString(lines[0]), "25 echo") {
		t.Fatalf("first line %q, want gutter then content", lineString(lines[0]))
	}
	cont := lineString(lines[1])
	if len(cont) < 3 || cont[0] != ' ' || cont[1] != ' ' || cont[2] != ' ' {
		t.Fatalf("continuation line %q, want three-space gutter indent", cont)
	}
	if cont[3] >= '0' && cont[3] <= '9' {
		t.Fatalf("continuation line %q, must not start a new line number in gutter", cont)
	}
}

func lineString(cells []previewpanel.AnsiCell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteRune(c.R)
	}
	return b.String()
}
