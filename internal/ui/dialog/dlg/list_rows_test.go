package dlg

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// drawRows draws item as the driven list of the SFTP-shaped spec (the filter holds focus, so the
// cursor row is lit) and returns the screen, the list's placement and the styles used.
func drawRows(t *testing.T, item Item) (tcell.SimulationScreen, PlacedItem, theme.Theme) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)
	styles := theme.Default()
	styles.UseNerdfontIcons = false
	item.Driven = true
	spec := listSpec(item)
	Draw(screen, 80, 24, spec, "filter", styles)
	screen.Show()
	it, ok := mustPlace(t, spec).Item("hosts")
	if !ok {
		t.Fatal("list not placed")
	}
	return screen, it, styles
}

func rowAt(screen tcell.SimulationScreen, it PlacedItem, i int) string {
	return tcelltest.TextAt(screen, it.Rect.X, it.Rect.Y+i, it.Rect.Width)
}

func TestListRowIcon(t *testing.T) {
	item := listItem(3, 0, 0, 4)
	item.Fit = FitPath
	item.Rows[0].Icon = "D"
	item.Rows[0].Text = "/srv/orchard/willow"
	item.Rows[0].Match = []search.Range{{Start: 5, End: 12}}
	item.Rows[1].Icon = "F"
	screen, it, styles := drawRows(t, item)
	if got := rowAt(screen, it, 0); !strings.HasPrefix(got, "D /srv/orchard/willow") {
		t.Errorf("icon row = %q", got)
	}
	if got := rowAt(screen, it, 2); !strings.HasPrefix(got, "cable 2") {
		t.Errorf("row without icon = %q", got)
	}
	// Match ranges stay keyed to Text: "orchard" starts after the icon and its space.
	_, plain, _ := screen.Get(it.Rect.X+2, it.Rect.Y)
	_, hit, _ := screen.Get(it.Rect.X+2+5, it.Rect.Y)
	if plain == hit {
		t.Errorf("match did not change the style of the highlighted cell: %v", hit)
	}
	// By default the icon is part of the row style; IconOptionFG keeps it on the option foreground.
	if _, st, _ := screen.Get(it.Rect.X, it.Rect.Y+1); st != styles.DialogText.Background(bgOf(styles)) {
		t.Errorf("icon style = %v, want the plain row style", st)
	}
	item.Rows[1].Missing = true
	screen, it, styles = drawRows(t, item)
	if _, st, _ := screen.Get(it.Rect.X, it.Rect.Y+1); st != styles.DialogOptionInvalidStyle() {
		t.Errorf("missing row icon style = %v, want the invalid style", st)
	}
	item.IconOptionFG = true
	screen, it, styles = drawRows(t, item)
	fg, _, _ := styles.DialogOptionRowStyle(false, false).Decompose()
	if _, st, _ := screen.Get(it.Rect.X, it.Rect.Y+1); st != styles.DialogOptionInvalidStyle().Foreground(fg) {
		t.Errorf("IconOptionFG icon style = %v, want the option foreground on the row background", st)
	}
}

func bgOf(styles theme.Theme) tcell.Color {
	_, bg, _ := styles.DialogSurface.Decompose()
	return bg
}

func TestListRowMissingVariantAndMarks(t *testing.T) {
	item := listItem(4, 2, 0, 4)
	item.Rows[0].Missing = true
	item.Rows[1].Marks = RowMarks{Pinned: true}
	item.Rows[2].Missing = true // under the cursor: cursor style wins
	screen, it, styles := drawRows(t, item)
	if _, st, _ := screen.Get(it.Rect.X, it.Rect.Y); st != styles.DialogOptionInvalidStyle() {
		t.Errorf("missing row style = %v, want the invalid style", st)
	}
	if _, st, _ := screen.Get(it.Rect.X, it.Rect.Y+2); st != styles.DialogOptionRowStyle(true, false) {
		t.Errorf("missing row under the cursor style = %v, want the cursor style", st)
	}
	// Marks sit at the row's right edge and the text is fitted to what is left: Missing adds its
	// own mark, a pinned row gets the pin glyph, a clean row none.
	end := it.Rect.Width
	if got := rowAt(screen, it, 0); !strings.HasSuffix(got, " "+styles.IconDialogMissing()) {
		t.Errorf("missing row = %q, want the missing mark at the right edge", got)
	}
	if got := rowAt(screen, it, 1); !strings.HasSuffix(got, " "+string(styles.IconPinRune())) {
		t.Errorf("pinned row = %q, want the pin mark at the right edge", got)
	}
	if got := rowAt(screen, it, 3); strings.TrimRight(got, " ") != "dock 3" {
		t.Errorf("plain row = %q", got)
	}
	if len([]rune(rowAt(screen, it, 0))) != end {
		t.Errorf("row width = %d, want %d", len([]rune(rowAt(screen, it, 0))), end)
	}
}

