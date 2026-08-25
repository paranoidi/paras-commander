package app

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func TestCompareViewFooterEscFirst(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.ViewMode = ui.ViewCompare
	app.model.CompareView.IgnoreEmpty = true
	keys := app.activeFooterKeys()
	if len(keys) == 0 {
		t.Fatal("footer keys empty")
	}
	if keys[0] != menu.FooterEscClose {
		t.Fatalf("footer[0] = %+v, want Esc Close", keys[0])
	}
	var foundMerge, foundEmpty bool
	for _, fk := range keys {
		if fk.Key == tcell.KeyF5 && fk.Hint == "Merge" {
			foundMerge = true
		}
		if fk.Hint == "Show empty" {
			foundEmpty = true
		}
	}
	if !foundMerge {
		t.Fatalf("footer missing F5 Merge: %+v", keys)
	}
	if !foundEmpty {
		t.Fatalf("footer missing Show empty (ignore-empty on): %+v", keys)
	}
}

func TestCompareViewOpenIgnoresEmptyByDefault(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	left := pathloc.MustParse(t.TempDir())
	right := pathloc.MustParse(t.TempDir())
	app.model.Primary.Path = left
	app.model.Secondary.Path = right

	app.openComparePanels()
	if !app.model.CompareView.IgnoreEmpty {
		t.Fatal("open: IgnoreEmpty = false, want true")
	}
}

func TestCompareViewToggleEmpty(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.ViewMode = ui.ViewCompare
	app.model.CompareView = ui.CompareViewState{IgnoreEmpty: true, Filter: comparepkg.FilterAll}
	app.model.CompareSnapshot = comparepkg.Snapshot{
		Rows: []comparepkg.Row{
			{Kind: comparepkg.KindPrimaryOnly, PrimaryRel: "empty.txt", Size: 0, HashDone: true},
			{Kind: comparepkg.KindPrimaryOnly, PrimaryRel: "data.bin", Size: 10, HashDone: true},
		},
	}

	if n := len(app.compareCtrl.FilteredRows()); n != 1 {
		t.Fatalf("filtered with ignore = %d, want 1", n)
	}
	if !app.tryDispatchCompare(keymap.ActionCompareToggleEmpty) {
		t.Fatal("toggle-empty not dispatched")
	}
	if app.model.CompareView.IgnoreEmpty {
		t.Fatal("after toggle: IgnoreEmpty still true")
	}
	if n := len(app.compareCtrl.FilteredRows()); n != 2 {
		t.Fatalf("filtered after show = %d, want 2", n)
	}
	keys := app.activeFooterKeys()
	var foundIgnore bool
	for _, fk := range keys {
		if fk.Hint == "Ignore empty" {
			foundIgnore = true
		}
	}
	if !foundIgnore {
		t.Fatalf("footer missing Ignore empty after toggle: %+v", keys)
	}
}

func TestCompareViewLeftRightColumnFocus(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.ViewMode = ui.ViewCompare
	app.model.CompareView = ui.CompareViewState{FocusColumn: ui.CompareColumnSecondary}
	app.model.CompareSnapshot = comparepkg.Snapshot{
		Rows: []comparepkg.Row{
			{PrimaryRel: "a.txt", SecondaryRel: "b.txt", Kind: comparepkg.KindContentDiff, HashDone: true},
		},
	}

	app.handleCompareViewKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if app.model.CompareView.FocusColumn != ui.CompareColumnPrimary {
		t.Fatalf("after left: FocusColumn = %v, want primary", app.model.CompareView.FocusColumn)
	}

	app.handleCompareViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if app.model.CompareView.FocusColumn != ui.CompareColumnSecondary {
		t.Fatalf("after right: FocusColumn = %v, want secondary", app.model.CompareView.FocusColumn)
	}
}

func TestCompareViewOpenUsesPrimaryColumnFocus(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	left := pathloc.MustParse(t.TempDir())
	right := pathloc.MustParse(t.TempDir())
	app.model.Primary.Path = left
	app.model.Secondary.Path = right

	app.openComparePanels()
	if app.model.ViewMode != ui.ViewCompare {
		t.Fatal("compare view did not open")
	}
	if app.model.CompareView.FocusColumn != ui.CompareColumnPrimary {
		t.Fatalf("open FocusColumn = %v, want primary", app.model.CompareView.FocusColumn)
	}
}

