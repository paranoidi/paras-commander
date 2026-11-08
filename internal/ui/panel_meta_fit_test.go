package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// wideMetaColumns returns n meta columns, each 12 cells wide, titled meadow0, meadow1, ...
func wideMetaColumns(n int) []MetaColumnState {
	cols := make([]MetaColumnState, n)
	for i := range cols {
		title := "meadow" + string(rune('0'+i))
		cols[i] = MetaColumnState{EntryName: title, ColumnTitle: title, Results: map[string]string{"/harbor/lantern": "abcdefghijkl"}}
	}
	return cols
}

func metaTitles(layouts []MetaColumnLayout) string {
	titles := make([]string, len(layouts))
	for i, lay := range layouts {
		titles[i] = lay.Title
	}
	return strings.Join(titles, " ")
}

func TestFitPanelMetaColumns(t *testing.T) {
	cols := wideMetaColumns(4)
	tests := []struct {
		name   string
		width  int
		format panel.ColumnSet
		want   string
	}{
		// Each column takes 12 + 2 gap. Brief reserves 6 after the name, mtime 24.
		{"all fit", 100, panel.ColumnsBrief, "meadow0 meadow1 meadow2 meadow3"},
		// 62 left: four columns would leave 6, three leave exactly 20.
		{"rightmost dropped first", 70, panel.ColumnsBrief, "meadow0 meadow1 meadow2"},
		// 29 left: one column would leave 15.
		{"name keeps the row", 55, panel.ColumnsModified, ""},
		{"name-only shows none", 30, panel.ColumnsBrief, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := panel.State{Columns: tt.format}
			layoutRes := panelColumnLayout(Rect{Width: tt.width}, st, PanelDisplayConfig{NarrowPanelsNameOnly: true})
			kept, totalW := fitPanelMetaColumns(layoutRes, st, cols)
			if got := metaTitles(kept); got != tt.want {
				t.Fatalf("kept = %q, want %q", got, tt.want)
			}
			if len(kept) == 0 {
				if totalW != 0 {
					t.Fatalf("totalW = %d with no columns", totalW)
				}
				return
			}
			if w := panelNameColumnWidth(layoutRes, tt.format, true, totalW); w < config.MinPanelNameWidthWithMeta {
				t.Fatalf("name width = %d, below the floor", w)
			}
		})
	}
}

func TestPanelColumnLayoutDropsFileColumnsThenGoesNameOnly(t *testing.T) {
	st := panel.State{Columns: panel.ColumnsModified}
	display := PanelDisplayConfig{NarrowPanelsNameOnly: false}
	// Size and Modified reserve 25 cells: 37 row text leaves 12, 33 leaves 8. Size alone reserves
	// 7: 15 row text leaves 8, and the 15 cells of name alone pass the floor; 9 do not.
	if got := panelColumnLayout(Rect{Width: 39}, st, display); got.NameOnlyDisplay || got.Columns != panel.ColumnsModified {
		t.Fatalf("a legible name should keep the columns, got name-only=%v %v", got.NameOnlyDisplay, got.Columns.Names())
	}
	if got := panelColumnLayout(Rect{Width: 35}, st, display); got.NameOnlyDisplay || got.Columns != panel.ColumnsBrief {
		t.Fatalf("a name below the floor should drop the rightmost column first, got name-only=%v %v", got.NameOnlyDisplay, got.Columns.Names())
	}
	if got := panelColumnLayout(Rect{Width: 17}, st, display); got.NameOnlyDisplay || got.Columns != 0 {
		t.Fatalf("with no room for size the name keeps the row, got name-only=%v %v", got.NameOnlyDisplay, got.Columns.Names())
	}
	if got := panelColumnLayout(Rect{Width: 11}, st, display); !got.NameOnlyDisplay || got.Columns != 0 {
		t.Fatalf("a name below the floor with no column left should go name-only, got name-only=%v %v", got.NameOnlyDisplay, got.Columns.Names())
	}
}

func TestPanelNameOnlyWidth(t *testing.T) {
	st := panel.State{}
	if got := PanelNameOnlyWidth(100, st, false); got != 98 {
		t.Fatalf("no icons = %d, want 98", got)
	}
	if got, want := PanelNameOnlyWidth(100, st, true), 98-panelIconListLeadingGutter-panelIconStripCells; got != want {
		t.Fatalf("icons = %d, want %d", got, want)
	}
	tree := panel.State{ListLayout: panel.ListLayoutTree}
	if got, want := PanelNameOnlyWidth(100, tree, true), 98-panelIconListLeadingGutter; got != want {
		t.Fatalf("tree = %d, want %d", got, want)
	}
}

