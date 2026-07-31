package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

func TestLeaderMenuVisibleItemsThreeColumnsGrowRows(t *testing.T) {
	layout := geom.Layout{Width: 80, Height: 24, Menu: geom.Rect{Height: 1}, Footer: geom.Rect{Y: 23, Height: 1}}
	items := make([]LeaderMenuItem, 4)
	if got := len(LeaderMenuVisibleItems(layout, items)); got != 4 {
		t.Fatalf("visible = %d, want 4", got)
	}
	items = make([]LeaderMenuItem, 7)
	if got := len(LeaderMenuVisibleItems(layout, items)); got != 7 {
		t.Fatalf("visible = %d, want 7", got)
	}
}

func TestLeaderMenuContentRowsWithGroupHeaders(t *testing.T) {
	items := []LeaderMenuItem{
		{GroupTitle: "File", GroupColumn: 0},
		{Key: 'c', Label: "Copy", GroupColumn: 0},
		{Key: 'M', Label: "Move", GroupColumn: 0},
		{Key: 'm', Label: "Mkdir", GroupColumn: 0},
		{GroupTitle: "App", GroupColumn: 2},
		{Key: '?', Label: "Help", GroupColumn: 2},
	}
	if got := leaderMenuContentRows(items); got != 4 {
		t.Fatalf("rows = %d, want 4 (max of col0=4 and col2=2)", got)
	}
}

func TestLeaderMenuSplitByColumn(t *testing.T) {
	items := []LeaderMenuItem{
		{GroupTitle: "File", GroupColumn: 0},
		{Key: 'c', Label: "Copy", GroupColumn: 0},
		{GroupTitle: "Selection", GroupColumn: 1},
		{Key: 'g', Label: "Select group", GroupColumn: 1},
		{GroupTitle: "Navigation", GroupColumn: 2},
		{Key: 'h', Label: "History", GroupColumn: 2},
	}
	buckets := leaderMenuSplitByColumn(items)
	if len(buckets[0]) != 2 || len(buckets[1]) != 2 || len(buckets[2]) != 2 {
		t.Fatalf("buckets = %d/%d/%d, want 2/2/2", len(buckets[0]), len(buckets[1]), len(buckets[2]))
	}
}

func TestLeaderMenuIndexForKeyCaseSensitiveWhenConfigured(t *testing.T) {
	items := []LeaderMenuItem{
		{Key: 'f', Label: "Find files"},
		{Key: 'F', Label: "Find duplicates"},
	}
	if i, ok := LeaderMenuIndexForKey(items, 'f'); !ok || i != 0 {
		t.Fatalf("f = %d %v, want 0 true", i, ok)
	}
	if i, ok := LeaderMenuIndexForKey(items, 'F'); !ok || i != 1 {
		t.Fatalf("F = %d %v, want 1 true", i, ok)
	}
	if _, ok := LeaderMenuIndexForKey(items, 'd'); ok {
		t.Fatal("d should not match")
	}
}

func TestLeaderMenuIndexForKeySkipsGroupHeaders(t *testing.T) {
	items := []LeaderMenuItem{
		{GroupTitle: "Tools", GroupColumn: 1},
		{Key: 'f', Label: "Find files", GroupColumn: 1},
		{Key: 'F', Label: "Find duplicates", GroupColumn: 1},
	}
	if i, ok := LeaderMenuIndexForKey(items, 'F'); !ok || i != 1 {
		t.Fatalf("F = %d %v, want action index 1", i, ok)
	}
}

func TestLeaderMenuIndexForKeyCaseInsensitiveForAutoKeys(t *testing.T) {
	items := []LeaderMenuItem{{Label: "git status"}, {Label: "Print working directory"}}
	if i, ok := LeaderMenuIndexForKey(items, 'G'); !ok || i != 0 {
		t.Fatalf("G = %d %v, want 0 true", i, ok)
	}
	if i, ok := LeaderMenuIndexForKey(items, 'p'); !ok || i != 1 {
		t.Fatalf("p = %d %v, want 1 true", i, ok)
	}
}

