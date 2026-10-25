package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// mouseSpyScreen records the mouse mode calls the app makes.
type mouseSpyScreen struct {
	tcell.SimulationScreen
	calls []string
}

func (s *mouseSpyScreen) EnableMouse(flags ...tcell.MouseFlags) {
	s.calls = append(s.calls, "enable")
	s.SimulationScreen.EnableMouse(flags...)
}

func (s *mouseSpyScreen) DisableMouse() {
	s.calls = append(s.calls, "disable")
	s.SimulationScreen.DisableMouse()
}

// mouseTestApp builds an app with mouse support on over a directory holding a subdirectory and
// several files. Both panels list the same directory.
func mouseTestApp(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"walnut", "pepper", "violin", "marble", "garden", "orchid", "pillow", "canyon", "ribbon", "thunder", "meadow", "lantern", "harbor", "velvet", "copper", "willow", "cactus", "anchor", "breeze", "falcon"} {
		writeFile(t, filepath.Join(dir, n+".txt"))
	}
	if err := os.MkdirAll(filepath.Join(dir, "aaa-kettle"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "aaa-kettle", "inside.txt"))
	app := newApp(t, newScreen(t, 100, 14), dir)
	app.config.UI.Mouse = true
	return app, dir
}

// cellOf returns a screen cell on the entry row idx of panel id (scrolled by the panel's offset).
func cellOf(t *testing.T, a *App, id, idx int) (int, int) {
	t.Helper()
	w, h := a.screen.Size()
	lay := a.layoutForTerminalSize(w, h)
	col, p := lay.Primary, &a.model.Primary
	if id == ui.SecondaryPanel {
		col, p = lay.Secondary, &a.model.Secondary
	}
	file := ui.FileListFrame(col, p, id, a.model.ActivePanel, false, a.selectedStripSplitParams(id, 0))
	return file.X + 3, ui.PanelListTop(file) + idx - p.ScrollOffset
}

func mouseEv(x, y int, b tcell.ButtonMask) *tcell.EventMouse {
	return tcell.NewEventMouse(x, y, b, tcell.ModNone)
}

// click sends a press and release.
func click(a *App, x, y int, b tcell.ButtonMask) {
	a.handleMouse(mouseEv(x, y, b))
	a.handleMouse(mouseEv(x, y, tcell.ButtonNone))
}

func TestMouseClickFocusesPanelAndMovesCursor(t *testing.T) {
	app, _ := mouseTestApp(t)
	x, y := cellOf(t, app, ui.PrimaryPanel, 4)
	click(app, x, y, tcell.Button1)
	if app.model.ActivePanel != ui.PrimaryPanel || app.model.Primary.Cursor != 4 {
		t.Fatalf("active=%d cursor=%d want primary/4", app.model.ActivePanel, app.model.Primary.Cursor)
	}
	x, y = cellOf(t, app, ui.SecondaryPanel, 6)
	click(app, x, y, tcell.Button1)
	if app.model.ActivePanel != ui.SecondaryPanel || app.model.Secondary.Cursor != 6 {
		t.Fatalf("active=%d cursor=%d want secondary/6", app.model.ActivePanel, app.model.Secondary.Cursor)
	}
	if app.model.Primary.Cursor != 4 {
		t.Fatalf("primary cursor moved to %d", app.model.Primary.Cursor)
	}
}

func TestMouseClickWithSwappedPanes(t *testing.T) {
	app, _ := mouseTestApp(t)
	app.model.SwapPanes = true
	lay := app.layoutForTerminalSize(100, 14)
	if lay.Secondary.X >= lay.Primary.X {
		t.Fatalf("expected secondary on the left, got %+v / %+v", lay.Secondary, lay.Primary)
	}
	x, y := cellOf(t, app, ui.SecondaryPanel, 3)
	click(app, x, y, tcell.Button1)
	if app.model.ActivePanel != ui.SecondaryPanel || app.model.Secondary.Cursor != 3 {
		t.Fatalf("active=%d cursor=%d want secondary/3", app.model.ActivePanel, app.model.Secondary.Cursor)
	}
}

