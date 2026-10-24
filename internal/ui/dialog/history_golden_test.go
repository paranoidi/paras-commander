package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Render goldens for the History dialog: every focus position with and without the zoxide
// checkbox, lists shorter than the viewport, exactly filling it and overflowing it, cursor at the
// top, middle and bottom, history and zoxide source icons, missing paths, pinned / job marks,
// fuzzy highlights, long paths that need fitting and narrow sizes.

var historyGoldenWords = []string{
	"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor", "ivory", "jasper",
	"kestrel", "lantern", "meadow", "nectar", "orchard", "pebble", "quartz", "river", "saffron",
	"thistle", "umber", "velvet", "willow", "yarrow", "zephyr", "anchor", "breeze", "copper",
}

func historyGoldenPaths(n int) []string {
	out := make([]string, n)
	for i := range out {
		a, b := historyGoldenWords[i%len(historyGoldenWords)], historyGoldenWords[(i*5+3)%len(historyGoldenWords)]
		out[i] = fmt.Sprintf("/home/%s/%s-%d", a, b, i)
	}
	return out
}

// historyGoldenMarks pins every path ending in 3 or 7, gives 5 and 7 a running job.
func historyGoldenMarks(path string) RowMarks {
	var m RowMarks
	switch {
	case strings.HasSuffix(path, "3"):
		m.Pinned = true
	case strings.HasSuffix(path, "5"):
		m.HasJob, m.JobStatus, m.JobWrite = true, "running", true
	case strings.HasSuffix(path, "7"):
		m.Pinned, m.HasJob, m.JobStatus = true, true, "queued"
	}
	return m
}

// historyGoldenState builds a state with n history rows followed by z zoxide rows. focus names
// the focus position: filter, zoxide, ok or cancel.
func historyGoldenState(n, z, selected, scroll int, zoxide bool, focus string) HistoryDialogState {
	st := HistoryDialogState{
		Open: true, Paths: historyGoldenPaths(n + z), Selected: selected, ListScroll: scroll,
		ZoxideAvailable: zoxide, IncludeZoxide: zoxide, ZoxideFrom: n,
	}
	st.DisplayLines = st.Paths
	st.PathMissing = make([]bool, len(st.Paths))
	st.MatchRanges = make([][]search.Range, len(st.Paths))
	for i := range st.Paths {
		st.Ranked = append(st.Ranked, i)
	}
	switch focus {
	case "zoxide":
		st.Focus = HistoryZoxide
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	}
	return st
}

