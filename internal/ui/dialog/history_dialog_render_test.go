package dialog

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func TestDrawHistoryDialogSmoke(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	layout := geom.CalculateLayout(80, 24, true, geom.PanelWidthSplit{})
	state := HistoryDialogState{
		Open:         true,
		PanelID:      0,
		Paths:        []string{"/tmp/a", "/tmp/b"},
		DisplayLines: []string{"/tmp/a", "/tmp/b"},
		Query:        "tmp",
		Ranked:       []int{0, 1},
		MatchRanges: [][]search.Range{
			{{Start: 2, End: 5}},
			{{Start: 2, End: 5}},
		},
		Selected:   0,
		ListScroll: 0,
	}
	DrawHistoryDialog(screen, layout, state, styles, uiscrollbar.StyleNone, nil)
	cell, _, _ := screen.Get(4, layout.Menu.Height+4)
	if cell == "" || cell == " " {
		t.Fatal("expected filter row content")
	}
}

func TestDrawHistoryDialogShowsPinIconOnlyForPinnedRow(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	layout := geom.CalculateLayout(80, 24, true, geom.PanelWidthSplit{})
	state := HistoryDialogState{
		Open:         true,
		Paths:        []string{"/tmp/pinned-alpha", "/tmp/other-bravo"},
		DisplayLines: []string{"/tmp/pinned-alpha", "/tmp/other-bravo"},
		Ranked:       []int{0, 1},
		MatchRanges:  [][]search.Range{nil, nil},
		Selected:     -1,
	}
	rowMarks := func(absPath string) RowMarks {
		return RowMarks{Pinned: absPath == "/tmp/pinned-alpha"}
	}
	DrawHistoryDialog(screen, layout, state, styles, uiscrollbar.StyleNone, rowMarks)

	pinIcon := []rune(styles.IconPin())[0]
	rowContainingHasIcon := func(needle string) bool {
		for y := 0; y < 24; y++ {
			row := ""
			hasIcon := false
			for x := 0; x < 80; x++ {
				str, _, _ := screen.Get(x, y)
				row += str
				r, _ := utf8.DecodeRuneInString(str)
				if r == pinIcon {
					hasIcon = true
				}
			}
			if strings.Contains(row, needle) {
				return hasIcon
			}
		}
		t.Fatalf("row containing %q not found on screen", needle)
		return false
	}
	if !rowContainingHasIcon("pinned-alpha") {
		t.Error("expected pin icon on pinned row")
	}
	if rowContainingHasIcon("other-bravo") {
		t.Error("did not expect pin icon on unpinned row")
	}
}

func TestEnsureHistoryListScroll(t *testing.T) {
	st := HistoryDialogState{
		Ranked:     make([]int, 20),
		Selected:   15,
		ListScroll: 0,
	}
	for i := range st.Ranked {
		st.Ranked[i] = i
	}
	EnsureHistoryListScroll(&st, 5)
	if st.ListScroll != 11 {
		t.Fatalf("ListScroll = %d want 11", st.ListScroll)
	}
}

func TestDrawHistoryDialogCursorRowSourceIconVisible(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	styles.UseNerdfontIcons = true
	layout := geom.CalculateLayout(80, 24, true, geom.PanelWidthSplit{})
	state := HistoryDialogState{
		Open:         true,
		Paths:        []string{"/tmp/a", "/tmp/b"},
		DisplayLines: []string{"/tmp/a", "/tmp/b"},
		Ranked:       []int{0, 1},
		ZoxideFrom:   2,
	}
	DrawHistoryDialog(screen, layout, state, styles, uiscrollbar.StyleNone, nil)
	w, h := screen.Size()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell, st, _ := screen.Get(x, y)
			if cell != styles.IconDialogHistory() {
				continue
			}
			fg, bg, _ := st.Decompose()
			if fg == bg {
				t.Fatalf("cursor-row icon at (%d,%d) fg == bg %v", x, y, fg)
			}
			if want, _, _ := styles.DialogOptionRowStyle(true, false).Decompose(); fg != want {
				t.Fatalf("cursor-row icon fg = %v, want %v", fg, want)
			}
			return
		}
	}
	t.Fatal("history source icon not drawn")
}

func TestDrawHistoryDialogScrollbarOnOverflow(t *testing.T) {
	paths := make([]string, 40)
	ranked := make([]int, len(paths))
	for i := range paths {
		paths[i] = fmt.Sprintf("/tmp/orchard%02d", i)
		ranked[i] = i
	}
	state := HistoryDialogState{Open: true, Paths: paths, DisplayLines: paths, Ranked: ranked, ZoxideFrom: len(paths)}
	render := func(sb uiscrollbar.Style) string {
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatalf("Init: %v", err)
		}
		defer screen.Fini()
		screen.SetSize(80, 24)
		DrawHistoryDialog(screen, geom.CalculateLayout(80, 24, true, geom.PanelWidthSplit{}), state, theme.Default(), sb, nil)
		var b strings.Builder
		for y := 0; y < 24; y++ {
			cell, _, _ := screen.Get(79-2, y) // right border column of the 78-wide (clamped to 76) dialog
			b.WriteString(cell)
		}
		return b.String()
	}
	if render(uiscrollbar.StyleNone) == render(uiscrollbar.StyleBar) {
		t.Fatal("bar scrollbar did not change the right border on an overflowing list")
	}
}
