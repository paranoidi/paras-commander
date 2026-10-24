package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the dedup "remove empty directories" confirmation: every focus position,
// list lengths below, at and above the visible cap (with the "(+N more)" line), an empty list,
// long paths, with and without row icons and narrow / short screens. The icon painter is a
// stand-in that marks the cell where the real devicon strip is drawn.

var dedupGoldenWords = []string{
	"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor", "ivory", "jasper",
	"kestrel", "lantern", "meadow", "nectar", "orchard", "pebble", "quartz", "river", "saffron",
}

func dedupGoldenDirs(n int, long bool) []string {
	out := make([]string, n)
	for i := range out {
		a := dedupGoldenWords[i%len(dedupGoldenWords)]
		b := dedupGoldenWords[(i*7+3)%len(dedupGoldenWords)]
		out[i] = a + "/" + b
		if long {
			out[i] = strings.Repeat(a+"-"+b+"/", 5) + b
		}
	}
	return out
}

func dedupGoldenState(n int, long bool, focus string) DedupEmptyDirsConfirmState {
	st := DedupEmptyDirsConfirmState{Open: true, Dirs: dedupGoldenDirs(n, long)}
	switch focus {
	case "no":
		st.Focus = DedupEmptyDirsNo
	case "cancel":
		st.Focus = dlg.Cancel
	default:
		st.Focus = DedupEmptyDirsYes
	}
	return st
}

func dedupGoldenIconPainter(screen tcell.Screen, x, y int, _ string, styles theme.Theme) {
	screen.SetContent(x, y, '#', nil, styles.DialogText)
}

func TestDedupEmptyDirsConfirmGoldenRender(t *testing.T) {
	goldenBandRows["dedup_empty_dirs"] = 30
	var cases []simpleGolden
	add := func(name string, w, h int, st DedupEmptyDirsConfirmState, icons bool) {
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s", w, h, name), w, h, func(s tcell.Screen, l Layout) {
			ctx := DialogRenderContext{Styles: asciiDialogStyles()}
			var painter DedupEmptyDirsRowIconPainter
			if icons {
				ctx.ShowIcons, ctx.IconLead = true, 2
				painter = dedupGoldenIconPainter
			}
			DrawDedupEmptyDirsConfirmDialog(s, l, st, ctx, painter)
		}})
	}
	for _, focus := range []string{"yes", "no", "cancel"} {
		add("n3_focus-"+focus, 80, 24, dedupGoldenState(3, false, focus), false)
	}
	add("n3_icons", 80, 24, dedupGoldenState(3, false, "yes"), true)
	add("n0", 80, 24, dedupGoldenState(0, false, "yes"), false)
	add("n1", 80, 24, dedupGoldenState(1, false, "no"), false)
	add("n1_icons", 80, 24, dedupGoldenState(1, false, "no"), true)
	add("n9", 80, 30, dedupGoldenState(9, false, "yes"), false)
	add("n10_exact", 80, 30, dedupGoldenState(10, false, "yes"), false)
	add("n10_exact_icons", 80, 30, dedupGoldenState(10, false, "cancel"), true)
	add("n11_more", 80, 30, dedupGoldenState(11, false, "yes"), false)
	add("n11_more_icons", 80, 30, dedupGoldenState(11, false, "no"), true)
	add("n40_more", 80, 30, dedupGoldenState(40, false, "yes"), false)
	add("n4_long", 100, 30, dedupGoldenState(4, true, "yes"), false)
	add("n4_long_icons", 100, 30, dedupGoldenState(4, true, "yes"), true)
	add("n12_long_icons", 100, 30, dedupGoldenState(12, true, "no"), true)
	add("n3_narrow60", 60, 24, dedupGoldenState(3, true, "yes"), true)
	add("n3_narrow40", 40, 24, dedupGoldenState(3, false, "yes"), false)
	add("n3_narrow40_long", 40, 24, dedupGoldenState(3, true, "no"), true)
	add("n3_short14", 80, 14, dedupGoldenState(3, false, "yes"), false)
	add("n12_short12", 80, 12, dedupGoldenState(12, false, "yes"), true)
	runSimpleGoldens(t, "dedup_empty_dirs", cases)
}