func TestPanelNameColumnWidthCountsEveryMetaColumn(t *testing.T) {
	st := panel.State{Columns: panel.ColumnsBrief}
	// The zoom gate must see the crushed name, not the fitted one.
	if got := PanelNameColumnWidth(70, st, false, true, wideMetaColumns(4)); got != 6 {
		t.Fatalf("got %d, want 6", got)
	}
}

func TestPanelHeaderColumnAtSkipsDroppedMetaColumn(t *testing.T) {
	file := Rect{X: 0, Y: 0, Width: 70, Height: 20}
	model := &Model{
		Primary:              panel.State{Columns: panel.ColumnsBrief},
		MetaResults:          [2][]MetaColumnState{PrimaryPanel: wideMetaColumns(4)},
		NarrowPanelsNameOnly: true,
	}
	seen := map[string]bool{}
	for x := 1; x < file.Width-1; x++ {
		if c, onTitle, _ := PanelHeaderColumnAt(file, model, PrimaryPanel, x, file.Y+1); onTitle && c.MetaColumn != "" {
			seen[c.MetaColumn] = true
		}
	}
	if !seen["meadow2"] || seen["meadow3"] {
		t.Fatalf("header meta hits = %v, want meadow0 to meadow2", seen)
	}
}

func TestDrawPanelKeepsNameReadableWithManyMetaColumns(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(60, 10)

	const name = "lantern-harbor"
	state := panel.State{
		Path:    pathloc.MustParse("/harbor"),
		Entries: []localfs.Entry{{Name: name, Path: "/harbor/" + name, Type: localfs.EntryFile}},
	}
	drawPanel(screen, Rect{Width: 60, Height: 10}, state, PanelStyleConfig{Styles: theme.Default()}, PanelContext{},
		PanelDisplayConfig{MetaColumns: wideMetaColumns(8), NarrowPanelsNameOnly: true})

	var row strings.Builder
	for x := 1; x < 59; x++ {
		ch, _, _ := screen.Get(x, 2)
		row.WriteString(ch)
	}
	if !strings.Contains(row.String(), name) {
		t.Fatalf("row %q does not show %q", row.String(), name)
	}
}

func TestFitCarouselMetaColumns(t *testing.T) {
	longName := localfs.Entry{Name: "lantern-harbor-meadow-pebble", Path: "/harbor/lantern", Type: localfs.EntryFile}
	shortName := localfs.Entry{Name: "fig", Path: "/harbor/lantern", Type: localfs.EntryFile}
	fitLayout := panelcarousel.DefaultLayout()
	fitLayout.Splits[1] = panelcarousel.ColumnSplitSpec{Kind: panelcarousel.SplitFitChars, Value: 60}
	frame := Rect{Width: 120, Height: 20}

	tests := []struct {
		name     string
		layout   panelcarousel.Layout
		entry    localfs.Entry
		cols     int
		wantKept int // -1: fewer than cols
		wantName int
	}{
		{"flex drops", panelcarousel.DefaultLayout(), longName, 8, -1, config.MinPanelNameWidthWithMeta},
		{"fit drops at the cap", fitLayout, longName, 8, -1, config.MinPanelNameWidthWithMeta},
		// A column hugging short names never reaches the floor and must not lose its meta for it.
		{"fit keeps meta beside short names", fitLayout, shortName, 2, 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := panel.State{Entries: []localfs.Entry{tt.entry}}
			cols := wideMetaColumns(tt.cols)
			fitWidths := panelcarousel.MeasureFitColumnWidths(tt.layout, panelcarousel.Column{}, state, false, true, uiscrollbar.StyleThumb, 10)
			kept, totalW := FitCarouselMetaColumns(frame, tt.layout, state, false, true, uiscrollbar.StyleThumb, 10, fitWidths, cols)
			if tt.wantKept >= 0 && len(kept) != tt.wantKept {
				t.Fatalf("kept %d columns, want %d", len(kept), tt.wantKept)
			}
			if tt.wantKept < 0 && len(kept) == len(cols) {
				t.Fatal("eight wide columns should not fit")
			}
			measured := panelcarousel.WithCenterMeta(fitWidths, totalW)
			if w := panelcarousel.CenterNameWidth(frame, tt.layout, state, false, true, uiscrollbar.StyleThumb, 10, measured, totalW); w < tt.wantName {
				t.Fatalf("center name width = %d, below %d", w, tt.wantName)
			}
			for i, lay := range kept {
				if lay.Title != cols[i].ColumnTitle {
					t.Fatalf("kept[%d] = %q: columns must drop from the right", i, lay.Title)
				}
			}
		})
	}
}