func TestHistoryGoldenRender(t *testing.T) {
	goldenBandRows["history"] = 30
	var cases []simpleGolden
	add := func(name string, w, h int, st HistoryDialogState, styles theme.Theme) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawHistoryDialog(s, l, st, styles, uiscrollbar.StyleThumb, historyGoldenMarks)
		}})
	}
	ascii := asciiDialogStyles()
	nerd := theme.Default()
	nerd.UseNerdfontIcons = true
	// every focus position, with and without the checkbox
	for _, focus := range []string{"filter", "zoxide", "ok", "cancel"} {
		add("zox_n6_z3_sel2_focus-"+focus, 80, 24, historyGoldenState(6, 3, 2, 0, true, focus), ascii)
	}
	for _, focus := range []string{"filter", "ok", "cancel"} {
		add("nozox_n6_sel2_focus-"+focus, 80, 24, historyGoldenState(6, 0, 2, 0, false, focus), ascii)
	}
	add("zox_n6_z3_sel2_focus-ok_nerd", 80, 24, historyGoldenState(6, 3, 2, 0, true, "ok"), nerd)
	add("zox_n6_z3_sel2_nerd", 80, 24, historyGoldenState(6, 3, 2, 0, true, "filter"), nerd)
	both := historyGoldenState(6, 3, 2, 0, true, "filter")
	both.BothPanels = true
	add("both_panels", 80, 24, both, ascii)
	// viewport 12 rows at 80x24
	for _, c := range []struct {
		name         string
		n, sel, scrl int
	}{
		{"n0", 0, 0, 0}, {"n1_sel0", 1, 0, 0}, {"n11_sel10", 11, 10, 0}, {"n12_sel0", 12, 0, 0}, {"n12_sel11", 12, 11, 0},
		{"n13_sel12_scroll1", 13, 12, 1}, {"n30_sel0", 30, 0, 0}, {"n30_sel11", 30, 11, 0}, {"n30_sel15_scroll4", 30, 15, 4},
		{"n30_sel29_scroll18", 30, 29, 18},
	} {
		add("zox_"+c.name, 80, 24, historyGoldenState(c.n, 0, c.sel, c.scrl, true, "filter"), ascii)
		add("nozox_"+c.name, 80, 24, historyGoldenState(c.n, 0, c.sel, c.scrl, false, "filter"), ascii)
	}
	// the cursor row is not lit when focus is elsewhere
	add("zox_n30_sel15_scroll4_focus-ok", 80, 24, historyGoldenState(30, 0, 15, 4, true, "ok"), ascii)
	// source icons around the history / zoxide boundary
	add("zox_n8_z8_sel7_icons", 80, 24, historyGoldenState(8, 8, 7, 0, true, "filter"), ascii)
	add("zox_n8_z8_sel8_icons_nerd", 80, 24, historyGoldenState(8, 8, 8, 0, true, "filter"), nerd)
	// 100x40: viewport 18 rows
	add("zox_n18_sel17", 100, 40, historyGoldenState(18, 0, 17, 0, true, "filter"), ascii)
	add("zox_n40_sel20_scroll3", 100, 40, historyGoldenState(40, 0, 20, 3, true, "filter"), ascii)
	add("nozox_n40_sel39_scroll22", 100, 40, historyGoldenState(40, 0, 39, 22, false, "filter"), ascii)
	// missing paths: under the cursor and elsewhere, with and without marks
	mst := historyGoldenState(12, 4, 3, 0, true, "filter")
	mst.PathMissing[3], mst.PathMissing[5], mst.PathMissing[0], mst.PathMissing[13] = true, true, true, true
	add("missing_cursor_on_missing", 80, 24, mst, ascii)
	mst.Selected = 4
	add("missing_cursor_elsewhere", 80, 24, mst, ascii)
	add("missing_nerd", 80, 24, mst, nerd)
	mst.Focus = dlg.OK
	add("missing_focus-ok", 80, 24, mst, ascii)
	// fuzzy highlights
	hst := historyGoldenState(28, 0, 1, 0, true, "filter")
	hst.Query, hst.QueryCursor = "ee", 2
	hst.Ranked = []int{14, 12, 6}
	hst.Paths[14] = "/home/meadow/green-14"
	hst.DisplayLines = hst.Paths
	hst.MatchRanges[14] = []search.Range{{Start: 7, End: 9}, {Start: 15, End: 17}}
	hst.MatchRanges[12] = []search.Range{{Start: 6, End: 8}}
	hst.MatchRanges[6] = []search.Range{{Start: 6, End: 9}}
	add("filter_ee", 100, 40, hst, ascii)
	add("filter_ee_nerd", 100, 40, hst, nerd)
	long := hst
	long.Query = strings.Repeat("meadow ", 20)
	long.QueryCursor, long.QueryScroll = len([]rune(long.Query)), 60
	add("filter_long_scrolled", 100, 40, long, ascii)
	// long paths that need fitting, with highlights inside the shortened text and marks
	lst := historyGoldenState(8, 2, 2, 0, true, "filter")
	for i := range lst.Paths {
		lst.Paths[i] = "/home/" + strings.Join(historyGoldenWords[i:i+9], "/") + "/final-" + historyGoldenWords[i] + fmt.Sprint(i)
	}
	lst.DisplayLines = lst.Paths
	lst.MatchRanges[2] = []search.Range{{Start: 6, End: 10}, {Start: 60, End: 64}}
	lst.MatchRanges[3] = []search.Range{{Start: 1, End: 5}}
	lst.PathMissing[1] = true
	add("long_paths_80", 80, 24, lst, ascii)
	add("long_paths_nerd", 80, 24, lst, nerd)
	add("long_paths_50", 50, 24, lst, ascii)
	add("long_paths_narrow40", 40, 24, lst, ascii)
	// marks ending in the last visible cells
	mk := historyGoldenState(10, 0, 3, 0, true, "filter")
	add("marks_cursor_on_pinned", 80, 24, mk, ascii)
	mk.Selected = 5
	add("marks_cursor_on_job", 80, 24, mk, ascii)
	mk.Selected = 7
	add("marks_cursor_on_both", 80, 24, mk, nerd)
	// narrow and short
	st20 := historyGoldenState(20, 0, 9, 2, true, "filter")
	add("n20_sel9_narrow50", 50, 24, st20, ascii)
	add("n20_sel9_narrow40", 40, 24, st20, ascii)
	add("n20_sel9_narrow36", 40, 24, historyGoldenState(20, 0, 9, 2, true, "ok"), ascii)
	add("n20_sel9_too_narrow", 39, 24, st20, ascii)
	st4 := historyGoldenState(20, 0, 9, 6, true, "filter")
	add("n20_sel9_h16", 80, 16, st4, ascii)
	add("n20_sel9_h14", 80, 14, st4, ascii)
	st4n := historyGoldenState(20, 0, 9, 6, false, "filter")
	add("nozox_n20_sel9_h14", 80, 14, st4n, ascii)
	add("n20_sel9_h12", 80, 12, st4, ascii)
	add("nozox_n20_sel9_h12", 80, 12, st4n, ascii)
	add("n20_sel9_h10", 80, 10, st4, ascii)
	runSimpleGoldens(t, "history", cases)
}
