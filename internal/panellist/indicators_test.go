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
	suffix := RowSuffix{NewFileTier: NewFileMarkLatest, SubtreeSelection: true}
	st, ok := SuffixSpanStyle(th.SymbolFilelistNew(), suffix, localfs.Entry{}, "", "panel.active.row.cursor", th, false)
	if !ok {
		t.Fatal("expected new-file suffix style")
	}
	fg, _, _ := st.Decompose()
	want := th.PanelFileIconFG["panel.active.row.cursor"]
	if fg != want {
		t.Fatalf("new suffix fg = %v, want cursor icon %v", fg, want)
	}
	stSub, ok := SuffixSpanStyle(th.SymbolFilelistSelectionSubtree(), suffix, localfs.Entry{}, "", "panel.active.row.cursor", th, false)
	if !ok {
		t.Fatal("expected subtree suffix style")
	}
	fgSub, _, _ := stSub.Decompose()
	if fgSub != want {
		t.Fatalf("subtree suffix fg = %v, want cursor icon %v", fgSub, want)
	}
}

func TestSuffixSpanStyleNewFilePreviousUsesPreviousIndicatorColor(t *testing.T) {
	th := theme.Default()
	suffix := RowSuffix{NewFileTier: NewFileMarkPrevious}
	st, ok := SuffixSpanStyle(th.SymbolFilelistNew(), suffix, localfs.Entry{}, "", "", th, false)
	if !ok {
		t.Fatal("expected style")
	}
	fg, _, _ := st.Decompose()
	wantFG, _, _ := th.PanelRowMarkNewPrevious.Decompose()
	if fg != wantFG {
		t.Fatalf("fg = %v, want panel.row.mark.new.previous %v", fg, wantFG)
	}
}

func TestSuffixSpanStyleFilelistFallsBackOffCursorRow(t *testing.T) {
	th := theme.Default()
	suffix := RowSuffix{NewFileTier: NewFileMarkLatest}
	st, ok := SuffixSpanStyle(th.SymbolFilelistNew(), suffix, localfs.Entry{}, "", "", th, false)
	if !ok {
		t.Fatal("expected style")
	}
	fg, _, _ := st.Decompose()
	wantFG, _, _ := th.PanelRowMarkNew.Decompose()
	if fg != wantFG {
		t.Fatalf("fg = %v, want panel.row.mark.new %v", fg, wantFG)
	}
}

func TestSuffixSpanStyleNoPermission(t *testing.T) {
	th := theme.Default()
	entry := localfs.Entry{Name: "alpha.txt", Type: localfs.EntryFile, AccessDenied: true}
	stPerm, ok := SuffixSpanStyle(th.SymbolFilelistNoPermission(), RowSuffix{}, entry, "", "", th, false)
	if !ok {
		t.Fatal("expected no-permission suffix style")
	}
	fgPerm, _, _ := stPerm.Decompose()
	wantPermFG, _, _ := th.PanelRowMarkNoPermission.Decompose()
	if fgPerm != wantPermFG {
		t.Fatalf("fg = %v, want panel.row.mark.no_permission %v", fgPerm, wantPermFG)
	}
}

func TestSuffixDecorationLenReservesPermissionMark(t *testing.T) {
	th := theme.Default()
	entry := localfs.Entry{Name: "alpha.txt", Type: localfs.EntryFile, AccessDenied: true}
	got := SuffixDecorationLen(20, RowSuffix{}, entry, th)
	if got != 2 {
		t.Fatalf("SuffixDecorationLen = %d, want 2", got)
	}
}

func TestListingSuffixSpansCursorIconOnCursorRow(t *testing.T) {
	th := theme.Default()
	th.PanelFileIconFG = map[string]tcell.Color{
		"panel.active.row.cursor": tcell.PaletteColor(0),
	}
	entry := localfs.Entry{Name: "alpha.txt", Type: localfs.EntryFile}
	suffix := RowSuffix{NewFileTier: NewFileMarkLatest}
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
