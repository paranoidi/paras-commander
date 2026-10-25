package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// Render goldens for the path picker: bookmark-style rows (source, name, path columns) and
// history-style rows (path only), the navigate purpose (Cancel only) and the apply purposes
// (OK / Cancel), every focus position, lists shorter than the viewport, exactly filling it and
// overflowing it, cursor at the top, middle and bottom, missing paths, pinned / job marks,
// fuzzy highlights across the columns, long paths that need fitting, an invalid query, the
// completion dropdown and narrow sizes.

var pickerGoldenWords = []string{
	"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor", "ivory", "jasper",
	"kestrel", "lantern", "meadow", "nectar", "orchard", "pebble", "quartz", "river", "saffron",
	"thistle", "umber", "velvet", "willow", "yarrow", "zephyr", "anchor", "breeze", "copper",
}

// pickerGoldenItems builds n rows; bookmarks gives each a source and a name.
func pickerGoldenItems(n int, bookmarks bool) []PathPickerItem {
	out := make([]PathPickerItem, n)
	for i := range out {
		a, b := pickerGoldenWords[i%len(pickerGoldenWords)], pickerGoldenWords[(i*5+3)%len(pickerGoldenWords)]
		out[i] = PathPickerItem{Path: fmt.Sprintf("/home/%s/%s-%d", a, b, i)}
		if bookmarks {
			out[i].Source = []string{"fzf-marks", "gnome", "pinned"}[i%3]
			out[i].Name = a + "-" + b
		}
	}
	return out
}

