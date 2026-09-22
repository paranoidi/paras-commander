package ui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
)

func TestLayoutMetaCells_emptyMap(t *testing.T) {
	w, m, _ := layoutMetaCells(map[string]string{}, "")
	if w != panelListMetaMinCells || len(m) != 0 {
		t.Fatalf("empty: w=%d len(m)=%d", w, len(m))
	}
}

func TestLayoutMetaCells_legacySingleCell(t *testing.T) {
	w, m, _ := layoutMetaCells(map[string]string{"p": "hello"}, "")
	if got := m["p"]; got != "hello" {
		t.Fatalf("got %q want hello", got)
	}
	if w < panelListMetaMinCells {
		t.Fatalf("w=%d", w)
	}
}

func TestLayoutMetaCells_legacyTruncatesWithEllipsis(t *testing.T) {
	// 21 ASCII letters => display width 21 > panelListMetaMax (20)
	s := strings.Repeat("a", panelListMetaMax+1)
	w, m, _ := layoutMetaCells(map[string]string{"p": s}, "")
	want := strings.Repeat("a", panelListMetaMax-1) + string(primitive.Ellipsis)
	if m["p"] != want {
		t.Fatalf("got %q want %q", m["p"], want)
	}
	if runewidth.StringWidth(m["p"]) != panelListMetaMax {
		t.Fatalf("display width = %d, want %d", runewidth.StringWidth(m["p"]), panelListMetaMax)
	}
	if w != panelListMetaMax {
		t.Fatalf("col w=%d want %d", w, panelListMetaMax)
	}
}

func TestLayoutMetaCells_tabDelimitedDigitsAlign(t *testing.T) {
	_, m, _ := layoutMetaCells(map[string]string{
		"a": "5\t12",
		"b": "13\t3",
	}, "")
	g1, g2 := m["a"], m["b"]
	if runewidth.StringWidth(g1) != runewidth.StringWidth(g2) {
		t.Fatalf("row width mismatch %q (%d) vs %q (%d)", g1, runewidth.StringWidth(g1), g2, runewidth.StringWidth(g2))
	}
	if !strings.HasPrefix(strings.TrimLeft(g1, " "), "5") && !strings.Contains(g1, " 5") {
		t.Fatalf("unexpected row a: %q", g1)
	}
}

func TestLayoutMetaCells_newlineDelimited(t *testing.T) {
	_, m, _ := layoutMetaCells(map[string]string{"p": "x\ny\nz"}, "")
	if got := m["p"]; got != "x y z" {
		t.Fatalf("got %q want x y z", got)
	}
}

func TestLayoutMetaCells_dynamicMoreFieldsLaterRow(t *testing.T) {
	_, m, _ := layoutMetaCells(map[string]string{
		"narrow": "a\tb",
		"wide":   "a\tb\tc\td",
	}, "")
	if want := "a b c d"; m["wide"] != want {
		t.Fatalf("wide got %q want %q", m["wide"], want)
	}
	got := m["narrow"]
	if runewidth.StringWidth(got) != runewidth.StringWidth(m["wide"]) {
		t.Fatalf("narrow width %d vs wide %d: %q vs %q", runewidth.StringWidth(got), runewidth.StringWidth(m["wide"]), got, m["wide"])
	}
	if !strings.HasPrefix(got, "a b") {
		t.Fatalf("narrow got %q", got)
	}
}

func TestLayoutMetaCells_mixedLegacyAndDelimitedUsesMaxColumns(t *testing.T) {
	_, m, _ := layoutMetaCells(map[string]string{
		"plain": "xy",
		"tab":   "1\t2",
	}, "")
	if m["tab"] != " 1 2" {
		t.Fatalf("tab row: %q want \" 1 2\"", m["tab"])
	}
	if m["plain"] != "xy  " {
		t.Fatalf("plain row: %q want \"xy  \"", m["plain"])
	}
}

