package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the dedup "Keep Directory" confirmation: every focus position, list lengths
// below, at and above the visible cap (with the "(+N more)" line), one versus several groups, a
// root directory, long paths and narrow / short screens.

func dedupKeepGoldenState(n int, long bool, focus string) DedupKeepConfirmState {
	st := DedupKeepConfirmState{Open: true, Dir: "orchard/meadow", Groups: n}
	for i := 0; i < n; i++ {
		a := dedupGoldenWords[i%len(dedupGoldenWords)]
		b := dedupGoldenWords[(i*7+3)%len(dedupGoldenWords)]
		p := a + "/" + b + ".txt"
		if long {
			p = strings.Repeat(a+"-"+b+"/", 5) + b + ".txt"
		}
		st.Kept = append(st.Kept, p)
	}
	if long {
		st.Dir = strings.Repeat("saffron-river/", 6) + "pebble"
	}
	st.Focus = map[string]dlg.ID{"this": DedupKeepThis, "previous": DedupKeepPrevious, "cancel": dlg.Cancel}[focus]
	return st
}

func TestDedupKeepConfirmGoldenRender(t *testing.T) {
	goldenBandRows["dedup_keep_confirm"] = 26
	var cases []simpleGolden
	add := func(name string, w, h int, st DedupKeepConfirmState) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			DrawDedupKeepConfirmDialog(s, l, st, DialogRenderContext{Styles: asciiDialogStyles()})
		}})
	}
	add("n1_previous", 80, 24, dedupKeepGoldenState(1, false, "previous"))
	add("n1_this", 80, 24, dedupKeepGoldenState(1, false, "this"))
	add("n3_previous", 80, 24, dedupKeepGoldenState(3, false, "previous"))
	add("n3_cancel", 80, 24, dedupKeepGoldenState(3, false, "cancel"))
	add("n8_exact", 80, 30, dedupKeepGoldenState(8, false, "previous"))
	add("n9_more", 80, 30, dedupKeepGoldenState(9, false, "previous"))
	add("n40_more", 80, 30, dedupKeepGoldenState(40, false, "this"))
	root := dedupKeepGoldenState(2, false, "previous")
	root.Dir = ""
	add("root", 80, 24, root)
	add("n4_long", 100, 30, dedupKeepGoldenState(4, true, "previous"))
	add("n3_narrow60", 60, 24, dedupKeepGoldenState(3, false, "previous"))
	add("n3_narrow60_long", 60, 24, dedupKeepGoldenState(3, true, "previous"))
	add("n3_short14", 80, 14, dedupKeepGoldenState(3, false, "previous"))
	runSimpleGoldens(t, "dedup_keep_confirm", cases)
}

func TestDedupKeepConfirmIsAltOnly(t *testing.T) {
	if !DedupKeepConfirmView(DedupKeepConfirmState{Kept: []string{"harbor/amber.txt"}}).AltOnly {
		t.Error("AltOnly not set")
	}
}