// pickerGoldenMarks pins paths ending in 3 or 7, gives 5 and 7 a running job.
func pickerGoldenMarks(path string) RowMarks {
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

// pickerGoldenState builds an apply-purpose picker. focus is filter, ok or cancel.
func pickerGoldenState(n, selected, scroll int, bookmarks bool, focus string) PathPickerState {
	st := PathPickerState{
		Open: true, Title: "Bookmarks", Purpose: PathPickerPurposeApplyTransferDestination,
		Items: pickerGoldenItems(n, bookmarks), Selected: selected, ListScroll: scroll,
	}
	st.MatchRanges = make([][]search.Range, n)
	for i := 0; i < n; i++ {
		st.Ranked = append(st.Ranked, i)
	}
	switch focus {
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	}
	return st
}

func TestPathPickerGoldenRender(t *testing.T) {
	goldenBandRows["path_picker"] = 30
	var cases []simpleGolden
	add := func(name string, w, h int, st PathPickerState, styles theme.Theme, marks RowMarksFunc) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawPathPickerDialog(s, l, st, styles, uiscrollbar.StyleThumb, marks)
		}})
	}
	ascii := asciiDialogStyles()
	nerd := theme.Default()
	nerd.UseNerdfontIcons = true
	// every focus position, apply purpose and navigate purpose, both row shapes
	for _, focus := range []string{"filter", "ok"} {
		add("apply_bm_n6_sel2_focus-"+focus, 80, 24, pickerGoldenState(6, 2, 0, true, focus), ascii, nil)
		add("apply_hist_n6_sel2_focus-"+focus, 80, 24, pickerGoldenState(6, 2, 0, false, focus), ascii, nil)
	}
	for _, focus := range []string{"filter"} {
		nav := pickerGoldenState(6, 2, 0, true, focus)
		nav.Purpose = PathPickerPurposeNavigate
		add("navigate_bm_n6_sel2_focus-"+focus, 80, 24, nav, ascii, nil)
		nav = pickerGoldenState(6, 2, 0, false, focus)
		nav.Purpose = PathPickerPurposeNavigate
		add("navigate_hist_n6_sel2_focus-"+focus, 80, 24, nav, ascii, nil)
	}
	notitle := pickerGoldenState(4, 1, 0, false, "filter")
	notitle.Title = ""
	add("default_title", 80, 24, notitle, ascii, nil)
	// viewport 12 rows at 80x24
	for _, c := range []struct {
		name         string
		n, sel, scrl int
	}{
		{"n0", 0, 0, 0}, {"n1_sel0", 1, 0, 0}, {"n11_sel10", 11, 10, 0}, {"n12_sel0", 12, 0, 0}, {"n12_sel11", 12, 11, 0},
		{"n13_sel12_scroll1", 13, 12, 1}, {"n30_sel0", 30, 0, 0}, {"n30_sel11", 30, 11, 0}, {"n30_sel15_scroll4", 30, 15, 4},
		{"n30_sel29_scroll18", 30, 29, 18},
	} {
		add("bm_"+c.name, 80, 24, pickerGoldenState(c.n, c.sel, c.scrl, true, "filter"), ascii, nil)
		add("hist_"+c.name, 80, 24, pickerGoldenState(c.n, c.sel, c.scrl, false, "filter"), ascii, nil)
	}
	add("bm_n30_sel15_scroll4_focus-ok", 80, 24, pickerGoldenState(30, 15, 4, true, "ok"), ascii, nil)
	add("bm_n30_sel15_scroll4_nerd", 80, 24, pickerGoldenState(30, 15, 4, true, "filter"), nerd, nil)
	// 100x40: viewport 18 rows
	add("bm_n18_sel17", 100, 40, pickerGoldenState(18, 17, 0, true, "filter"), ascii, nil)
	add("bm_n40_sel20_scroll3", 100, 40, pickerGoldenState(40, 20, 3, true, "filter"), ascii, nil)
	add("hist_n40_sel39_scroll22", 100, 40, pickerGoldenState(40, 39, 22, false, "filter"), ascii, nil)
	// marks, with and without missing paths, in both row shapes
	mk := pickerGoldenState(10, 3, 0, true, "filter")
	add("marks_bm_cursor_on_pinned", 80, 24, mk, ascii, pickerGoldenMarks)
	mk.Selected = 5
	add("marks_bm_cursor_on_job", 80, 24, mk, ascii, pickerGoldenMarks)
	mk.Selected = 7
	add("marks_bm_cursor_on_both_nerd", 80, 24, mk, nerd, pickerGoldenMarks)
	mh := pickerGoldenState(10, 5, 0, false, "filter")
	add("marks_hist_cursor_on_job", 80, 24, mh, ascii, pickerGoldenMarks)
	mh.Items[4].PathMissing, mh.Items[7].PathMissing = true, true
	add("marks_hist_missing", 80, 24, mh, ascii, pickerGoldenMarks)
	// missing paths: under the cursor and elsewhere
	mst := pickerGoldenState(12, 3, 0, true, "filter")
	mst.Items[3].PathMissing, mst.Items[5].PathMissing, mst.Items[0].PathMissing = true, true, true
	add("missing_bm_cursor_on_missing", 80, 24, mst, ascii, nil)
	mst.Selected = 4
	add("missing_bm_cursor_elsewhere", 80, 24, mst, ascii, nil)
	add("missing_bm_nerd", 80, 24, mst, nerd, nil)
	mst.Focus = dlg.OK
	add("missing_bm_focus-ok", 80, 24, mst, ascii, nil)
	// fuzzy highlights across name and path columns (ranges are offsets into "name path")
	hst := pickerGoldenState(28, 1, 0, true, "filter")
	hst.Query, hst.QueryCursor = "ee", 2
	hst.Ranked = []int{14, 12, 6}
	hst.Items[14].Name = "meadow-green"
	hst.Items[14].Path = "/home/meadow/green-14"
	hst.MatchRanges[14] = []search.Range{{Start: 8, End: 10}, {Start: 20, End: 22}}
	hst.MatchRanges[12] = []search.Range{{Start: 1, End: 3}}
	hst.MatchRanges[6] = []search.Range{{Start: len(hst.Items[6].Name) + 7, End: len(hst.Items[6].Name) + 10}}
	add("filter_ee_bm", 100, 40, hst, ascii, nil)
	add("filter_ee_bm_nerd", 100, 40, hst, nerd, nil)
	hh := pickerGoldenState(28, 1, 0, false, "filter")
	hh.Query, hh.QueryCursor = "ee", 2
	hh.Ranked = []int{14, 12, 6}
	hh.MatchRanges[14] = []search.Range{{Start: 7, End: 9}, {Start: 15, End: 17}}
	hh.MatchRanges[12] = []search.Range{{Start: 6, End: 8}}
	add("filter_ee_hist", 100, 40, hh, ascii, nil)
	long := hh
	long.Query = strings.Repeat("meadow ", 20)
	long.QueryCursor, long.QueryScroll = len([]rune(long.Query)), 60
	add("filter_long_scrolled", 100, 40, long, ascii, nil)
	inv := pickerGoldenState(6, 1, 0, false, "filter")
	inv.Query, inv.QueryCursor, inv.QueryPathInvalid = "/home/nowhere", 13, true
	add("query_path_invalid", 80, 24, inv, ascii, nil)
	inv.QueryPathCheckPending = true
	add("query_path_check_pending", 80, 24, inv, ascii, nil)
	// completion dropdown over the list
	comp := pickerGoldenState(12, 1, 0, true, "filter")
	comp.Query, comp.QueryCursor = "/home/a", 7
	comp.Completion.Set(comp.Query, 6, []pathpick.Candidate{
		{Name: "amber", IsDir: true, Ranges: []search.Range{{Start: 0, End: 1}}},
		{Name: "anchor", IsDir: true, Ranges: []search.Range{{Start: 0, End: 1}}},
		{Name: "apple.txt", Ranges: []search.Range{{Start: 0, End: 1}}},
	})
	add("completion_dropdown", 80, 24, comp, ascii, nil)
	comp.Completion.Set(comp.Query, 6, []pathpick.Candidate{{Name: "amber-lantern", IsDir: true, Ranges: []search.Range{{Start: 0, End: 1}}}})
	add("completion_ghost", 80, 24, comp, ascii, nil)
	// long paths and names: the columns shrink and the paths are fitted
	lst := pickerGoldenState(8, 2, 0, true, "filter")
	for i := range lst.Items {
		lst.Items[i].Name = "bookmark-" + strings.Join(pickerGoldenWords[i:i+3], "-")
		lst.Items[i].Path = "/home/" + strings.Join(pickerGoldenWords[i:i+9], "/") + "/final-" + pickerGoldenWords[i]
	}
	lst.MatchRanges[2] = []search.Range{{Start: 3, End: 7}, {Start: len(lst.Items[2].Name) + 40, End: len(lst.Items[2].Name) + 44}}
	lst.MatchRanges[3] = []search.Range{{Start: 12, End: 16}}
	lst.Items[1].PathMissing = true
	add("long_bm_80", 80, 24, lst, ascii, nil)
	add("long_bm_marks", 80, 24, lst, ascii, pickerGoldenMarks)
	add("long_bm_nerd", 80, 24, lst, nerd, nil)
	add("long_bm_50", 50, 24, lst, ascii, nil)
	add("long_bm_narrow40", 40, 24, lst, ascii, nil)
	lh := pickerGoldenState(8, 2, 0, false, "filter")
	for i := range lh.Items {
		lh.Items[i].Path = "/home/" + strings.Join(pickerGoldenWords[i:i+9], "/") + "/final-" + pickerGoldenWords[i]
	}
	lh.MatchRanges[2] = []search.Range{{Start: 6, End: 10}, {Start: 60, End: 64}}
	add("long_hist_80", 80, 24, lh, ascii, nil)
	add("long_hist_narrow40", 40, 24, lh, ascii, nil)
	// narrow and short
	st20 := pickerGoldenState(20, 9, 2, true, "filter")
	add("bm_n20_sel9_narrow50", 50, 24, st20, ascii, nil)
	add("bm_n20_sel9_narrow40", 40, 24, st20, ascii, nil)
	add("bm_n20_sel9_narrow36", 40, 24, pickerGoldenState(20, 9, 2, true, "ok"), ascii, nil)
	add("bm_n20_sel9_too_narrow", 39, 24, st20, ascii, nil)
	st4 := pickerGoldenState(20, 9, 6, true, "filter")
	add("bm_n20_sel9_h20", 80, 20, pickerGoldenState(20, 9, 6, true, "filter"), ascii, nil)
	add("bm_n20_sel9_h16", 80, 16, st4, ascii, nil)
	add("bm_n20_sel9_h13", 80, 13, st4, ascii, nil)
	add("bm_n20_sel9_h12", 80, 12, st4, ascii, nil)
	add("bm_n20_sel9_h10", 80, 10, st4, ascii, nil)
	runSimpleGoldens(t, "path_picker", cases)
}