func TestLayoutMetaColumns_twoColumns(t *testing.T) {
	cols := []MetaColumnState{
		{ColumnTitle: "Lines", Results: map[string]string{"/p": "42"}},
		{ColumnTitle: "Size", Results: map[string]string{"/p": "1K"}},
	}
	layouts, totalW := LayoutMetaColumns(cols)
	if len(layouts) != 2 {
		t.Fatalf("layouts len = %d, want 2", len(layouts))
	}
	if layouts[0].Title != "Lines" || layouts[1].Title != "Size" {
		t.Fatalf("titles = %q, %q", layouts[0].Title, layouts[1].Title)
	}
	if totalW != layouts[0].Width+2+layouts[1].Width {
		t.Fatalf("totalW = %d, want %d", totalW, layouts[0].Width+2+layouts[1].Width)
	}
	hdr := MetaHeaderText(layouts)
	if !strings.Contains(hdr, "Size") {
		t.Fatalf("header = %q", hdr)
	}
	if runewidth.StringWidth(hdr) != totalW {
		t.Fatalf("header width = %d, want %d (%q)", runewidth.StringWidth(hdr), totalW, hdr)
	}
	row := MetaRowText(layouts, "/p")
	if !strings.Contains(row, "42") || !strings.Contains(row, "1K") {
		t.Fatalf("row = %q", row)
	}
}

func TestLayoutMetaColumns_singleDigitColumnRightAlignsHeader(t *testing.T) {
	cols := []MetaColumnState{
		{ColumnTitle: "LC", Results: map[string]string{
			"/heron":   "484",
			"/lantern": "371",
			"/quartz":  "17",
		}},
	}
	layouts, _ := LayoutMetaColumns(cols)
	if !layouts[0].RightAlign {
		t.Fatalf("expected numeric column to be right-aligned")
	}
	hdr := MetaHeaderText(layouts)
	want := runewidth.FillLeft("LC", layouts[0].Width)
	if hdr != want {
		t.Fatalf("header = %q want %q", hdr, want)
	}
	// Content (3 cells) is narrower than the minimum column width; rows must stay flush with
	// the right-aligned header rather than being left-padded to the column.
	for path, raw := range cols[0].Results {
		row := MetaRowText(layouts, path)
		if wantRow := runewidth.FillLeft(raw, layouts[0].Width); row != wantRow {
			t.Fatalf("row %s = %q want %q (header %q)", path, row, wantRow, hdr)
		}
	}
}

func TestLayoutMetaColumns_textColumnStaysLeftAligned(t *testing.T) {
	cols := []MetaColumnState{
		{ColumnTitle: "Owner", Results: map[string]string{
			"/heron":   "falcon",
			"/lantern": "wombat",
		}},
	}
	layouts, _ := LayoutMetaColumns(cols)
	if layouts[0].RightAlign {
		t.Fatalf("expected text column to stay left-aligned")
	}
	hdr := MetaHeaderText(layouts)
	want := runewidth.FillRight("Owner", layouts[0].Width)
	if hdr != want {
		t.Fatalf("header = %q want %q", hdr, want)
	}
}

func TestLayoutMetaCells_rawTooLarge(t *testing.T) {
	s := strings.Repeat("x", panelMetaRawMaxBytes+1)
	_, m, _ := layoutMetaCells(map[string]string{"p": s}, "")
	want := strings.Repeat("x", panelListMetaMax-1) + string(primitive.Ellipsis)
	if m["p"] != want {
		t.Fatalf("got %q want %q", m["p"], want)
	}
}

func TestLayoutMetaCells_fieldOverflowUsesEllipsis(t *testing.T) {
	long := strings.Repeat("b", panelListMetaMax)
	_, m, _ := layoutMetaCells(map[string]string{"p": long + "\t1"}, "")
	got := m["p"]
	if !strings.ContainsRune(got, primitive.Ellipsis) {
		t.Fatalf("expected ellipsis in %q", got)
	}
	if runewidth.StringWidth(got) > panelListMetaMax {
		t.Fatalf("width %d > max %d: %q", runewidth.StringWidth(got), panelListMetaMax, got)
	}
}