func leaderMenuMacroX(macroCol int) int {
	contentWidth := 80 - leaderMenuLeftMargin
	macroW := contentWidth / leaderMenuMacroColumns
	return leaderMenuLeftMargin + macroCol*macroW
}

func TestDrawLeaderMenuRendersConfiguredCase(t *testing.T) {
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 50)

	layout := geom.Layout{Width: 80, Height: 50, Footer: geom.Rect{Y: 49, Height: 1}}
	state := LeaderMenuState{
		Open: true,
		Items: []LeaderMenuItem{
			{GroupTitle: "Tools", GroupColumn: 1},
			{Key: 'f', Label: "Find files", GroupColumn: 1},
			{Key: 'F', Label: "Find duplicates", GroupColumn: 1},
		},
	}
	DrawLeaderMenu(screen, layout, state, theme.Default())

	wantKeyFG, _, _ := theme.Default().LeaderMenuKey.Decompose()
	x := leaderMenuMacroX(1)
	mainc, style, _ := screen.Get(x, 46)
	fg, _, _ := style.Decompose()
	if mainc != "f" || fg != wantKeyFG {
		t.Fatalf("first key at (%d,46) = %q fg=%v, want f", x, mainc, fg)
	}
}

func TestDrawLeaderMenuRendersGroupTitleCyan(t *testing.T) {
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 50)

	layout := geom.Layout{Width: 80, Height: 50, Footer: geom.Rect{Y: 49, Height: 1}}
	state := LeaderMenuState{
		Open:  true,
		Items: []LeaderMenuItem{{GroupTitle: "File", GroupColumn: 0}, {Key: 'c', Label: "Copy", GroupColumn: 0}},
	}
	DrawLeaderMenu(screen, layout, state, theme.Default())

	wantFG, _, _ := theme.Default().LeaderMenuGroup.Decompose()
	x := leaderMenuMacroX(0)
	_, style, _ := screen.Get(x, 46)
	fg, _, _ := style.Decompose()
	if fg != wantFG {
		t.Fatalf("group fg = %v, want %v", fg, wantFG)
	}
	mainc, _, _ := screen.Get(x, 46)
	if mainc != "F" {
		t.Fatalf("group title start = %q, want F (File)", mainc)
	}
}

func TestDrawLeaderMenuOmitsDirectKeyWhenWidthConstrained(t *testing.T) {
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(40, 50)

	items := []LeaderMenuItem{
		{GroupTitle: "File", GroupColumn: 0},
		{Key: 'c', Label: "Copy", GroupColumn: 0, DirectKey: "F5"},
		{GroupTitle: "Tools", GroupColumn: 1},
		{Key: 'f', Label: "Find files", GroupColumn: 1, DirectKey: "C-f"},
	}
	layout := geom.Layout{Width: 40, Height: 50, Footer: geom.Rect{Y: 49, Height: 1}}
	if leaderMenuShowDirectKeys(layout, items) {
		t.Fatal("expected direct keys suppressed at minimum width")
	}

	state := LeaderMenuState{Open: true, Items: items}
	styles := theme.Default()
	DrawLeaderMenu(screen, layout, state, styles)

	wantArrowFG, _, _ := styles.LeaderMenuArrow.Decompose()
	x := leaderMenuLeftMargin + (40-leaderMenuLeftMargin)/leaderMenuMacroColumns
	y := 48
	suffixCol := x + 4 + len("Find files")
	mainc, style, _ := screen.Get(suffixCol+1, y)
	fg, _, _ := style.Decompose()
	if mainc == "C" && fg == wantArrowFG {
		t.Fatalf("direct key suffix at (%d,%d) should be omitted when width is constrained", suffixCol+1, y)
	}
}

