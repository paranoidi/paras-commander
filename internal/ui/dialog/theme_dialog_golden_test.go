package dialog

import (
	"fmt"
	"strings"

	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
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

// themeGoldenSamples fill each app sample's rows with a line naming it and the row, so a
// golden shows where the samples sit and where the pane cuts them off.
func themeGoldenSamples() ThemeAppSamples {
	fill := func(name string, rows int) ThemeSamplePainter {
		return func(s tcell.Screen, x, y, width, maxY int, styles theme.Theme) {
			for i := range rows {
				if y+i < maxY {
					primitive.Text(s, x, y+i, width, fmt.Sprintf("%s %d", name, i+1), styles.DialogText)
				}
			}
		}
	}
	return ThemeAppSamples{
		MenuBar:  fill("menu", ThemeAppMenuRows),
		Jobs:     fill("job", ThemeAppJobRows),
		Progress: fill("progress", ThemeAppProgressRows),
		Panel:    fill("panel", ThemePanelRows),
	}
}

func themeGoldenState(n, selected int, focus string) ThemeDialogState {
	st := ThemeDialogState{Open: true, Selected: selected, Choices: themeGoldenChoices(n), CurrentName: "meadow-theme"}
	for i := range n {
		st.Ranked = append(st.Ranked, i)
	}
	switch focus {
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	default:
		st.Focus = ThemeFilter
	}
	return st
}

func TestThemeDialogGoldenRender(t *testing.T) {
	goldenBandRows["theme_dialog"] = 40
	var cases []simpleGolden
	addPage := func(page, w, h, n, sel int) {
		st := themeGoldenState(n, sel, "filter")
		st.Page = page - 1
		name := fmt.Sprintf("%dx%d_n%d_sel%d_focus-filter_page%d", w, h, n, sel, page)
		cases = append(cases, simpleGolden{name, w, h, func(s tcell.Screen, l Layout) {
			EnsureThemeListScroll(&st, ThemeDialogListRows(st, l, asciiDialogStyles()))
			DrawThemeDialog(s, l, st, asciiDialogStyles(), themeGoldenSamples())
		}})
	}
	add := func(w, h, n, sel int, focus string) {
		st := themeGoldenState(n, sel, focus)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_n%d_sel%d_focus-%s", w, h, n, sel, focus), w, h, func(s tcell.Screen, l Layout) {
			EnsureThemeListScroll(&st, ThemeDialogListRows(st, l, asciiDialogStyles()))
			DrawThemeDialog(s, l, st, asciiDialogStyles(), themeGoldenSamples())
		}})
	}
	for _, focus := range []string{"filter", "ok"} {
		add(100, 40, 5, 2, focus)
	}
	// shorter than the viewport (rows = max(n,17)+1)
	add(100, 40, 1, 0, "filter")
	add(100, 40, 17, 0, "filter")
	add(100, 40, 17, 8, "filter")
	add(100, 40, 17, 16, "filter")
	add(100, 40, 25, 24, "filter")
	// exactly filling / overflowing a short screen (viewport 14 rows)
	add(100, 20, 14, 0, "filter")
	add(100, 20, 14, 13, "filter")
	add(100, 20, 30, 0, "filter")
	add(100, 20, 30, 6, "filter")
	add(100, 20, 30, 7, "filter")
	add(100, 20, 30, 15, "filter")
	add(100, 20, 30, 22, "filter")
	add(100, 20, 30, 23, "filter")
	add(100, 20, 30, 29, "filter")
	add(100, 20, 30, 15, "ok")
	add(100, 20, 30, 15, "cancel")
	// default terminal
	add(80, 24, 30, 0, "filter")
	add(80, 24, 30, 12, "filter")
	add(80, 24, 30, 29, "filter")
	// narrow
	add(60, 24, 30, 12, "filter")
	add(40, 24, 5, 3, "filter")
	// page 2: the whole pane, cut off on a short screen, narrow
	addPage(2, 120, 60, 17, 8)
	addPage(2, 100, 40, 17, 8)
	addPage(2, 100, 20, 30, 15)
	addPage(2, 80, 24, 30, 12)
	addPage(2, 60, 24, 30, 12)
	addPage(2, 40, 24, 5, 3)
	// page 3: the file panel sample, whole and cut off
	addPage(3, 100, 40, 17, 8)
	addPage(3, 100, 20, 30, 15)
	addPage(3, 80, 24, 30, 12)
	// too short to draw
	add(100, 9, 5, 1, "filter")
	// empty list draws nothing
	add(100, 40, 0, 0, "filter")
	runSimpleGoldens(t, "theme_dialog", cases)
}
