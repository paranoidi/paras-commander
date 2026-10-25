package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
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
