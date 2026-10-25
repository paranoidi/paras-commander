package dialog

import (
	"fmt"
	"strings"

	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the theme picker: every focus position, a list shorter than the viewport,
// exactly filling it and overflowing it (screen-height clamped), cursor at the top, middle and
// bottom, scrolled offsets, an empty list and a narrow screen.

var themeGoldenWords = []string{
	"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor", "ivory", "jasper",
	"kestrel", "lantern", "meadow", "nectar", "orchard", "pebble", "quartz", "river", "saffron",
	"thistle", "umber", "velvet", "willow", "yarrow", "zephyr", "anchor", "breeze", "copper",
	"drift", "fern",
}

func themeGoldenChoices(n int) []ThemeChoice {
	out := make([]ThemeChoice, n)
	for i := range out {
		w := themeGoldenWords[i%len(themeGoldenWords)]
		out[i] = ThemeChoice{Name: w + "-theme", Label: strings.ToUpper(w[:1]) + w[1:] + " Theme"}
	}
	return out
}

func themeGoldenState(n, selected int, focus string) ThemeDialogState {
	st := ThemeDialogState{Open: true, Selected: selected, Choices: themeGoldenChoices(n), CurrentName: "meadow-theme"}
	switch focus {
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	default:
		st.Focus = ThemeList
	}
	return st
}

func TestThemeDialogGoldenRender(t *testing.T) {
	goldenBandRows["theme_dialog"] = 40
	var cases []simpleGolden
	add := func(w, h, n, sel int, focus string) {
		st := themeGoldenState(n, sel, focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_n%d_sel%d_focus-%s", w, h, n, sel, focus), w, h, func(s tcell.Screen, l Layout) {
			DrawThemeDialog(s, l, st, asciiDialogStyles())
		}})
	}
	for _, focus := range []string{"list", "ok"} {
		add(100, 40, 5, 2, focus)
	}
	// shorter than the viewport (rows = max(n,17)+1)
	add(100, 40, 1, 0, "list")
	add(100, 40, 17, 0, "list")
	add(100, 40, 17, 8, "list")
	add(100, 40, 17, 16, "list")
	add(100, 40, 25, 24, "list")
	// exactly filling / overflowing a short screen (viewport 14 rows)
	add(100, 20, 14, 0, "list")
	add(100, 20, 14, 13, "list")
	add(100, 20, 30, 0, "list")
	add(100, 20, 30, 6, "list")
	add(100, 20, 30, 7, "list")
	add(100, 20, 30, 15, "list")
	add(100, 20, 30, 22, "list")
	add(100, 20, 30, 23, "list")
	add(100, 20, 30, 29, "list")
	add(100, 20, 30, 15, "ok")
	add(100, 20, 30, 15, "cancel")
	// default terminal
	add(80, 24, 30, 0, "list")
	add(80, 24, 30, 12, "list")
	add(80, 24, 30, 29, "list")
	// narrow
	add(60, 24, 30, 12, "list")
	add(40, 24, 5, 3, "list")
	// too short to draw
	add(100, 9, 5, 1, "list")
	// empty list draws nothing
	add(100, 40, 0, 0, "list")
	runSimpleGoldens(t, "theme_dialog", cases)
}