func TestMouseDoubleClickOpensDirectory(t *testing.T) {
	app, dir := mouseTestApp(t)
	idx := -1
	for i := 0; i < app.model.Primary.VisibleEntryCount(); i++ {
		if e, _, ok := app.model.Primary.VisibleEntry(i); ok && e.Name == "aaa-kettle" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("fixture directory not listed")
	}
	x, y := cellOf(t, app, ui.PrimaryPanel, idx)
	click(app, x, y, tcell.Button1)
	if app.model.Primary.PathString() != dir {
		t.Fatal("single click must not open")
	}
	click(app, x, y, tcell.Button1)
	want := filepath.Join(dir, "aaa-kettle")
	drainInterruptEventsUntil(t, app, app.screen.(tcell.SimulationScreen), 2*time.Second, func() bool {
		return app.model.Primary.PathString() == want
	})
	if got := app.model.Primary.PathString(); got != want {
		t.Fatalf("path = %q want %q", got, want)
	}
}

func TestMouseHeldButtonDoesNotRepeatPress(t *testing.T) {
	app, _ := mouseTestApp(t)
	x, y := cellOf(t, app, ui.PrimaryPanel, 2)
	app.handleMouse(mouseEv(x, y, tcell.Button1))
	app.model.Primary.Cursor = 0
	app.handleMouse(mouseEv(x, y, tcell.Button1)) // drag report with the button still held
	if app.model.Primary.Cursor != 0 {
		t.Fatal("held button re-fired the press")
	}
}

func TestMouseWheelScrollsPanelUnderPointerWithoutFocus(t *testing.T) {
	app, _ := mouseTestApp(t)
	x, y := cellOf(t, app, ui.PrimaryPanel, 1)
	app.handleMouse(mouseEv(x, y, tcell.WheelDown))
	if got := app.model.Primary.Cursor; got != 3 {
		t.Fatalf("active cursor = %d want 3", got)
	}
	app.handleMouse(mouseEv(x, y, tcell.WheelUp))
	if got := app.model.Primary.Cursor; got != 0 {
		t.Fatalf("active cursor = %d want 0", got)
	}
	x, y = cellOf(t, app, ui.SecondaryPanel, 1)
	app.handleMouse(mouseEv(x, y, tcell.WheelDown))
	if app.model.ActivePanel != ui.PrimaryPanel || app.model.Secondary.Cursor != 3 {
		t.Fatalf("active=%d secondary cursor=%d want primary/3", app.model.ActivePanel, app.model.Secondary.Cursor)
	}
}

func TestMouseRightClickTogglesSelection(t *testing.T) {
	app, _ := mouseTestApp(t)
	x, y := cellOf(t, app, ui.PrimaryPanel, 5)
	click(app, x, y, tcell.Button2)
	if n := app.model.Primary.SelectedPathCount(); n != 1 {
		t.Fatalf("selected = %d want 1", n)
	}
}

func TestMouseSwallowedWhileDialogOpen(t *testing.T) {
	app, _ := mouseTestApp(t)
	app.openQuitConfirm()
	if app.inputMode() == InputModeNormal {
		t.Fatal("dialog did not open")
	}
	x, y := cellOf(t, app, ui.PrimaryPanel, 4)
	click(app, x, y, tcell.Button1)
	app.handleMouse(mouseEv(x, y, tcell.WheelDown))
	if app.model.Primary.Cursor != 0 {
		t.Fatalf("cursor moved to %d under a dialog", app.model.Primary.Cursor)
	}
}

func TestMouseEnabledOnlyByOption(t *testing.T) {
	spy := &mouseSpyScreen{SimulationScreen: newScreen(t, 80, 24)}
	app := newTestApp(t, spy, testOptions(t.TempDir()))
	app.applyMouseConfig()
	if len(spy.calls) != 0 {
		t.Fatalf("mouse off: unexpected screen calls %v", spy.calls)
	}
	app.config.UI.Mouse = true
	app.applyMouseConfig()
	app.applyMouseConfig()
	if len(spy.calls) != 1 || spy.calls[0] != "enable" {
		t.Fatalf("calls = %v want [enable]", spy.calls)
	}
	app.config.UI.Mouse = false
	app.applyMouseConfig()
	if len(spy.calls) != 2 || spy.calls[1] != "disable" {
		t.Fatalf("calls = %v want [enable disable]", spy.calls)
	}
}

