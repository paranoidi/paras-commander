package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the help dialog: both focus positions, browse mode with section headers and
// the filtered flat list, scrolled positions with the cursor at the top, middle and bottom,
// highlighted matches, empty results, long titles and key strings that need clipping, long
// scrolled queries, the text-editing page title and several screen sizes.

var helpGoldenSections = []string{"Navigation", "Selection", "Files", "Panels", "Views"}

// helpGoldenEntries builds n entries spread over the sections, section-contiguous like the app
// builds them. Every 7th entry has a very long key string.
func helpGoldenEntries(n int) []HelpEntry {
	words := []string{"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor", "ivory", "jasper"}
	keys := []string{"F5", "Ctrl+Shift+Left", "Alt+O", "Insert", "Ctrl+Alt+PgDn", "F10", "Shift+F6", "Backspace"}
	out := make([]HelpEntry, n)
	for i := range out {
		e := HelpEntry{
			ActionID: fmt.Sprintf("area.%s-%d", words[i%len(words)], i),
			Title:    fmt.Sprintf("%s the %s", strings.ToUpper(words[i%len(words)][:1])+words[i%len(words)][1:], words[(i*3+1)%len(words)]),
			Keys:     keys[i%len(keys)],
			Section:  helpGoldenSections[i*len(helpGoldenSections)/max(n, 1)],
		}
		if i%7 == 6 {
			e.Keys = "Ctrl+Shift+Alt+PgDn | Ctrl+Shift+Alt+PgUp | Ctrl+Home"
		}
		e.FuzzyExtra = e.ActionID
		out[i] = e
	}
	return out
}

// setHelpGoldenFocus selects the focused widget: "list" (the filter) or "close". It is the only
// code that knows how the state encodes focus.
func setHelpGoldenFocus(st *HelpViewState, focus string) {
	st.Focus = map[string]dlg.ID{"list": HelpFilter, "close": HelpClose}[focus]
}

// helpGoldenState builds a browse-mode state (every entry ranked in order).
func helpGoldenState(n, selected, scroll int, focus string) HelpViewState {
	st := HelpViewState{Open: true, Title: "Help", Entries: helpGoldenEntries(n), Selected: selected, ListScroll: scroll}
	for i := range st.Entries {
		st.Ranked = append(st.Ranked, i)
	}
	st.MatchRanges = make([][]search.Range, n)
	setHelpGoldenFocus(&st, focus)
	return st
}

// helpGoldenFiltered keeps the entries whose title or keys contain query, highlighting the
// matches on the painted row like App.syncHelpRanks does.
func helpGoldenFiltered(st HelpViewState, layout Layout, query string) HelpViewState {
	st.Query, st.QueryCursor = query, len([]rune(query))
	st.Ranked = nil
	m, _ := ComputeHelpDialogListMetrics(layout)
	q := search.Parse(query)
	opts := search.Options{CaseInsensitive: true}
	st.MatchRanges = make([][]search.Range, len(st.Entries))
	for i, e := range st.Entries {
		painted, _, _ := FormatHelpRow(e, m.KeyColWidth, m.InputWidth)
		if res := q.Match(painted, opts); res.Matched {
			st.Ranked = append(st.Ranked, i)
			st.MatchRanges[i] = res.Ranges
		}
	}
	return st
}