func TestCompareViewKeyRightVisuallyFocusesRightColumn(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.ViewMode = ui.ViewCompare
	app.model.CompareSnapshot = comparepkg.Snapshot{
		PrimaryRoot:   pathloc.MustParse("/left-root"),
		SecondaryRoot: pathloc.MustParse("/right-root"),
		Phase:         comparepkg.PhaseDone,
		Rows: []comparepkg.Row{
			{PrimaryRel: "alpha.txt", SecondaryRel: "beta.txt", Kind: comparepkg.KindContentDiff, HashDone: true},
		},
	}
	app.model.CompareView = ui.CompareViewState{
		Selected:    0,
		Filter:      comparepkg.FilterAll,
		FocusColumn: ui.CompareColumnPrimary,
	}

	app.render()
	leftX, rightX, lineY := compareViewColumnCoords(t, app)
	styles := theme.Default()
	if !compareColumnHasCursorBG(screen, styles, leftX, lineY) {
		t.Fatal("open: left column should have cursor background")
	}
	if compareColumnHasCursorBG(screen, styles, rightX, lineY) {
		t.Fatal("open: right column should not have cursor background")
	}

	_, _ = app.handleKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	app.render()
	if app.model.CompareView.FocusColumn != ui.CompareColumnSecondary {
		t.Fatalf("after right: FocusColumn = %v, want secondary", app.model.CompareView.FocusColumn)
	}
	if compareColumnHasCursorBG(screen, styles, leftX, lineY) {
		t.Fatal("after right: left column should not have cursor background")
	}
	if !compareColumnHasCursorBG(screen, styles, rightX, lineY) {
		t.Fatal("after right: right column should have cursor background")
	}
}

func TestCompareViewKeyDownScrollsBeforeCursorLeavesViewport(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.ViewMode = ui.ViewCompare

	visible := app.compareVisibleRows()
	if visible < 3 {
		t.Fatalf("compareVisibleRows() = %d, want >= 3", visible)
	}
	rows := make([]comparepkg.Row, visible+3)
	for i := range rows {
		name := fmt.Sprintf("entry-%02d.txt", i)
		rows[i] = comparepkg.Row{
			PrimaryRel: name, SecondaryRel: name,
			Kind: comparepkg.KindEqual, HashDone: true,
		}
	}
	app.model.CompareSnapshot = comparepkg.Snapshot{
		PrimaryRoot:   pathloc.MustParse("/left-root"),
		SecondaryRoot: pathloc.MustParse("/right-root"),
		Phase:         comparepkg.PhaseDone,
		Rows:          rows,
	}
	app.model.CompareView = ui.CompareViewState{Selected: 0, Filter: comparepkg.FilterAll}

	for i := 0; i < visible; i++ {
		app.handleCompareViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	st := app.model.CompareView
	if st.Selected != visible {
		t.Fatalf("Selected = %d, want %d", st.Selected, visible)
	}
	if st.ListScroll != 1 {
		t.Fatalf("ListScroll = %d, want 1 (scroll as soon as cursor would leave viewport)", st.ListScroll)
	}
	if st.Selected < st.ListScroll || st.Selected >= st.ListScroll+visible {
		t.Fatalf("Selected %d outside visible window [%d, %d)", st.Selected, st.ListScroll, st.ListScroll+visible)
	}
}

func compareViewColumnCoords(t *testing.T, app *App) (leftX, rightX, lineY int) {
	t.Helper()
	width, height := app.screen.Size()
	layout := app.layoutForTerminalSize(width, height)
	rect := ui.MergeTwinPanelRects(layout.Primary, layout.Secondary, app.model.SplitOrientation)
	contentX := rect.X + 2
	contentW := rect.Width - 4
	pathW := (contentW - 2 - 1) / 2
	statusX := contentX + pathW
	rightX = statusX + 2 + 1
	return contentX, rightX, rect.Y + 2
}

func compareColumnHasCursorBG(screen tcell.SimulationScreen, styles theme.Theme, x, y int) bool {
	_, activeBG, _ := styles.PanelCursorActive.Decompose()
	for dx := 0; dx < 8; dx++ {
		ch, st, _ := screen.Get(x+dx, y)
		if ch != "" && ch != " " {
			_, bg, _ := st.Decompose()
			return bg == activeBG
		}
	}
	_, st, _ := screen.Get(x, y)
	_, bg, _ := st.Decompose()
	return bg == activeBG
}
