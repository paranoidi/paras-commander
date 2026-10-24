package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the SFTP connect dialog: every focus position, a host list shorter than the
// viewport, exactly filling it and overflowing it, cursor at the top, middle and bottom, scrolled
// offsets, an empty list, a filter value with match highlights, a typed location and narrow sizes.

var sftpGoldenWords = []string{
	"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor", "ivory", "jasper",
	"kestrel", "lantern", "meadow", "nectar", "orchard", "pebble", "quartz", "river", "saffron",
	"thistle", "umber", "velvet", "willow", "yarrow", "zephyr", "anchor", "breeze", "copper",
}

func sftpGoldenLines(n int) []string {
	out := make([]string, n)
	for i := range out {
		w := sftpGoldenWords[i%len(sftpGoldenWords)]
		out[i] = fmt.Sprintf("%-10s %s@%s.example.net:22", w, "user"+w[:1], w)
	}
	return out
}

func sftpGoldenState(n, selected, scroll int, focus string) SFTPConnectDialogState {
	st := SFTPConnectDialogState{
		Open: true, DisplayLines: sftpGoldenLines(n), Selected: selected, ListScroll: scroll,
		Location: FileDialogField{Label: "Location", Value: "sftp://", Prefill: "sftp://", Cursor: 7, PrefillPending: true},
	}
	for i := 0; i < n; i++ {
		st.Ranked = append(st.Ranked, i)
	}
	st.MatchRanges = make([][]search.Range, n)
	switch focus {
	case "location":
		st.Focus = SFTPLocation
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	default:
		st.Focus = SFTPFilter
	}
	return st
}

func TestSFTPConnectGoldenRender(t *testing.T) {
	goldenBandRows["sftp_connect"] = 40
	var cases []simpleGolden
	add := func(name string, w, h int, st SFTPConnectDialogState) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawSFTPConnectDialog(s, l, st, asciiDialogStyles())
		}})
	}
	for _, focus := range []string{"filter", "location", "ok", "cancel"} {
		add("n5_sel2_focus-"+focus, 100, 40, sftpGoldenState(5, 2, 0, focus))
	}
	// 100x40: viewport 12 rows
	add("n0_focus-filter", 100, 40, sftpGoldenState(0, 0, 0, "filter"))
	add("n0_focus-location", 100, 40, sftpGoldenState(0, 0, 0, "location"))
	add("n1_sel0", 100, 40, sftpGoldenState(1, 0, 0, "filter"))
	add("n11_sel10", 100, 40, sftpGoldenState(11, 10, 0, "filter"))
	add("n12_sel0", 100, 40, sftpGoldenState(12, 0, 0, "filter"))
	add("n12_sel11", 100, 40, sftpGoldenState(12, 11, 0, "filter"))
	add("n20_sel0", 100, 40, sftpGoldenState(20, 0, 0, "filter"))
	add("n20_sel11", 100, 40, sftpGoldenState(20, 11, 0, "filter"))
	add("n20_sel12_scroll1", 100, 40, sftpGoldenState(20, 12, 1, "filter"))
	add("n20_sel15_scroll4", 100, 40, sftpGoldenState(20, 15, 4, "filter"))
	add("n20_sel19_scroll8", 100, 40, sftpGoldenState(20, 19, 8, "filter"))
	add("n20_sel15_scroll4_location", 100, 40, sftpGoldenState(20, 15, 4, "location"))
	add("n20_sel15_scroll4_ok", 100, 40, sftpGoldenState(20, 15, 4, "ok"))
	// 80x24: viewport 8 rows
	add("n20_sel0", 80, 24, sftpGoldenState(20, 0, 0, "filter"))
	add("n20_sel7", 80, 24, sftpGoldenState(20, 7, 0, "filter"))
	add("n20_sel8_scroll1", 80, 24, sftpGoldenState(20, 8, 1, "filter"))
	add("n20_sel19_scroll12", 80, 24, sftpGoldenState(20, 19, 12, "filter"))
	add("n8_sel7", 80, 24, sftpGoldenState(8, 7, 0, "filter"))
	// filter value with highlights
	filtered := sftpGoldenState(28, 1, 0, "filter")
	filtered.Query, filtered.QueryCursor = "ee", 2
	filtered.Ranked = []int{14, 12, 6}
	filtered.MatchRanges[14] = []search.Range{{Start: 1, End: 3}}
	filtered.MatchRanges[12] = []search.Range{{Start: 0, End: 2}}
	filtered.MatchRanges[6] = []search.Range{{Start: 3, End: 5}}
	add("filter_ee", 100, 40, filtered)
	longQuery := filtered
	longQuery.Query = strings.Repeat("meadow ", 20)
	longQuery.QueryCursor, longQuery.QueryScroll = len([]rune(longQuery.Query)), 60
	add("filter_long_scrolled", 100, 40, longQuery)
	typed := sftpGoldenState(5, 1, 0, "location")
	typed.Location = FileDialogField{Label: "Location", Value: "sftp://orchard.example.net/srv", Prefill: "sftp://", Cursor: 30}
	add("location_typed", 100, 40, typed)
	// narrow and short
	add("n20_sel9_narrow50", 50, 24, sftpGoldenState(20, 9, 2, "filter"))
	add("n20_sel9_narrow44", 44, 24, sftpGoldenState(20, 9, 2, "filter"))
	add("n20_sel9_too_narrow", 43, 24, sftpGoldenState(20, 9, 2, "filter"))
	add("n20_sel9_h14", 80, 14, sftpGoldenState(20, 9, 7, "filter"))
	add("n20_sel9_h12", 80, 12, sftpGoldenState(20, 9, 7, "filter"))
	add("n20_sel9_h10", 80, 10, sftpGoldenState(20, 9, 7, "filter"))
	add("n3_sel1_h10", 80, 10, sftpGoldenState(3, 1, 0, "filter"))
	runSimpleGoldens(t, "sftp_connect", cases)
}