func TestListRowMarksPlacement(t *testing.T) {
	long := strings.Repeat("quartz", 12)
	item := listItem(2, 0, 0, 3)
	item.Fit = FitRight
	item.Rows[0].Text = long
	item.Rows[0].Marks = RowMarks{Pinned: true, HasJob: true}
	item.Rows[1].Marks = RowMarks{Pinned: true}
	// At the edge the long text is cut to leave room for both marks (4 cells).
	screen, it, _ := drawRows(t, item)
	row := []rune(rowAt(screen, it, 0))
	if got := string(row[len(row)-4:]); strings.TrimSpace(got) == "" {
		t.Errorf("marks missing at the row end: %q", string(row))
	}
	if string(row[len(row)-5]) != "…" {
		t.Errorf("text not fitted before the marks: %q", string(row))
	}
	// A uniform reserved column keeps rows without marks at the same text width.
	item.MarksWidth = 6
	screen, it, _ = drawRows(t, item)
	row = []rune(rowAt(screen, it, 0))
	if string(row[len(row)-7]) != "…" {
		t.Errorf("MarksWidth did not reserve the column: %q", string(row))
	}
	// After the text the marks follow it instead of sitting at the edge.
	item.MarksWidth = 0
	item.MarksAfterText = true
	item.Rows[1].Text = "dock"
	screen, it, styles := drawRows(t, item)
	if got := rowAt(screen, it, 1); !strings.HasPrefix(got, "dock "+string(styles.IconPinRune())) {
		t.Errorf("marks after text = %q", got)
	}
}

func TestListFitNoneKeepsPrefittedTextAndDisplayRanges(t *testing.T) {
	item := listItem(2, 0, 0, 3)
	item.Fit = FitNone
	item.Rows[0].Text = "fzf-marks  amber  /h/b/c…final"
	item.Rows[0].Match = []search.Range{{Start: 11, End: 16}}
	screen, it, _ := drawRows(t, item)
	if got := strings.TrimRight(rowAt(screen, it, 0), " "); got != item.Rows[0].Text {
		t.Errorf("row = %q, want the text as given", got)
	}
	_, plain, _ := screen.Get(it.Rect.X, it.Rect.Y)
	_, hit, _ := screen.Get(it.Rect.X+11, it.Rect.Y)
	_, beyond, _ := screen.Get(it.Rect.X+16, it.Rect.Y)
	if hit == plain || beyond != plain {
		t.Errorf("highlight cells: plain=%v hit=%v after=%v, want the display range 11..16 highlighted", plain, hit, beyond)
	}
}

func TestListUnusedRowsKeepMarksColumn(t *testing.T) {
	item := listItem(1, 0, 0, 3)
	item.MarksWidth = 4
	screen, it, styles := drawRows(t, item)
	_, surface, _ := styles.DialogSurface.Decompose()
	_, st, _ := screen.Get(it.Rect.X+it.Rect.Width-1, it.Rect.Y+2)
	if _, bg, _ := st.Decompose(); bg != surface {
		t.Errorf("unused row marks cell background = %v, want the surface %v", bg, surface)
	}
	if st != tcell.StyleDefault.Background(surface) {
		t.Errorf("unused row marks cell style = %v, want a blank-filled marks cell", st)
	}
}