func TestDrawLeaderMenuOmitsDirectKeyWhenEntriesHidden(t *testing.T) {
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 8)

	items := []LeaderMenuItem{
		{GroupTitle: "Tools", GroupColumn: 1},
		{Key: 'f', Label: "Find files", GroupColumn: 1, DirectKey: "C-f"},
		{Key: 'F', Label: "Find duplicates", GroupColumn: 1, DirectKey: "C-M-f"},
		{Key: 'C', Label: "Compare panels", GroupColumn: 1, DirectKey: "C-M-c"},
		{Key: 'R', Label: "Run for each", GroupColumn: 1, DirectKey: "C-M-r"},
		{Key: 'x', Label: "Extra one", GroupColumn: 1, DirectKey: "C-x"},
		{Key: 'y', Label: "Extra two", GroupColumn: 1, DirectKey: "C-y"},
	}
	layout := geom.Layout{Width: 80, Height: 8, Footer: geom.Rect{Y: 7, Height: 1}}
	if LeaderMenuHiddenActionCount(layout, items) == 0 {
		t.Fatal("test setup: expected hidden leader menu actions")
	}

	state := LeaderMenuState{Open: true, Items: items}
	styles := theme.Default()
	DrawLeaderMenu(screen, layout, state, styles)

	wantArrowFG, _, _ := styles.LeaderMenuArrow.Decompose()
	x := leaderMenuMacroX(1)
	y := 2 // group header on row 1, first action on row 2
	suffixCol := x + 4 + len("Find files")
	mainc, style, _ := screen.Get(suffixCol+1, y)
	fg, _, _ := style.Decompose()
	if mainc == "C" && fg == wantArrowFG {
		t.Fatalf("direct key suffix at (%d,%d) should be omitted when entries are hidden", suffixCol+1, y)
	}
}

func TestDrawLeaderMenuRendersDirectKeySuffix(t *testing.T) {
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 50)

	layout := geom.Layout{Width: 80, Height: 50, Footer: geom.Rect{Y: 49, Height: 1}}
	state := LeaderMenuState{
		Open: true,
		Items: []LeaderMenuItem{
			{GroupTitle: "Tools", GroupColumn: 1},
			{Key: 'f', Label: "Find files", GroupColumn: 1, DirectKey: "C-f"},
		},
	}
	styles := theme.Default()
	DrawLeaderMenu(screen, layout, state, styles)

	wantLabelFG, _, _ := styles.LeaderMenuLabel.Decompose()
	wantArrowFG, _, _ := styles.LeaderMenuArrow.Decompose()
	x := leaderMenuMacroX(1)
	y := 47           // group header on row above action
	labelCol := x + 4 // key, space, arrow, space
	mainc, style, _ := screen.Get(labelCol, y)
	fg, _, _ := style.Decompose()
	if mainc != "F" || fg != wantLabelFG {
		t.Fatalf("label start at (%d,%d) = %q fg=%v, want F with label fg", labelCol, y, mainc, fg)
	}
	suffixCol := labelCol + len("Find files")
	mainc, style, _ = screen.Get(suffixCol, y)
	fg, _, _ = style.Decompose()
	if mainc != " " || fg != wantArrowFG {
		t.Fatalf("suffix space at (%d,%d) = %q fg=%v, want space with arrow fg", suffixCol, y, mainc, fg)
	}
	mainc, style, _ = screen.Get(suffixCol+1, y)
	fg, _, _ = style.Decompose()
	if mainc != "C" || fg != wantArrowFG {
		t.Fatalf("suffix key at (%d,%d) = %q fg=%v, want C with arrow fg", suffixCol+1, y, mainc, fg)
	}
}

func TestDrawLeaderMenuRendersQuestionMarkHelp(t *testing.T) {
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 50)

	layout := geom.Layout{Width: 80, Height: 50, Footer: geom.Rect{Y: 49, Height: 1}}
	state := LeaderMenuState{
		Open:  true,
		Items: []LeaderMenuItem{{GroupTitle: "App", GroupColumn: 2}, {Key: '?', Label: "Help", GroupColumn: 2}},
	}
	DrawLeaderMenu(screen, layout, state, theme.Default())

	x := leaderMenuMacroX(2)
	mainc, _, _ := screen.Get(x, 47)
	if mainc != "?" {
		t.Fatalf("help key = %q, want ?", mainc)
	}
}
