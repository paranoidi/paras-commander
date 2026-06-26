package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func TestCompareViewFooterEscFirst(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.ViewMode = ui.ViewCompare
	keys := app.activeFooterKeys()
	if len(keys) == 0 {
		t.Fatal("footer keys empty")
	}
	if keys[0] != menu.FooterEscClose {
		t.Fatalf("footer[0] = %+v, want Esc Close", keys[0])
	}
	var foundMerge bool
	for _, fk := range keys {
		if fk.Key == tcell.KeyF5 && fk.Hint == "Merge" {
			foundMerge = true
		}
	}
	if !foundMerge {
		t.Fatalf("footer missing F5 Merge: %+v", keys)
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