func TestMouseOffAroundTerminalRelease(t *testing.T) {
	spy := &mouseSpyScreen{SimulationScreen: newScreen(t, 80, 24)}
	opts := testOptions(t.TempDir())
	opts.Config.UI.Mouse = true
	app := newTestApp(t, spy, opts)
	app.applyMouseConfig()
	spy.calls = nil
	var inside []string
	err := app.withTerminalReleased(func() error {
		inside = append(inside, spy.calls...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(inside) != 1 || inside[0] != "disable" {
		t.Fatalf("calls before hand-off = %v want [disable]", inside)
	}
	if len(spy.calls) != 2 || spy.calls[1] != "enable" {
		t.Fatalf("calls = %v want [disable enable]", spy.calls)
	}
}

// screenTextCell renders the app and returns the first cell showing text, scanning rows top
// down. Panel glyphs are one cell wide here, so a rune index is a column.
func screenTextCell(t *testing.T, a *App, text string) (int, int) {
	t.Helper()
	a.render()
	screen := a.screen.(tcell.SimulationScreen)
	w, h := screen.Size()
	want := []rune(text)
	for y := 0; y < h; y++ {
		row := []rune(tcelltest.TextAt(screen, 0, y, w))
		for x := 0; x+len(want) <= len(row); x++ {
			if string(row[x:x+len(want)]) == text {
				return x, y
			}
		}
	}
	t.Fatalf("%q not on screen", text)
	return 0, 0
}

func tallMouseTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"walnut", "pepper", "violin"} {
		writeFile(t, filepath.Join(dir, n+".txt"))
	}
	app := newApp(t, newScreen(t, 100, 30), dir)
	app.config.UI.Mouse = true
	return app
}

func TestMouseMenuBarClickTogglesPulldown(t *testing.T) {
	app := tallMouseTestApp(t)
	x, y := screenTextCell(t, app, " File ")
	click(app, x+2, y, tcell.Button1)
	menus := menu.ActiveDefinitions(app.model.MenuDefinitions)
	if !app.model.Menu.Open || !app.model.Menu.PulldownOpen || menus[app.model.Menu.ActiveMenu].ID != menu.TopFile {
		t.Fatalf("menu state %+v want File pulldown", app.model.Menu)
	}
	click(app, x+2, y, tcell.Button1)
	if app.model.Menu.Open {
		t.Fatal("second click on the open menu's label should close it")
	}
}

func TestMouseMenuItemClickActivates(t *testing.T) {
	app := tallMouseTestApp(t)
	x, y := screenTextCell(t, app, " Left ")
	click(app, x+2, y, tcell.Button1)
	x, y = screenTextCell(t, app, "Sort...")
	click(app, x, y, tcell.Button1)
	if app.model.Menu.Open || !app.model.SortDialog.Open {
		t.Fatalf("menu open=%v sort dialog open=%v want closed/open", app.model.Menu.Open, app.model.SortDialog.Open)
	}
}

func TestMouseMenuWheelAndOutsideClick(t *testing.T) {
	app := tallMouseTestApp(t)
	x, y := screenTextCell(t, app, " Left ")
	click(app, x+2, y, tcell.Button1)
	ix, iy := screenTextCell(t, app, "Sort...")
	first := app.model.Menu.SelectedItem
	app.handleMouse(mouseEv(ix, iy, tcell.WheelDown))
	if app.model.Menu.SelectedItem != first+1 {
		t.Fatalf("selected = %d want %d", app.model.Menu.SelectedItem, first+1)
	}
	click(app, 90, 20, tcell.Button1)
	if app.model.Menu.Open {
		t.Fatal("click outside the pulldown should close the menu")
	}
	if app.model.SortDialog.Open {
		t.Fatal("outside click must not reach what lies below")
	}
}

func TestMouseFooterClickPressesKey(t *testing.T) {
	app := tallMouseTestApp(t)
	x, y := screenTextCell(t, app, "F9 Menu")
	click(app, x+4, y, tcell.Button1)
	if !app.model.Menu.Open {
		t.Fatal("footer F9 click did not open the menu")
	}
	x, y = screenTextCell(t, app, "Esc Close")
	click(app, x, y, tcell.Button1)
	if app.model.Menu.Open {
		t.Fatal("footer Esc click did not close the menu")
	}
}

func TestFooterKeyEventFromBindingLabel(t *testing.T) {
	ev, ok := menu.FunctionKey{KeyLabel: "M-o", Hint: "Open"}.Event()
	if !ok || ev.Key() != tcell.KeyRune || ev.Rune() != 'o' || ev.Modifiers()&tcell.ModAlt == 0 {
		t.Fatalf("event = %v ok=%v want Alt+o", ev, ok)
	}
	ev, _ = menu.FunctionKey{Key: tcell.KeyUp, Mod: tcell.ModCtrl, KeyLabel: "C-up"}.Event()
	if ev.Key() != tcell.KeyUp || ev.Modifiers() != tcell.ModCtrl {
		t.Fatalf("event = %v want Ctrl+Up", ev)
	}
}

func TestMouseLeaderMenuClickRunsEntry(t *testing.T) {
	app := tallMouseTestApp(t)
	app.openBuiltinLeaderMenu()
	x, y := screenTextCell(t, app, "Sort dialog")
	click(app, x+3, y, tcell.Button1)
	if app.model.LeaderMenu.Open || !app.model.SortDialog.Open {
		t.Fatalf("leader open=%v sort dialog open=%v want closed/open", app.model.LeaderMenu.Open, app.model.SortDialog.Open)
	}
}

func TestMouseLeaderMenuOutsideClickCloses(t *testing.T) {
	app := tallMouseTestApp(t)
	app.openBuiltinLeaderMenu()
	x, y := cellOf(t, app, ui.PrimaryPanel, 1)
	click(app, x, y, tcell.Button1)
	if app.model.LeaderMenu.Open {
		t.Fatal("click outside the strip should close the leader menu")
	}
	if app.model.Primary.Cursor != 0 {
		t.Fatal("outside click must not reach the panel")
	}
}

func TestMouseMessagesListClickAndWheel(t *testing.T) {
	app := tallMouseTestApp(t)
	app.model.MessageLog = []ui.MessageLogEntry{
		{Time: "10:00:01", Text: "harbor opened"},
		{Text: "harbor continued"},
		{Time: "10:00:02", Text: "meadow copied"},
		{Time: "10:00:03", Text: "lantern moved"},
		{Time: "10:00:04", Text: "velvet deleted"},
	}
	app.openMessagesView()
	x, y := screenTextCell(t, app, "meadow copied")
	click(app, x, y, tcell.Button1)
	if got := app.model.MessagesView.Selected; got != 2 {
		t.Fatalf("selected = %d want 2", got)
	}
	x, y = screenTextCell(t, app, "harbor continued")
	click(app, x, y, tcell.Button1)
	if got := app.model.MessagesView.Selected; got != 0 {
		t.Fatalf("continuation row selected %d want its message 0", got)
	}
	app.handleMouse(mouseEv(x, y, tcell.WheelDown))
	if got := app.model.MessagesView.Selected; got != 4 {
		t.Fatalf("after wheel selected = %d want 4", got)
	}
}

func TestMouseCommandsListClickSelectsEntry(t *testing.T) {
	app := tallMouseTestApp(t)
	app.commandsMu.Lock()
	app.model.CommandsList = []ui.CommandRunEntry{
		{ID: "1", UserCommandLine: "echo walnut", Phase: ui.CommandRunDone},
		{ID: "2", UserCommandLine: "echo pepper", Phase: ui.CommandRunDone},
		{ID: "3", UserCommandLine: "echo violin", Phase: ui.CommandRunDone},
	}
	app.commandsMu.Unlock()
	app.commandsCtrl.OpenView()
	app.model.CommandsView.FocusPane = 1
	x, y := screenTextCell(t, app, "echo pepper")
	click(app, x, y, tcell.Button1)
	if app.model.CommandsView.Selected != 1 || app.model.CommandsView.FocusPane != 0 {
		t.Fatalf("selected=%d focus=%d want 1/0", app.model.CommandsView.Selected, app.model.CommandsView.FocusPane)
	}
	x, y = screenTextCell(t, app, "echo walnut")
	click(app, x, y, tcell.Button1)
	if app.model.CommandsView.Selected != 0 {
		t.Fatalf("selected=%d want 0 (oldest, bottom row)", app.model.CommandsView.Selected)
	}
}
