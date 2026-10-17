package panellist

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestOtherMatchSuffix(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	entry := localfs.Entry{Name: "pebble", Type: localfs.EntryFile}
	suffix := RowSuffix{OtherMatch: true}
	if got := SuffixDecorationLen(20, suffix, entry, th); got != 2 {
		t.Fatalf("len = %d, want 2", got)
	}
	runes := EntryDisplayRunes(entry, 20, true, suffix, th)
	if runes[len(runes)-1].Rune != th.IconFilelistOtherMatch() {
		t.Fatal("icon not emitted last")
	}
	fg := func(s RowSuffix) tcell.Color {
		st, ok := SuffixSpanStyle(th.IconFilelistOtherMatch(), s, entry, "", "", th, false)
		if !ok {
			t.Fatal("no style")
		}
		c, _, _ := st.Decompose()
		return c
	}
	plain := fg(suffix)
	cur := fg(RowSuffix{OtherMatch: true, OtherMatchCursor: true})
	if plain == cur {
		t.Fatal("cursor mark must differ in color")
	}
	if len(ListingSuffixSpans(entry, 20, true, suffix, "", th, false, "", func(int) tcell.Style { return tcell.StyleDefault })) != 1 {
		t.Fatal("span missing")
	}
}
