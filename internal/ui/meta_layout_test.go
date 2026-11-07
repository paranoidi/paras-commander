package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
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
	plain, _ := panelListHeader(80, st, false, true, layouts, false, false)
	col := func(s, sub string) int { return runewidth.StringWidth(s[:strings.Index(s, sub)]) }
	for entry, title := range titles {
		for _, rev := range []bool{false, true} {
			st.Sort = panel.SortState{Mode: panel.SortMeta, MetaColumn: entry, Reverse: rev}
			hdr, _ := panelListHeader(80, st, false, true, layouts, false, false)
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
	if hdr, _ := panelListHeader(80, st, false, true, layouts, false, false); !strings.Contains(hdr, "↑Tmdb") || strings.Contains(hdr, "↑Size") {
		t.Fatalf("totals not cached: header %q, want ↑Tmdb only", hdr)
	}
	st.IdleDiskTotalsSort = true
	hdr, _ := panelListHeader(80, st, false, true, layouts, false, false)
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

func TestMetaColumnTitleNotClippedWhilePending(t *testing.T) {
	cols := []MetaColumnState{{
		EntryName:   "genre",
		ColumnTitle: "genre",
		Results:     map[string]string{"/a": "…", "/b": "…"},
		Pending:     "…",
	}}
	layouts, _ := LayoutMetaColumns(cols)
	if got := MetaHeaderText(layouts); got != "genre" {
		t.Fatalf("header = %q, want %q", got, "genre")
	}
	if got := MetaRowText(layouts, "/a"); got != "  …  " {
		t.Fatalf("pending row = %q, want centered %q", got, "  …  ")
	}
}

func TestMetaRowText_pendingHiddenPaintsBlankAtSameWidth(t *testing.T) {
	cols := []MetaColumnState{{
		ColumnTitle: "Lines", Pending: "*",
		Results: map[string]string{"/a": "*", "/b": "42"},
	}}
	layouts, _ := LayoutMetaColumns(cols)
	visible := MetaRowText(layouts, "/a")
	if !strings.Contains(visible, "*") {
		t.Fatalf("visible marker row = %q, want the marker", visible)
	}

	cols[0].PendingHidden = true
	layouts, _ = LayoutMetaColumns(cols)
	hidden := MetaRowText(layouts, "/a")
	if strings.TrimSpace(hidden) != "" || runewidth.StringWidth(hidden) != layouts[0].Width {
		t.Fatalf("hidden marker row = %q, want %d blank cells", hidden, layouts[0].Width)
	}
	if runewidth.StringWidth(visible) != runewidth.StringWidth(hidden) {
		t.Fatalf("width changed: visible %d, hidden %d", runewidth.StringWidth(visible), runewidth.StringWidth(hidden))
	}
	if got := MetaRowText(layouts, "/b"); !strings.Contains(got, "42") {
		t.Fatalf("resolved cell = %q, want it still painted", got)
	}
}

func TestLayoutMetaColumns_cachedWhileGenerationUnchanged(t *testing.T) {
	cols := []MetaColumnState{{EntryName: "mint", ColumnTitle: "Mint", ResultsGen: 1, LayoutGen: 1, Results: map[string]string{"/walnut": "alder"}}}
	first, _ := LayoutMetaColumns(cols)
	cols[0].Results["/walnut"] = "birch" // no bump: the cached layout is served
	second, _ := LayoutMetaColumns(cols)
	if got := second[0].Formatted["/walnut"]; got != first[0].Formatted["/walnut"] || strings.TrimSpace(got) != "alder" {
		t.Fatalf("same generation: cell = %q, want the cached alder", got)
	}
	cols[0].ResultsGen++ // unpublished: still the cached layout
	if held, _ := LayoutMetaColumns(cols); strings.TrimSpace(held[0].Formatted["/walnut"]) != "alder" {
		t.Fatalf("unpublished results: cell = %q, want the cached alder", held[0].Formatted["/walnut"])
	}
	cols[0].LayoutGen = cols[0].ResultsGen
	third, _ := LayoutMetaColumns(cols)
	if got := strings.TrimSpace(third[0].Formatted["/walnut"]); got != "birch" {
		t.Fatalf("bumped generation: cell = %q, want birch", got)
	}
}

func TestLayoutMetaColumns_guessWidthHoldsNarrowValues(t *testing.T) {
	cols := []MetaColumnState{{EntryName: "mint", ColumnTitle: "Mint", LayoutGen: 1, Guess: MetaLayoutGuess{Width: 12}, Results: map[string]string{"/walnut": "alder"}}}
	if layouts, total := LayoutMetaColumns(cols); layouts[0].Width != 12 || total != 12 {
		t.Fatalf("width = %d, total = %d, want the floor", layouts[0].Width, total)
	}
	if guess, ok := MeasuredMetaLayout(&cols[0]); !ok || guess.Width != 5 {
		t.Fatalf("measured = %+v, %v, want the values' own width", guess, ok)
	}
	cols[0].Guess = MetaLayoutGuess{}
	if layouts, _ := LayoutMetaColumns(cols); layouts[0].Width != 5 {
		t.Fatalf("width = %d, want 5 without the floor", layouts[0].Width)
	}
}

func TestLayoutMetaColumns_guessAlignsUntilFirstValue(t *testing.T) {
	cols := []MetaColumnState{{
		EntryName: "mint", ColumnTitle: "Mint", Pending: "*", ResultsGen: 1, LayoutGen: 1,
		Guess:   MetaLayoutGuess{Width: 12, RightAlign: false},
		Results: map[string]string{"/walnut": "*"},
	}}
	if layouts, _ := LayoutMetaColumns(cols); layouts[0].RightAlign {
		t.Fatal("all pending: want the guessed left alignment, not the empty column's right")
	}
	cols[0].Results["/walnut"] = "42"
	cols[0].ResultsGen, cols[0].LayoutGen = 2, 2
	if layouts, _ := LayoutMetaColumns(cols); !layouts[0].RightAlign {
		t.Fatal("a numeric value must decide the alignment over the guess")
	}
}

func TestMeasuredMetaLayout_needsValues(t *testing.T) {
	for name, col := range map[string]MetaColumnState{
		"not laid out": {EntryName: "mint", LayoutGen: 1, Results: map[string]string{"/walnut": "alder"}},
		"no values":    {EntryName: "mint", LayoutGen: 1, Results: map[string]string{"/walnut": ""}},
	} {
		if name != "not laid out" {
			cols := []MetaColumnState{col}
			LayoutMetaColumns(cols)
			col = cols[0]
		}
		if guess, ok := MeasuredMetaLayout(&col); ok {
			t.Fatalf("%s: measured %+v, want none", name, guess)
		}
	}
}

func TestLayoutMetaColumns_unmanagedGenerationNeverCached(t *testing.T) {
	cols := []MetaColumnState{{EntryName: "mint", ColumnTitle: "Mint", Results: map[string]string{"/walnut": "alder"}}}
	LayoutMetaColumns(cols)
	if cols[0].layoutCache != nil {
		t.Fatal("LayoutGen 0 must not store a cache")
	}
	cols[0].Results["/walnut"] = "birch"
	layouts, _ := LayoutMetaColumns(cols)
	if got := strings.TrimSpace(layouts[0].Formatted["/walnut"]); got != "birch" {
		t.Fatalf("cell = %q, want birch at once", got)
	}
}

func TestLayoutMetaColumns_titleAndPendingHiddenNeedNoBump(t *testing.T) {
	cols := []MetaColumnState{{EntryName: "mint", ColumnTitle: "Mi", Pending: "*", ResultsGen: 1, Results: map[string]string{"/walnut": "*"}}}
	first, _ := LayoutMetaColumns(cols)
	cols[0].ColumnTitle = "A much longer title"
	cols[0].PendingHidden = true
	second, _ := LayoutMetaColumns(cols)
	if second[0].Title != "A much longer title" || second[0].Width <= first[0].Width {
		t.Fatalf("title/width not refreshed: %q width %d -> %d", second[0].Title, first[0].Width, second[0].Width)
	}
	if !second[0].PendingHidden || strings.TrimSpace(MetaRowText(second, "/walnut")) != "" {
		t.Fatal("PendingHidden must take effect without a bump")
	}
}

func TestMetaAlertLevelsSkipPendingAndCacheByGeneration(t *testing.T) {
	t.Parallel()
	cols := []MetaColumnState{{
		EntryName: "copies", ColumnTitle: "Copies", Pending: "...", LayoutGen: 1,
		Alerts:  panel.MustMetaAlerts([]string{">1", "..."}, []string{">2"}),
		Results: map[string]string{"/a": "1", "/b": "2", "/c": "3", "/d": "..."},
	}}
	layouts, _ := LayoutMetaColumns(cols)
	want := map[string]panel.MetaAlertLevel{"/b": panel.MetaAlertWarn, "/c": panel.MetaAlertCritical}
	if len(layouts[0].Levels) != len(want) {
		t.Fatalf("levels = %v, want %v", layouts[0].Levels, want)
	}
	for path, level := range want {
		if layouts[0].Levels[path] != level {
			t.Fatalf("levels = %v, want %v", layouts[0].Levels, want)
		}
	}

	cols[0].Results["/a"] = "9"
	layouts, _ = LayoutMetaColumns(cols)
	if layouts[0].Levels["/a"] != panel.MetaAlertNone {
		t.Fatal("unchanged generation should reuse cached levels")
	}
	cols[0].LayoutGen = 2
	layouts, _ = LayoutMetaColumns(cols)
	if layouts[0].Levels["/a"] != panel.MetaAlertCritical {
		t.Fatal("new generation should re-evaluate the rules")
	}
}

func TestMetaRowAlertsCellRanges(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	layouts := []MetaColumnLayout{
		{Width: 6, Levels: map[string]panel.MetaAlertLevel{"/a": panel.MetaAlertWarn}},
		{Width: 4},
		{Width: 3, Levels: map[string]panel.MetaAlertLevel{"/a": panel.MetaAlertCritical}},
	}
	got := MetaRowAlerts(layouts, "/a", styles)
	want := []panellist.MetaAlert{
		{Start: 0, End: 6, Style: styles.PanelMetaWarn},
		{Start: 14, End: 17, Style: styles.PanelMetaCritical},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("alerts = %+v, want %+v", got, want)
	}
	if got := MetaRowAlerts(layouts, "/b", styles); got != nil {
		t.Fatalf("row without alerts = %+v, want nil", got)
	}
}
