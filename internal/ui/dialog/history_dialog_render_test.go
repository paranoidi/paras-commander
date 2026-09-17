package dialog

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
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
		Open:              true,
		PanelID:           0,
		Paths:             []string{"/tmp/a", "/tmp/b"},
		PanelCurrentIndex: 0,
		DisplayLines:      []string{"/tmp/a", "/tmp/b"},
		Query:             "tmp",
		Ranked:            []int{0, 1},
		MatchRanges: [][]search.Range{
			{{Start: 2, End: 5}},
			{{Start: 2, End: 5}},
		},
		Selected:   0,
		ListScroll: 0,
		Focus:      0,
	}
	DrawHistoryDialog(screen, layout, state, styles, nil)
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
		Focus:        0,
	}
	rowMarks := func(absPath string) RowMarks {
		return RowMarks{Pinned: absPath == "/tmp/pinned-alpha"}
	}
	DrawHistoryDialog(screen, layout, state, styles, rowMarks)

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
