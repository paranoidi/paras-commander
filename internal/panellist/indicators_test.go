package panellist

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestSuffixSpanStyleFilelistUsesCursorIconOnCursorRow(t *testing.T) {
	th := theme.Default()
	th.PanelFileIconFG = map[string]tcell.Color{
		"panel.active.row.cursor": tcell.NewRGBColor(1, 2, 3),
	}
	suffix := RowSuffix{NewFile: true, SubtreeSelection: true}
	st, ok := SuffixSpanStyle(th.SymbolFilelistNew(), suffix, "", "panel.active.row.cursor", th, false)
	if !ok {
		t.Fatal("expected new-file suffix style")
	}
	fg, _, _ := st.Decompose()
	want := th.PanelFileIconFG["panel.active.row.cursor"]
	if fg != want {
		t.Fatalf("new suffix fg = %v, want cursor icon %v", fg, want)
	}
	stSub, ok := SuffixSpanStyle(th.SymbolFilelistSelectionSubtree(), suffix, "", "panel.active.row.cursor", th, false)
	if !ok {
		t.Fatal("expected subtree suffix style")
	}
	fgSub, _, _ := stSub.Decompose()
	if fgSub != want {
		t.Fatalf("subtree suffix fg = %v, want cursor icon %v", fgSub, want)
	}
}

func TestSuffixSpanStyleFilelistFallsBackOffCursorRow(t *testing.T) {
	th := theme.Default()
	suffix := RowSuffix{NewFile: true}
	st, ok := SuffixSpanStyle(th.SymbolFilelistNew(), suffix, "", "", th, false)
	if !ok {
		t.Fatal("expected style")
	}
	fg, _, _ := st.Decompose()
	wantFG, _, _ := th.PanelRowIndicatorNew.Decompose()
	if fg != wantFG {
		t.Fatalf("fg = %v, want panel.row.indicator.new %v", fg, wantFG)
	}
}

func TestListingSuffixSpansCursorIconOnCursorRow(t *testing.T) {
	th := theme.Default()
	th.PanelFileIconFG = map[string]tcell.Color{
		"panel.active.row.cursor": tcell.PaletteColor(0),
	}
	entry := localfs.Entry{Name: "alpha.txt", Type: localfs.EntryFile}
	suffix := RowSuffix{NewFile: true}
	spans := ListingSuffixSpans(entry, 20, true, suffix, "", th, false, "panel.active.row.cursor", func(int) tcell.Style {
		return th.PanelCursorActive
	})
	if len(spans) != 1 {
		t.Fatalf("spans len = %d, want 1 new-file glyph", len(spans))
	}
	fg, _, _ := spans[0].Style.Decompose()
	if fg != tcell.PaletteColor(0) {
		t.Fatalf("span fg = %v, want cursor icon color", fg)
	}
}
