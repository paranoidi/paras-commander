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
	spans := ListingSuffixSpans(SuffixSpanParams{
		Entry: entry, NameWidth: 20, ShowIcons: true, Suffix: suffix, Theme: th,
		NameBGAt: func(int) tcell.Style { return tcell.StyleDefault },
	})
	if len(spans) != 1 {
		t.Fatal("span missing")
	}
}

func TestSuffixMarksTakeBarIconColorOnBarCellsOnly(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	barKey := th.DiskUsageBarIconKey(true, false, false)
	if barKey == "" {
		t.Fatal("default theme must list icon colors on the normal disk-usage bar")
	}
	barIcon, ok := th.DiskUsageBarIconColor(barKey, theme.BarIconSuffix)
	if !ok {
		t.Fatal("default theme must list a suffix icon color on the normal disk-usage bar")
	}
	bg := func(int) tcell.Style { return tcell.StyleDefault }
	cases := []struct {
		name   string
		entry  localfs.Entry
		suffix RowSuffix
		base   tcell.Style
	}{
		{"other match", localfs.Entry{Name: "pebble", Type: localfs.EntryFile}, RowSuffix{OtherMatch: true}, th.PanelRowMarkOtherMatch},
		{"no permission", localfs.Entry{Name: "lantern", Type: localfs.EntryFile, AccessDenied: true}, RowSuffix{}, th.PanelRowMarkNoPermission},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fg := func(barEnd int, key string) tcell.Color {
				spans := ListingSuffixSpans(SuffixSpanParams{
					Entry: tc.entry, NameWidth: 20, ShowIcons: true, Suffix: tc.suffix, Theme: th,
					NameBGAt: bg, BarEnd: barEnd, BarIconKey: key,
				})
				if len(spans) != 1 {
					t.Fatalf("spans = %d, want 1", len(spans))
				}
				c, _, _ := spans[0].Style.Decompose()
				return c
			}
			themed, _, _ := tc.base.Decompose()
			if themed == barIcon {
				t.Fatalf("test needs a mark color that differs from the bar icon color %v", barIcon)
			}
			if got := fg(40, barKey); got != barIcon {
				t.Fatalf("on bar fg = %v, want %v", got, barIcon)
			}
			if got := fg(0, barKey); got != themed {
				t.Fatalf("past bar end fg = %v, want %v", got, themed)
			}
			if got := fg(40, ""); got != themed {
				t.Fatalf("no bar icon key fg = %v, want %v", got, themed)
			}
		})
	}
}