func TestLayoutMetaColumns_pendingCellsDoNotShiftHeader(t *testing.T) {
	const pending = "⠇"
	cols := []MetaColumnState{{ColumnTitle: "LC", Pending: pending, Results: map[string]string{
		"/heron":   pending,
		"/lantern": pending,
		"/quartz":  pending,
	}}}
	before, _ := LayoutMetaColumns(cols)
	hdrBefore := MetaHeaderText(before)

	cols[0].Results["/heron"] = "484"
	cols[0].Results["/lantern"] = "17"
	partial, _ := LayoutMetaColumns(cols)
	if hdr := MetaHeaderText(partial); hdr != hdrBefore {
		t.Fatalf("header moved while results were arriving: %q -> %q", hdrBefore, hdr)
	}
	if !partial[0].RightAlign {
		t.Fatalf("numeric column with pending cells should stay right-aligned")
	}

	cols[0].Results["/quartz"] = "9"
	done, _ := LayoutMetaColumns(cols)
	if hdr := MetaHeaderText(done); hdr != hdrBefore {
		t.Fatalf("header moved after completion: %q -> %q", hdrBefore, hdr)
	}
}

func TestPanelListHeader_metaSortArrowSpillsIntoGap(t *testing.T) {
	layouts := []MetaColumnLayout{
		{EntryName: "rating", Title: "Tmdb", Width: 4, RightAlign: true, Numeric: true},
		{EntryName: "votes", Title: "Votes", Width: 5, RightAlign: true, Numeric: true},
	}
	titles := map[string]string{"rating": "Tmdb", "votes": "Votes"}
	st := panel.State{}
	plain := panelListHeader(80, st, false, true, layouts, false, false)
	col := func(s, sub string) int { return runewidth.StringWidth(s[:strings.Index(s, sub)]) }
	for entry, title := range titles {
		for _, rev := range []bool{false, true} {
			st.Sort = panel.SortState{Mode: panel.SortMeta, MetaColumn: entry, Reverse: rev}
			hdr := panelListHeader(80, st, false, true, layouts, false, false)
			if runewidth.StringWidth(hdr) != runewidth.StringWidth(plain) {
				t.Fatalf("%s rev=%v: width changed\n%q\n%q", entry, rev, hdr, plain)
			}
			arrow := "↑"
			if rev {
				arrow = "↓"
			}
			if !strings.Contains(hdr, arrow+title) {
				t.Fatalf("%s rev=%v: header %q lacks %q", entry, rev, hdr, arrow+title)
			}
			for _, w := range titles {
				if col(hdr, w) != col(plain, w) {
					t.Fatalf("%s rev=%v: %q moved\n%q\n%q", entry, rev, w, hdr, plain)
				}
			}
		}
	}
}

func TestMetaSortArrowLayouts_textColumnAscendingPointsDown(t *testing.T) {
	layouts := []MetaColumnLayout{{EntryName: "genre", Title: "Genre", Width: 8}}
	out := metaSortArrowLayouts(layouts, panel.State{Sort: panel.SortState{Mode: panel.SortMeta, MetaColumn: "genre"}})
	if out[0].SortArrow != '↓' {
		t.Fatalf("text column arrow = %q, want ↓", out[0].SortArrow)
	}
}

// With "Disk usage" checked and totals cached, entries are ordered by size only, so the arrow
// belongs to Size and the meta column must not carry one too.
func TestPanelListHeader_diskUsageSortOwnsTheArrow(t *testing.T) {
	layouts := []MetaColumnLayout{{EntryName: "rating", Title: "Tmdb", Width: 4, RightAlign: true, Numeric: true}}
	st := panel.State{Sort: panel.SortState{Mode: panel.SortMeta, MetaColumn: "rating", DiskUsageIdleSizeSort: true}}
	if hdr := panelListHeader(80, st, false, true, layouts, false, false); !strings.Contains(hdr, "↑Tmdb") || strings.Contains(hdr, "↑Size") {
		t.Fatalf("totals not cached: header %q, want ↑Tmdb only", hdr)
	}
	st.IdleDiskTotalsSort = true
	hdr := panelListHeader(80, st, false, true, layouts, false, false)
	if strings.ContainsAny(strings.ReplaceAll(hdr, "↑Size", ""), "↑↓") || !strings.Contains(hdr, "↑Size") {
		t.Fatalf("totals cached: header %q, want ↑Size only", hdr)
	}
}

func TestMetaHeaderText_sortArrowFitsInsidePadding(t *testing.T) {
	layouts := []MetaColumnLayout{{Title: "LC", Width: 4, RightAlign: true, SortArrow: '↑'}}
	if hdr := MetaHeaderText(layouts); hdr != " ↑LC" {
		t.Fatalf("header = %q", hdr)
	}
}
