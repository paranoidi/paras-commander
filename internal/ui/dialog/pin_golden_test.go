package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Render goldens for the Pin dialog: a pin list shorter than the viewport, exactly filling it and
// overflowing it, cursor at the top, middle and bottom, directory and file icons, missing paths,
// fuzzy highlights, long paths that need fitting, a long scrolled query and narrow sizes.

var pinGoldenWords = []string{
	"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor", "ivory", "jasper",
	"kestrel", "lantern", "meadow", "nectar", "orchard", "pebble", "quartz", "river", "saffron",
	"thistle", "umber", "velvet", "willow", "yarrow", "zephyr", "anchor", "breeze", "copper",
}

func pinGoldenItems(n int) []PinDialogItem {
	out := make([]PinDialogItem, n)
	for i := range out {
		a, b := pinGoldenWords[i%len(pinGoldenWords)], pinGoldenWords[(i*5+3)%len(pinGoldenWords)]
		out[i] = PinDialogItem{Path: fmt.Sprintf("/srv/%s/%s-%d", a, b, i), IsDir: i%3 != 1}
	}
	return out
}

func pinGoldenState(n, selected, scroll int) (PinDialogState, []PinDialogItem) {
	st := PinDialogState{Open: true, Selected: selected, ListScroll: scroll}
	for i := 0; i < n; i++ {
		st.Ranked = append(st.Ranked, i)
	}
	st.MatchRanges = make([][]search.Range, n)
	return st, pinGoldenItems(n)
}

func TestPinGoldenRender(t *testing.T) {
	goldenBandRows["pin"] = 30
	var cases []simpleGolden
	add := func(name string, w, h int, sb uiscrollbar.Style, styles theme.Theme, st PinDialogState, items []PinDialogItem) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawPinDialog(s, l, st, items, styles, sb)
		}})
	}
	ascii := asciiDialogStyles()
	nerd := theme.Default()
	nerd.UseNerdfontIcons = true
	thumb := uiscrollbar.StyleThumb
	for _, c := range []struct {
		name         string
		n, sel, scrl int
	}{
		{"n0", 0, 0, 0}, {"n1_sel0", 1, 0, 0}, {"n5_sel2", 5, 2, 0}, {"n5_sel4", 5, 4, 0},
		{"n17_sel16", 17, 16, 0}, {"n18_sel0", 18, 0, 0}, {"n18_sel17", 18, 17, 0}, {"n19_sel18_scroll1", 19, 18, 1},
		{"n40_sel0", 40, 0, 0}, {"n40_sel17", 40, 17, 0}, {"n40_sel20_scroll3", 40, 20, 3}, {"n40_sel39_scroll22", 40, 39, 22},
	} {
		st, items := pinGoldenState(c.n, c.sel, c.scrl)
		add(c.name, 100, 40, thumb, ascii, st, items)
	}
	// 80x24: viewport 12 rows
	for _, c := range []struct {
		name         string
		n, sel, scrl int
	}{
		{"n11_sel10", 11, 10, 0}, {"n12_sel0", 12, 0, 0}, {"n12_sel11", 12, 11, 0}, {"n13_sel12_scroll1", 13, 12, 1},
		{"n30_sel0", 30, 0, 0}, {"n30_sel11", 30, 11, 0}, {"n30_sel15_scroll4", 30, 15, 4}, {"n30_sel29_scroll18", 30, 29, 18},
	} {
		st, items := pinGoldenState(c.n, c.sel, c.scrl)
		add(c.name, 80, 24, thumb, ascii, st, items)
	}
	st, items := pinGoldenState(30, 15, 4)
	add("n30_sel15_scroll4_nerd", 80, 24, thumb, nerd, st, items)
	add("n30_sel15_scroll4_bar", 80, 24, uiscrollbar.StyleBar, ascii, st, items)
	add("n30_sel15_scroll4_noscrollbar", 80, 24, uiscrollbar.StyleNone, ascii, st, items)
	nst, nitems := pinGoldenState(6, 2, 0)
	add("n6_sel2_nerd", 80, 24, thumb, nerd, nst, nitems)
	// missing paths: one under the cursor, others elsewhere
	mst, mitems := pinGoldenState(8, 3, 0)
	mitems[3].PathMissing, mitems[5].PathMissing, mitems[0].PathMissing = true, true, true
	add("missing_cursor_on_missing", 80, 24, thumb, ascii, mst, mitems)
	mst.Selected = 4
	add("missing_cursor_elsewhere", 80, 24, thumb, ascii, mst, mitems)
	add("missing_nerd", 80, 24, thumb, nerd, mst, mitems)
	// fuzzy highlights
	hst, hitems := pinGoldenState(28, 1, 0)
	hst.Query, hst.QueryCursor = "ee", 2
	hst.Ranked = []int{14, 12, 6}
	hitems[14].Path = "/srv/meadow/green-14"
	hst.MatchRanges[14] = []search.Range{{Start: 6, End: 8}, {Start: 14, End: 16}}
	hst.MatchRanges[12] = []search.Range{{Start: 5, End: 7}}
	hst.MatchRanges[6] = []search.Range{{Start: 5, End: 8}}
	add("filter_ee", 100, 40, thumb, ascii, hst, hitems)
	add("filter_ee_nerd", 100, 40, thumb, nerd, hst, hitems)
	long := hst
	long.Query = strings.Repeat("meadow ", 20)
	long.QueryCursor, long.QueryScroll = len([]rune(long.Query)), 60
	add("filter_long_scrolled", 100, 40, thumb, ascii, long, hitems)
	// long paths that need fitting, highlights inside the shortened text
	lst, litems := pinGoldenState(5, 2, 0)
	for i := range litems {
		litems[i].Path = "/home/" + strings.Join(pinGoldenWords[i:i+9], "/") + "/final-" + pinGoldenWords[i] + ".txt"
	}
	lst.MatchRanges[2] = []search.Range{{Start: 6, End: 10}, {Start: 60, End: 64}}
	lst.MatchRanges[3] = []search.Range{{Start: 1, End: 5}}
	lst.Query = "x"
	add("long_paths_80", 80, 24, thumb, ascii, lst, litems)
	add("long_paths_nerd", 80, 24, thumb, nerd, lst, litems)
	litems[1].PathMissing = true
	add("long_paths_missing_50", 50, 24, thumb, ascii, lst, litems)
	add("long_paths_narrow40", 40, 24, thumb, ascii, lst, litems)
	// narrow and short
	st40, items40 := pinGoldenState(20, 9, 2)
	add("n20_sel9_narrow50", 50, 24, thumb, ascii, st40, items40)
	add("n20_sel9_narrow36", 40, 24, thumb, ascii, st40, items40)
	add("n20_sel9_too_narrow", 39, 24, thumb, ascii, st40, items40)
	st4 := st40
	st4.ListScroll = 6 // a 4-row viewport that holds the cursor
	add("n20_sel9_h16", 80, 16, thumb, ascii, st4, items40)
	add("n20_sel9_h14", 80, 14, thumb, ascii, st4, items40)
	add("n20_sel9_h10", 80, 10, thumb, ascii, st40, items40)
	add("n3_sel1_h12", 80, 12, thumb, ascii, func() PinDialogState { s, _ := pinGoldenState(3, 1, 0); return s }(), pinGoldenItems(3))
	runSimpleGoldens(t, "pin", cases)
}
