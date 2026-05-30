package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

func TestDrawPathPickerDialogSmoke(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	layout := geom.CalculateLayout(80, 24, true, geom.PanelWidthSplit{})
	state := PathPickerState{
		Open:  true,
		Title: "Bookmarks",
		Query: "pro",
		Items: []PathPickerItem{{
			Source: "fzf-marks",
			Name:   "proj",
			Path:   "/tmp/x",
		}},
		Ranked: []int{0},
		MatchRanges: [][]search.Range{
			{{Start: 0, End: 1}},
		},
		Selected:   0,
		ListScroll: 0,
		Focus:      0,
	}
	DrawPathPickerDialog(screen, layout, state, styles)
	cell, _, _ := screen.Get(4, layout.Menu.Height+4)
	if cell == "" || cell == " " {
		t.Fatal("expected filter row content")
	}
}

func TestDrawPathPickerDialogInvalidPathRowStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	layout := geom.CalculateLayout(80, 24, true, geom.PanelWidthSplit{})
	state := PathPickerState{
		Open:  true,
		Title: "Bookmarks",
		Items: []PathPickerItem{{
			Source:      "fzf-marks",
			Name:        "gone",
			Path:        "/no/such/path",
			PathMissing: true,
		}},
		Ranked:     []int{0},
		Selected:   1, // no cursor on row
		ListScroll: 0,
		Focus:      1, // OK focused, list row not active
	}
	DrawPathPickerDialog(screen, layout, state, styles)

	listY := -1
	nameCol := -1
	for y := layout.Menu.Height; y < 24; y++ {
		row := ""
		for x := 0; x < 80; x++ {
			cell, _, _ := screen.Get(x, y)
			row += cell
		}
		if idx := strings.Index(row, "gone"); idx >= 0 {
			listY = y
			nameCol = idx
			break
		}
	}
	if listY < 0 {
		t.Fatal("bookmark row not found on screen")
	}
	_, rowStyle, _ := screen.Get(nameCol, listY)
	if rowStyle != styles.DialogOptionInvalidStyle() {
		t.Fatalf("invalid path row style = %v, want dialog.option.invalid %v", rowStyle, styles.DialogOptionInvalidStyle())
	}
}

func TestEnsurePathPickerListScroll(t *testing.T) {
	st := PathPickerState{
		Ranked:     make([]int, 20),
		Selected:   15,
		ListScroll: 0,
	}
	for i := range st.Ranked {
		st.Ranked[i] = i
	}
	EnsurePathPickerListScroll(&st, 5)
	if st.ListScroll != 11 {
		t.Fatalf("ListScroll = %d want 11", st.ListScroll)
	}
}