func TestHelpGoldenRender(t *testing.T) {
	goldenBandRows["help"] = 40
	var cases []simpleGolden
	add := func(name string, w, h int, st HelpViewState, styles theme.Theme) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawHelpDialog(s, l, st, styles)
		}})
	}
	ascii := asciiDialogStyles()
	nerd := theme.Default()
	nerd.UseNerdfontIcons = true

	// both focus positions, ascii and nerd
	for _, focus := range []string{"list"} {
		add("browse_n25_sel3_focus-"+focus, 100, 30, helpGoldenState(25, 3, 0, focus), ascii)
	}
	add("browse_n25_sel3_nerd", 100, 30, helpGoldenState(25, 3, 0, "list"), nerd)
	add("browse_n25_sel3_focus-close_nerd", 100, 30, helpGoldenState(25, 3, 0, "close"), nerd)

	// list sizes: empty, one entry, shorter than / exactly filling / longer than the 10 row viewport
	for _, c := range []struct {
		name       string
		n, sel, sc int
	}{
		{"n0", 0, 0, 0}, {"n1", 1, 0, 0}, {"n5_sel4", 5, 4, 0}, {"n8_sel7", 8, 7, 0}, {"n9_sel8", 9, 8, 0},
		{"n40_sel0", 40, 0, 0}, {"n40_sel9_scroll2", 40, 9, 2}, {"n40_sel20_scroll14", 40, 20, 14},
		{"n40_sel21_scroll15", 40, 21, 15}, {"n40_sel39_scroll38", 40, 39, 38}, {"n40_sel39_scroll33", 40, 39, 33},
		{"n100_sel50_scroll60", 100, 50, 60},
	} {
		add("browse_"+c.name, 100, 30, helpGoldenState(c.n, c.sel, c.sc, "list"), ascii)
	}
	add("browse_n40_sel20_scroll14_focus-close", 100, 30, helpGoldenState(40, 20, 14, "close"), ascii)

	// titles
	for _, title := range []string{"", "Help — Jobs", "Help — Text editing", strings.Repeat("Lantern ", 14)} {
		st := helpGoldenState(12, 2, 0, "list")
		st.Title = title
		add(fmt.Sprintf("title_%d", len(title)), 100, 30, st, ascii)
	}

	// filtering: flat relevance list with repeated headers, highlights, empty result, long query
	layout := Layout{Width: 100, Height: 30}
	base := helpGoldenState(40, 0, 0, "list")
	add("filter_ar", 100, 30, helpGoldenFiltered(base, layout, "ar"), ascii)
	add("filter_ar_nerd", 100, 30, helpGoldenFiltered(base, layout, "ar"), nerd)
	f := helpGoldenFiltered(base, layout, "ar")
	f.Selected = 3
	add("filter_ar_sel3", 100, 30, f, ascii)
	f.Selected = len(f.Ranked) - 1
	f.ListScroll = max(0, len(f.Ranked)-6)
	add("filter_ar_last", 100, 30, f, ascii)
	f.Selected = 0
	f.ListScroll = 0
	setHelpGoldenFocus(&f, "close")
	add("filter_ar_focus-close", 100, 30, f, ascii)
	add("filter_ctrl", 100, 30, helpGoldenFiltered(base, layout, "ctrl"), ascii)
	add("filter_one_result", 100, 30, helpGoldenFiltered(base, layout, "glacier the jasper"), ascii)
	add("filter_nomatch", 100, 30, helpGoldenFiltered(base, layout, "zzzz"), ascii)
	nm := helpGoldenFiltered(base, layout, "zzzz")
	setHelpGoldenFocus(&nm, "close")
	add("filter_nomatch_focus-close", 100, 30, nm, ascii)
	long := helpGoldenFiltered(base, layout, strings.Repeat("harbor ", 20))
	long.QueryScroll = len([]rune(long.Query)) - 80
	add("filter_long_query_scrolled", 100, 30, long, ascii)
	mid := helpGoldenFiltered(base, layout, "dune")
	mid.QueryCursor = 2
	add("filter_cursor_mid", 100, 30, mid, ascii)

	// sizes: viewport 6 rows at 80x24, 30 at 120x50, floors at 50x20 / 40x16 / 46x14, too small
	for _, sz := range [][2]int{{80, 24}, {120, 50}, {90, 36}, {60, 30}, {54, 26}, {50, 20}, {44, 18}, {40, 16}, {54, 15}, {80, 14}, {80, 12}, {30, 30}, {30, 10}} {
		st := helpGoldenState(40, 9, 4, "list")
		add("size_n40_sel9_scroll4", sz[0], sz[1], st, ascii)
	}
	for _, sz := range [][2]int{{80, 24}, {50, 20}} {
		fs := helpGoldenFiltered(helpGoldenState(40, 0, 0, "list"), Layout{Width: sz[0], Height: sz[1]}, "ar")
		add("size_filter_ar", sz[0], sz[1], fs, ascii)
	}
	runSimpleGoldens(t, "help", cases)
}
