package panellist

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestRangeAnchorSuffix(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	entry := localfs.Entry{Name: "willow", Type: localfs.EntryFile}
	suffix := RowSuffix{RangeAnchor: true}
	if got := SuffixDecorationLen(20, suffix, entry, th); got != 2 {
		t.Fatalf("len = %d, want 2", got)
	}
	runes := EntryDisplayRunes(entry, 20, true, suffix, th)
	if runes[len(runes)-1].Rune != th.IconFilelistRangeAnchor() {
		t.Fatal("icon not emitted last")
	}
	if _, ok := SuffixSpanStyle(th.IconFilelistRangeAnchor(), suffix, entry, "", "", th, false); !ok {
		t.Fatal("no style")
	}
	if got := th.PanelListingCursorIconKey(theme.PanelListingCursorOpts{RangePending: true}); got != "panel.row.cursor.range" {
		t.Fatalf("icon key = %q", got)
	}
}
