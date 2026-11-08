package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/clipboard"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/statuscmds"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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

func TestMouseMessagesListClickCopiesAndWheel(t *testing.T) {
	prevWriter := clipboard.OSWriter
	clipboard.OSWriter = func(string) error { return nil }
	t.Cleanup(func() { clipboard.OSWriter = prevWriter; clipboard.Reset() })
	app := tallMouseTestApp(t)
	app.model.MessageLog = []ui.MessageLogEntry{
		{Time: "10:00:01", Text: "harbor opened", Full: "harbor opened harbor continued"},
		{Text: "harbor continued"},
		{Time: "10:00:02", Text: "meadow copied"},
		{Time: "10:00:03", Text: "lantern moved"},
		{Time: "10:00:04", Text: "velvet deleted"},
	}
	app.openMessagesView()
	selectedText := func() string { return app.model.MessageLog[app.model.MessagesView.Selected].Text }

	// The top message: the logged toast must not take the selection from it.
	x, y := screenTextCell(t, app, "harbor continued")
	click(app, x, y, tcell.Button1)
	if got := clipboard.LastSet(); got != "harbor opened harbor continued" {
		t.Fatalf("clipboard = %q, want the whole message", got)
	}
	if got := app.model.MessageLog[0].Text; got != "Copied message to clipboard" {
		t.Fatalf("newest log entry = %q, want the copy toast", got)
	}
	if got := selectedText(); got != "harbor opened" {
		t.Fatalf("continuation row selected %q, want its message", got)
	}

	x, y = screenTextCell(t, app, "meadow copied")
	click(app, x, y, tcell.Button1)
	if got := clipboard.LastSet(); got != "meadow copied" {
		t.Fatalf("clipboard = %q, want meadow copied", got)
	}
	if got := selectedText(); got != "meadow copied" {
		t.Fatalf("selected %q, want meadow copied", got)
	}
	app.handleMouse(mouseEv(x, y, tcell.WheelDown))
	if got := selectedText(); got != "velvet deleted" {
		t.Fatalf("after wheel selected %q, want velvet deleted", got)
	}
}

// Motion over a list row highlights it without moving the selection.
func TestMouseAuxListHoverKeepsSelection(t *testing.T) {
	app := tallMouseTestApp(t)
	app.model.MessageLog = []ui.MessageLogEntry{
		{Time: "10:00:01", Text: "harbor opened"},
		{Text: "harbor continued"},
		{Time: "10:00:02", Text: "meadow copied"},
	}
	app.openMessagesView()
	x, y := screenTextCell(t, app, "harbor continued")
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	if got := app.model.AuxListHover; !got.Active || got.Row != 0 {
		t.Fatalf("hover on a continuation row = %+v, want its message 0", got)
	}
	x, y = screenTextCell(t, app, "meadow copied")
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	if got := app.model.AuxListHover; !got.Active || got.Row != 2 {
		t.Fatalf("hover = %+v, want row 2", got)
	}
	if app.model.MessagesView.Selected != 0 {
		t.Fatalf("selected = %d, want 0: hover must not move the selection", app.model.MessagesView.Selected)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.render()
	if app.model.AuxListHover.Active {
		t.Fatal("a key press must hide the row hover")
	}
	app.handleMouse(mouseEv(x+1, y, tcell.ButtonNone))
	if !app.model.AuxListHover.Active {
		t.Fatal("pointer motion must bring the row hover back")
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

func TestMouseDialogOptionsAndOK(t *testing.T) {
	app := tallMouseTestApp(t)
	app.openSortDialog()
	x, y := screenTextCell(t, app, "Extension")
	click(app, x, y+1, tcell.Button1) // Size, the row below (the panel header also says Size)
	if app.model.SortDialog.SortMode != panel.SortSize {
		t.Fatalf("radio click: mode=%v want size", app.model.SortDialog.SortMode)
	}
	x, y = screenTextCell(t, app, "Reverse")
	click(app, x, y, tcell.Button1)
	if !app.model.SortDialog.SortReverse {
		t.Fatal("checkbox click did not toggle Reverse")
	}
	x, y = screenTextCell(t, app, "[ OK ]")
	click(app, x+2, y, tcell.Button1)
	if app.model.SortDialog.Open {
		t.Fatal("OK click did not apply and close")
	}
	if app.model.Primary.Sort.Mode != panel.SortSize || !app.model.Primary.Sort.Reverse {
		t.Fatalf("panel sort = %+v want size, reversed", app.model.Primary.Sort)
	}
}

func TestMouseDialogButtonPressQuits(t *testing.T) {
	app := tallMouseTestApp(t)
	app.openQuitConfirm()
	x, y := screenTextCell(t, app, "Quit Anyway")
	if quit, _ := app.handleMouse(mouseEv(x, y, tcell.Button1)); !quit {
		t.Fatal("Quit Anyway click did not quit")
	}
}

func TestMouseDialogMissIsSwallowed(t *testing.T) {
	app := tallMouseTestApp(t)
	app.openSortDialog()
	x, y := screenTextCell(t, app, "Sort order")
	click(app, x, y, tcell.Button1)  // the title on the frame
	click(app, 2, 25, tcell.Button1) // the panel under the dialog
	if !app.model.SortDialog.Open || app.model.SortDialog.Focus != "" {
		t.Fatalf("miss changed the dialog: open=%v focus=%q", app.model.SortDialog.Open, app.model.SortDialog.Focus)
	}
}

func TestMouseDialogListRowsAndWheel(t *testing.T) {
	app := sftpKeyApp(t, t.TempDir(), 12)
	st := &app.model.SFTPConnectDialog
	x, y := screenTextCell(t, app, listGoldenWords[2])
	click(app, x, y, tcell.Button1)
	if st.Selected != 2 || st.Focus != dialog.SFTPFilter {
		t.Fatalf("row click: selected=%d focus=%q want 2 / the filter that drives the list", st.Selected, st.Focus)
	}
	app.handleMouse(mouseEv(x, y, tcell.WheelDown))
	if st.Selected != 2+config.MouseWheelRows {
		t.Fatalf("wheel: selected=%d want %d", st.Selected, 2+config.MouseWheelRows)
	}
	x, y = screenTextCell(t, app, listGoldenWords[1])
	click(app, x, y, tcell.Button1)
	click(app, x, y, tcell.Button1) // double click: Enter in the filter takes the host
	if st.Selected != 1 || st.Focus != dialog.SFTPLocation || !strings.Contains(st.Location.Value, listGoldenWords[1]) {
		t.Fatalf("double click: selected=%d focus=%q location=%q", st.Selected, st.Focus, st.Location.Value)
	}
}

func TestMouseDialogInputPlacesCursor(t *testing.T) {
	app := tallMouseTestApp(t)
	app.openGroupSelect("select", "panel")
	gs := &app.model.GroupSelect
	gs.Text, gs.TextCursor = "harbor*.txt", 0
	gs.Focus = dialog.PatternRegexRadio
	x, y := screenTextCell(t, app, "harbor*.txt")
	click(app, x+4, y, tcell.Button1)
	if gs.Focus != dialog.PatternInput || gs.TextCursor != 4 {
		t.Fatalf("input click: focus=%q cursor=%d want input / 4", gs.Focus, gs.TextCursor)
	}
}

func TestMouseDialogCheckboxClickTogglesWhereEnterConfirms(t *testing.T) {
	app := tallMouseTestApp(t)
	app.openGroupSelect("select", "panel")
	gs := &app.model.GroupSelect
	gs.Text = "harbor*.txt"
	x, y := screenTextCell(t, app, "Case sensitive")
	click(app, x, y, tcell.Button1)
	if !gs.Open || !gs.CaseSensitive || gs.Focus != dialog.PatternCase {
		t.Fatalf("checkbox click: open=%v case=%v focus=%q, want it toggled like Space, not confirmed like Enter", gs.Open, gs.CaseSensitive, gs.Focus)
	}
}

func TestMouseDialogInputClickOnlyMovesCursor(t *testing.T) {
	app := sftpKeyApp(t, t.TempDir(), 12)
	st := &app.model.SFTPConnectDialog
	st.Location.Value, st.Location.Cursor, st.Location.PrefillPending = "sftp://keel.example", 0, false
	x, y := screenTextCell(t, app, "sftp://keel.example")
	click(app, x+7, y, tcell.Button1)
	if !st.Open || st.Focus != dialog.SFTPLocation || st.Location.Cursor != 7 {
		t.Fatalf("input click: open=%v focus=%q cursor=%d, want the location focused at 7 and the dialog open", st.Open, st.Focus, st.Location.Cursor)
	}
}

func TestMouseDialogWheelScrollsScrollOnlyList(t *testing.T) {
	dir := t.TempDir()
	app := newApp(t, newScreen(t, 100, 30), dir)
	app.config.UI.Mouse = true
	p := app.activePanel()
	for i := 1; i <= 25; i++ {
		name := filepath.Join(dir, fmt.Sprintf("thicket%02d.txt", i))
		writeFile(t, name)
		p.AddSelection(name)
	}
	dispatchFileOp(t, app, keymap.ActionFileDelete)
	d := &app.model.FileDialog
	focus := d.Focus
	x, y := screenTextCell(t, app, "thicket01.txt")
	click(app, x, y, tcell.Button1)
	if d.Focus != focus || d.DeleteListScroll != 0 {
		t.Fatalf("click on the delete list: focus %q scroll %d, want unchanged", d.Focus, d.DeleteListScroll)
	}
	app.handleMouse(mouseEv(x, y, tcell.WheelDown))
	if d.DeleteListScroll != config.MouseWheelRows || d.Focus != focus {
		t.Fatalf("wheel: scroll %d focus %q, want %d and focus kept", d.DeleteListScroll, d.Focus, config.MouseWheelRows)
	}
}

// titleSegmentAt returns a cell on the title path segment depth levels above panel id's
// directory, scanning the title row through ui.PanelTitleSegmentAt.
func titleSegmentAt(t *testing.T, a *App, id, depth int) (int, int) {
	t.Helper()
	w, h := a.screen.Size()
	lay := a.layoutForTerminalSize(w, h)
	col := lay.Primary
	if id == ui.SecondaryPanel {
		col = lay.Secondary
	}
	_, file, ok := a.mousePanelAt(lay, col.X+1, col.Y+1)
	if !ok {
		t.Fatal("panel not under the pointer")
	}
	for x := file.X; x < file.X+file.Width; x++ {
		if seg, ok := ui.PanelTitleSegmentAt(file, &a.model, id, a.styles, x, file.Y); ok && seg.Depth == depth {
			return x, file.Y
		}
	}
	t.Fatalf("no title segment at depth %d", depth)
	return 0, 0
}

func TestMouseHoverInvertsTitleSegment(t *testing.T) {
	app, _ := mouseTestApp(t)
	x, y := titleSegmentAt(t, app, ui.PrimaryPanel, 1)
	if _, rendered := app.handleMouse(mouseEv(x, y, tcell.ButtonNone)); !rendered {
		t.Fatal("entering a title segment must render")
	}
	if h := app.model.PanelTitleHover; !h.Active || h.Panel != ui.PrimaryPanel || h.X != x {
		t.Fatalf("hover = %+v", h)
	}
	_, style, _ := app.screen.Get(x, y)
	if _, _, attrs := style.Decompose(); attrs&tcell.AttrReverse == 0 {
		t.Fatal("hovered segment not inverted")
	}
	if _, rendered := app.handleMouse(mouseEv(x, y, tcell.ButtonNone)); rendered {
		t.Fatal("motion within the same segment must not render")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.PanelTitleHover.Active {
		t.Fatal("a key press must hide the title hover")
	}
	if _, rendered := app.handleMouse(mouseEv(x, y, tcell.ButtonNone)); !rendered || !app.model.PanelTitleHover.Active {
		t.Fatal("pointer motion must bring the title hover back")
	}
	cx, cy := cellOf(t, app, ui.PrimaryPanel, 2)
	if _, rendered := app.handleMouse(mouseEv(cx, cy, tcell.ButtonNone)); !rendered || app.model.PanelTitleHover.Active {
		t.Fatalf("leaving the title must clear the hover and render (hover %+v)", app.model.PanelTitleHover)
	}
	_, style, _ = app.screen.Get(x, y)
	if _, _, attrs := style.Decompose(); attrs&tcell.AttrReverse != 0 {
		t.Fatal("segment still inverted after the pointer left")
	}
}

func TestMouseClickTitleSegmentNavigatesToAncestor(t *testing.T) {
	app, dir := mouseTestApp(t)
	x, y := titleSegmentAt(t, app, ui.PrimaryPanel, 1)
	click(app, x, y, tcell.Button1)
	want := filepath.Dir(dir)
	drainInterruptEventsUntil(t, app, app.screen.(tcell.SimulationScreen), 2*time.Second, func() bool {
		return app.model.Primary.PathString() == want
	})
	if got := app.model.Primary.PathString(); got != want {
		t.Fatalf("path = %q want %q", got, want)
	}
	if app.model.ActivePanel != ui.PrimaryPanel {
		t.Fatalf("active = %d want primary", app.model.ActivePanel)
	}
	if e, ok := app.model.Primary.CurrentEntry(); !ok || e.Name != filepath.Base(dir) {
		t.Fatalf("cursor entry = %+v, want %q", e, filepath.Base(dir))
	}
	if app.model.Secondary.PathString() != dir {
		t.Fatal("other panel must not navigate")
	}
}

func TestMouseClickTitlePanelDirectoryRefreshes(t *testing.T) {
	app, dir := mouseTestApp(t)
	// The panel directory starts after the parent segment and its "/" separator.
	px, y := titleSegmentAt(t, app, ui.PrimaryPanel, 1)
	app.render()
	x := -1
	for cx := px; cx < px+100 && x < 0; cx++ {
		if str, _, _ := app.screen.Get(cx, y); str == "/" {
			x = cx + 1
		}
	}
	if x < 0 {
		t.Fatal("no separator after the parent segment")
	}
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	if _, style, _ := app.screen.Get(x, y); func() bool { _, _, attrs := style.Decompose(); return attrs&tcell.AttrReverse == 0 }() {
		t.Fatal("hovered panel directory not inverted")
	}
	writeFile(t, filepath.Join(dir, "zz-sparrow.txt"))
	click(app, x, y, tcell.Button1)
	listed := func() bool {
		for i := 0; i < app.model.Primary.VisibleEntryCount(); i++ {
			if e, _, ok := app.model.Primary.VisibleEntry(i); ok && e.Name == "zz-sparrow.txt" {
				return true
			}
		}
		return false
	}
	drainInterruptEventsUntil(t, app, app.screen.(tcell.SimulationScreen), 2*time.Second, listed)
	if !listed() || app.model.Primary.PathString() != dir {
		t.Fatalf("click on the panel directory did not refresh it (path %q)", app.model.Primary.PathString())
	}
}

// styleAt is the style drawn at (x, y).
func styleAt(a *App, x, y int) tcell.Style {
	_, style, _ := a.screen.Get(x, y)
	return style
}

func TestMouseHoverHighlightsDialogButton(t *testing.T) {
	app := tallMouseTestApp(t)
	app.openSortDialog()
	x, y := screenTextCell(t, app, "[ Cancel ]") // x+3 is "a", past the mnemonic
	focus := app.model.SortDialog.Focus
	plain := styleAt(app, x+3, y)
	if _, rendered := app.handleMouse(mouseEv(x+3, y, tcell.ButtonNone)); !rendered {
		t.Fatal("entering a button must render")
	}
	if got := styleAt(app, x+3, y); got != app.styles.DialogButtonActive {
		t.Fatalf("hovered button style = %v want %v (plain %v)", got, app.styles.DialogButtonActive, plain)
	}
	if app.model.SortDialog.Focus != focus {
		t.Fatalf("hover moved focus to %q", app.model.SortDialog.Focus)
	}
	if _, rendered := app.handleMouse(mouseEv(x+4, y, tcell.ButtonNone)); rendered {
		t.Fatal("motion within the same button must not render")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	app.render()
	if app.model.DialogButtonHover.Active {
		t.Fatal("a key press must hide the button hover")
	}
	if app.handleMouse(mouseEv(x+3, y, tcell.ButtonNone)); !app.model.DialogButtonHover.Active {
		t.Fatal("pointer motion must bring the button hover back")
	}
	if _, rendered := app.handleMouse(mouseEv(1, 1, tcell.ButtonNone)); !rendered {
		t.Fatal("leaving the button must render")
	}
	if got := styleAt(app, x+3, y); got != plain {
		t.Fatalf("button style after leaving = %v want %v", got, plain)
	}
}

func TestMouseHoverHighlightsMenuTitle(t *testing.T) {
	app := tallMouseTestApp(t)
	x, y := screenTextCell(t, app, " File ")
	plain := styleAt(app, x+2, y)
	if _, rendered := app.handleMouse(mouseEv(x+2, y, tcell.ButtonNone)); !rendered {
		t.Fatal("entering a menu title must render")
	}
	if got := styleAt(app, x+2, y); got != app.styles.MenuBarSelected {
		t.Fatalf("hovered title style = %v want %v", got, app.styles.MenuBarSelected)
	}
	if app.model.Menu.Open {
		t.Fatal("hover opened the menu")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.render()
	if app.model.MenuBarHover.Active {
		t.Fatal("a key press must hide the title hover")
	}
	app.handleMouse(mouseEv(x+3, y, tcell.ButtonNone))
	if !app.model.MenuBarHover.Active {
		t.Fatal("pointer motion must bring the title hover back")
	}
	cx, cy := cellOf(t, app, ui.PrimaryPanel, 1)
	app.handleMouse(mouseEv(cx, cy, tcell.ButtonNone))
	if got := styleAt(app, x+2, y); got != plain {
		t.Fatalf("title style after leaving = %v want %v", got, plain)
	}
}

func TestMousePulldownSelectionFollowsPointer(t *testing.T) {
	app := tallMouseTestApp(t)
	x, y := screenTextCell(t, app, " Left ")
	click(app, x+2, y, tcell.Button1)
	ix, iy := screenTextCell(t, app, "Sort...")
	w, h := app.screen.Size()
	want, _ := ui.PulldownItemAt(app.layoutForTerminalSize(w, h), &app.model, ix, iy)
	if want == app.model.Menu.SelectedItem {
		t.Fatal("fixture: Sort... is already selected")
	}
	if _, rendered := app.handleMouse(mouseEv(ix, iy, tcell.ButtonNone)); !rendered {
		t.Fatal("moving onto another item must render")
	}
	if app.model.Menu.SelectedItem != want {
		t.Fatalf("selected = %d want %d", app.model.Menu.SelectedItem, want)
	}
	if _, rendered := app.handleMouse(mouseEv(ix+1, iy, tcell.ButtonNone)); rendered {
		t.Fatal("motion within the same item must not render")
	}
	app.handleMouse(mouseEv(90, 20, tcell.ButtonNone))
	if !app.model.Menu.Open || app.model.Menu.SelectedItem != want {
		t.Fatalf("leaving the pulldown changed the menu: open=%v selected=%d", app.model.Menu.Open, app.model.Menu.SelectedItem)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.model.SortDialog.Open {
		t.Fatal("Enter did not run the item under the pointer")
	}
}

func TestMouseStatusCommandsClickOpensDialog(t *testing.T) {
	app := tallMouseTestApp(t)
	app.statusCmds = []*statusCmdRunner{{def: statuscmds.Def{Name: "quartz"}, text: "flint"}}
	app.render()
	x, y := screenTextCell(t, app, "flint")
	click(app, x, y, tcell.Button1)
	if !app.model.StatusCommandsDialog.Open {
		t.Fatal("click on the status-command area did not open the Status commands dialog")
	}
}

func TestMouseStatusCommandsClickClosesMenu(t *testing.T) {
	app := tallMouseTestApp(t)
	app.statusCmds = []*statusCmdRunner{{def: statuscmds.Def{Name: "quartz"}, text: "flint"}}
	app.render()
	x, y := screenTextCell(t, app, " File ")
	click(app, x+2, y, tcell.Button1)
	x, y = screenTextCell(t, app, "flint")
	click(app, x, y, tcell.Button1)
	if app.model.Menu.Open || !app.model.StatusCommandsDialog.Open {
		t.Fatalf("menu open=%v dialog open=%v want closed/open", app.model.Menu.Open, app.model.StatusCommandsDialog.Open)
	}
}

func TestMouseDialogListHoverKeepsCursor(t *testing.T) {
	app := sftpKeyApp(t, t.TempDir(), 12)
	st := &app.model.SFTPConnectDialog
	selected, focus := st.Selected, st.Focus
	x, y := screenTextCell(t, app, listGoldenWords[2])
	if _, rendered := app.handleMouse(mouseEv(x, y, tcell.ButtonNone)); !rendered {
		t.Fatal("moving onto another row must render its hover")
	}
	if st.Selected != selected || st.Focus != focus {
		t.Fatalf("hover: selected=%d focus=%q want %d / %q", st.Selected, st.Focus, selected, focus)
	}
	if app.model.DialogListHover.Y != y || app.model.DialogListHover.Width == 0 {
		t.Fatalf("hover row = %+v, want row %d", app.model.DialogListHover, y)
	}
	if _, rendered := app.handleMouse(mouseEv(x+1, y, tcell.ButtonNone)); rendered {
		t.Fatal("motion within the same row must not render")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	app.render()
	if app.model.DialogListHover.Width != 0 {
		t.Fatal("a key press must hide the row hover")
	}
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	if app.model.DialogListHover.Width == 0 {
		t.Fatal("pointer motion must bring the row hover back")
	}
}

func TestMousePanelRowHoverHiddenByKeyboard(t *testing.T) {
	app, _ := mouseTestApp(t)
	x, y := cellOf(t, app, ui.PrimaryPanel, 3)
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	if !app.model.PanelRowHover.Active {
		t.Fatal("pointer over a row must show its hover")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.render()
	if app.model.PanelRowHover.Active {
		t.Fatal("a key press must hide the row hover")
	}
	app.handleMouse(mouseEv(x+1, y, tcell.ButtonNone))
	if !app.model.PanelRowHover.Active {
		t.Fatal("pointer motion must bring the row hover back")
	}
}

func compareMouseTestApp(t *testing.T) *App {
	t.Helper()
	app := tallMouseTestApp(t)
	app.model.ViewMode = ui.ViewCompare
	app.model.CompareView = ui.CompareViewState{FocusColumn: ui.CompareColumnPrimary, Filter: comparepkg.FilterAll}
	var rows []comparepkg.Row
	for _, n := range []string{"harbor", "meadow", "lantern", "falcon"} {
		rows = append(rows, comparepkg.Row{PrimaryRel: n + ".txt", SecondaryRel: n + ".txt", Kind: comparepkg.KindContentDiff, HashDone: true})
	}
	app.model.CompareSnapshot = comparepkg.Snapshot{Rows: rows}
	return app
}

func TestMouseCompareHoverKeepsSelection(t *testing.T) {
	app := compareMouseTestApp(t)
	x, y := screenTextCell(t, app, "lantern.txt")
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	if got := app.model.AuxListHover; !got.Active || got.Row != 2 {
		t.Fatalf("hover = %+v, want row 2", got)
	}
	if app.model.CompareView.Selected != 0 {
		t.Fatalf("selected = %d, want 0: hover must not move the selection", app.model.CompareView.Selected)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.render()
	if app.model.AuxListHover.Active {
		t.Fatal("a key press must hide the row hover")
	}
	app.handleMouse(mouseEv(x+1, y, tcell.ButtonNone))
	if !app.model.AuxListHover.Active {
		t.Fatal("pointer motion must bring the row hover back")
	}
}

func TestMouseCompareClickSelectsRow(t *testing.T) {
	app := compareMouseTestApp(t)
	x, y := screenTextCell(t, app, "lantern.txt")
	click(app, x, y, tcell.Button1)
	if app.model.CompareView.Selected != 2 {
		t.Fatalf("selected = %d, want 2", app.model.CompareView.Selected)
	}
}

func dedupMouseTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("meadow", "lantern.txt"), filepath.Join("meadow", "beacon.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app := newApp(t, newScreen(t, 100, 30), dir)
	app.config.UI.Mouse = true
	app.openFindDuplicates()
	waitDedupDone(t, app)
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if len(app.model.DedupList) < 2 || len(app.model.DedupCopiesList) < 2 {
		t.Fatalf("need two rows per pane, got %d/%d", len(app.model.DedupList), len(app.model.DedupCopiesList))
	}
	return app
}

// dedupPaneCell is a cell on row idx of the main tree or the Copies pane.
func dedupPaneCell(a *App, copies bool, idx int) (int, int) {
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	rect := layout.Primary
	if copies {
		rect, _ = ui.DedupSecondaryRects(layout.Secondary)
	}
	return rect.X + 3, ui.PanelListTop(rect) + idx
}

func TestMouseDedupHoverKeepsSelection(t *testing.T) {
	app := dedupMouseTestApp(t)
	app.render()
	sel := app.model.DedupView.Main.Selected
	row := 0
	if sel == 0 {
		row = 1
	}
	x, y := dedupPaneCell(app, false, row)
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	if got := app.model.AuxListHover; !got.Active || got.Copies || got.Row != row {
		t.Fatalf("hover = %+v, want main row %d", got, row)
	}
	x2, y2 := dedupPaneCell(app, true, 1)
	app.handleMouse(mouseEv(x2, y2, tcell.ButtonNone))
	if got := app.model.AuxListHover; !got.Active || !got.Copies || got.Row != 1 {
		t.Fatalf("hover = %+v, want copies row 1", got)
	}
	if app.model.DedupView.Main.Selected != sel || app.model.DedupView.FocusCopies {
		t.Fatal("hover must not move the selection or focus")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.render()
	if app.model.AuxListHover.Active {
		t.Fatal("a key press must hide the row hover")
	}
	app.handleMouse(mouseEv(x2+1, y2, tcell.ButtonNone))
	if !app.model.AuxListHover.Active {
		t.Fatal("pointer motion must bring the row hover back")
	}
}

func TestMouseDedupClickSelectsAndFocusesPane(t *testing.T) {
	app := dedupMouseTestApp(t)
	app.render()
	x, y := dedupPaneCell(app, true, 1)
	click(app, x, y, tcell.Button1)
	if !app.model.DedupView.FocusCopies || app.model.DedupView.Copies.Selected != 1 {
		t.Fatalf("focusCopies=%v copies selected=%d, want true/1", app.model.DedupView.FocusCopies, app.model.DedupView.Copies.Selected)
	}
	x, y = dedupPaneCell(app, false, 0)
	click(app, x, y, tcell.Button1)
	if app.model.DedupView.FocusCopies || app.model.DedupView.Main.Selected != 0 {
		t.Fatalf("focusCopies=%v main selected=%d, want false/0", app.model.DedupView.FocusCopies, app.model.DedupView.Main.Selected)
	}
}

func TestMouseDedupBrowsePanelHoverAndClick(t *testing.T) {
	app := dedupMouseTestApp(t)
	dedupBrowseSettle(t, app, app.screen.(tcell.SimulationScreen))
	app.render()
	if app.model.DedupPanel.VisibleEntryCount() < 2 {
		t.Fatalf("browse panel has %d entries, want at least 2", app.model.DedupPanel.VisibleEntryCount())
	}
	w, h := app.screen.Size()
	_, rect := ui.DedupSecondaryRects(app.layoutForTerminalSize(w, h).Secondary)
	want := 0
	if app.model.DedupPanel.Cursor == 0 {
		want = 1
	}
	x, y := rect.X+3, ui.PanelListTop(rect)+want
	app.handleMouse(mouseEv(x, y, tcell.ButtonNone))
	if got := app.model.PanelRowHover; !got.Active || got.Panel != ui.DedupBrowsePanel || got.Entry != want {
		t.Fatalf("hover = %+v, want browse entry %d", got, want)
	}
	if app.model.DedupPanel.Cursor == want || app.model.DedupView.FocusPanel {
		t.Fatal("hover must not move the cursor or focus")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.render()
	if app.model.PanelRowHover.Active {
		t.Fatal("a key press must hide the row hover")
	}
	click(app, x, y, tcell.Button1)
	if !app.model.DedupView.FocusPanel || app.model.DedupPanel.Cursor != want {
		t.Fatalf("focusPanel=%v cursor=%d, want true/%d", app.model.DedupView.FocusPanel, app.model.DedupPanel.Cursor, want)
	}
}

// fullscreenTextPreviewApp opens a loaded fullscreen preview of guttered lines.
func fullscreenTextPreviewApp(t *testing.T, lines int) *App {
	t.Helper()
	prevWriter := clipboard.OSWriter
	clipboard.OSWriter = func(string) error { return nil }
	t.Cleanup(func() { clipboard.OSWriter = prevWriter; clipboard.Reset() })
	app, dir := mouseTestApp(t)
	app.model.ViewMode = ui.ViewFilePreview
	st := ui.FilePreviewState{
		Open: true, Phase: ui.FilePreviewPhaseDone, Path: filepath.Join(dir, "walnut.txt"),
		Source: ui.PreviewSourceInternalHighlighted, GutterWidth: 4,
	}
	var cells []ui.AnsiCell
	for i := 1; i <= lines; i++ {
		for _, r := range fmt.Sprintf("%3d harbor%d\n", i, i) {
			cells = append(cells, ui.AnsiCell{R: r})
		}
	}
	st.SetHighlightedCells(cells)
	app.model.FullscreenFilePreview = st
	app.render()
	return app
}

func TestMousePreviewDragCopiesTextWithoutGutter(t *testing.T) {
	app := fullscreenTextPreviewApp(t, 3)
	// Row 0 is the filename; body row 1 is line 1, whose text starts after the 4-cell gutter.
	app.handleMouse(mouseEv(4, 1, tcell.Button1))
	app.handleMouse(mouseEv(6, 2, tcell.Button1))
	if got := clipboard.LastSet(); got != "" {
		t.Fatalf("copied %q before the release", got)
	}
	app.handleMouse(mouseEv(6, 2, tcell.ButtonNone))
	if got, want := clipboard.LastSet(), "harbor1\nhar"; got != want {
		t.Fatalf("clipboard = %q, want %q", got, want)
	}
	if got := app.model.MessageLog[0]; !strings.Contains(got.Text, "Copied: harbor1 har") {
		t.Fatalf("toast = %q", got.Text)
	}
	if app.model.FullscreenFilePreview.Selection.Dragging {
		t.Fatal("release must end the drag")
	}
}

func TestMousePreviewClickAndNonTextPressesCopyNothing(t *testing.T) {
	app := fullscreenTextPreviewApp(t, 3)
	w, _ := app.screen.Size()
	for _, p := range []struct{ x, y int }{{6, 1}, {6, 0}, {w - 1, 1}} { // text, filename row, scrollbar column
		app.handleMouse(mouseEv(p.x, p.y, tcell.Button1))
		app.handleMouse(mouseEv(p.x+1, p.y, tcell.ButtonNone))
		if got := clipboard.LastSet(); got != "" {
			t.Fatalf("press at %v copied %q", p, got)
		}
	}
}

func TestMousePreviewDragPastBottomEdgeScrolls(t *testing.T) {
	app := fullscreenTextPreviewApp(t, 60)
	_, h := app.screen.Size()
	app.handleMouse(mouseEv(4, 1, tcell.Button1))
	app.handleMouse(mouseEv(4, h-1, tcell.Button1))
	app.handleMouse(mouseEv(5, h-1, tcell.Button1))
	if got := app.model.FullscreenFilePreview.Scroll; got != 2 {
		t.Fatalf("scroll = %d, want one row per report past the edge", got)
	}
	app.handleMouse(mouseEv(5, h-1, tcell.ButtonNone))
	if got := clipboard.LastSet(); !strings.HasPrefix(got, "harbor1\n") {
		t.Fatalf("clipboard = %q", got)
	}
}

func TestMousePreviewDoubleClickCopiesWord(t *testing.T) {
	app := fullscreenTextPreviewApp(t, 3)
	app.handleMouse(mouseEv(6, 2, tcell.Button1))
	app.handleMouse(mouseEv(6, 2, tcell.ButtonNone))
	if got := clipboard.LastSet(); got != "" {
		t.Fatalf("first click copied %q", got)
	}
	app.handleMouse(mouseEv(6, 2, tcell.Button1))
	if got := clipboard.LastSet(); got != "harbor2" {
		t.Fatalf("clipboard = %q", got)
	}
	if got := app.model.MessageLog[0].Text; !strings.Contains(got, "Copied: harbor2") {
		t.Fatalf("toast = %q", got)
	}
	// Motion before the release must not shrink the word.
	app.handleMouse(mouseEv(7, 2, tcell.Button1))
	app.handleMouse(mouseEv(7, 2, tcell.ButtonNone))
	st := app.model.FullscreenFilePreview
	if got := st.SelectedText(); got != "harbor2" {
		t.Fatalf("selection = %q", got)
	}
}

func TestCopyPreviewSelectionToast(t *testing.T) {
	short := strings.Repeat("x", config.MouseCopyToastMaxRunes-1)
	long := strings.Repeat("x", config.MouseCopyToastMaxRunes)
	for _, c := range []struct{ text, want string }{
		{"lantern\n  meadow", "Copied: lantern meadow"},
		{short, "Copied: " + short},
		{long, "Copied 1 line"},
		{long + "\n" + long + "\n", "Copied 2 lines"},
		{"\n\n", "Copied 2 lines"},
	} {
		app := fullscreenTextPreviewApp(t, 1)
		app.copyPreviewSelection(c.text)
		if got := app.model.MessageLog[0].Text; !strings.Contains(got, c.want) {
			t.Errorf("toast for %q = %q, want %q", c.text, got, c.want)
		}
		if got := clipboard.LastSet(); got != c.text {
			t.Errorf("clipboard = %q, want %q", got, c.text)
		}
	}
}

// mouseScreenText is everything painted on the app's simulation screen.
func mouseScreenText(t *testing.T, a *App) string {
	t.Helper()
	screen, ok := a.screen.(tcell.SimulationScreen)
	if !ok {
		t.Fatal("expected simulation screen")
	}
	w, h := screen.Size()
	return tcelltest.DumpRect(screen, 0, 0, w, h)
}

// mouseQuickViewApp latches quick view on the primary panel with its cursor on name.
func mouseQuickViewApp(t *testing.T, name string) (a *App, x, y int) {
	t.Helper()
	a, _ = mouseTestApp(t)
	a.config.UI.KeyRepeatDebounceMS = 0
	a.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, a.panelByID(ui.PrimaryPanel), name)
	a.model.QuickViewEnabled = true
	a.model.QuickViewPanel = ui.PrimaryPanel
	w, h := a.screen.Size()
	col := a.layoutForTerminalSize(w, h).Secondary
	return a, col.X + col.Width/2, col.Y + col.Height/2
}

func TestMouseWheelScrollsQuickViewFilePreview(t *testing.T) {
	app, x, y := mouseQuickViewApp(t, "walnut.txt")
	var body strings.Builder
	for i := range 200 {
		fmt.Fprintf(&body, "row-%03d\n", i)
	}
	if err := os.WriteFile(filepath.Join(app.panelByID(ui.PrimaryPanel).Path.String(), "walnut.txt"), []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	app.previewCtrl.ApplyQuickViewPreviewImmediately()
	screen, ok := app.screen.(tcell.SimulationScreen)
	if !ok {
		t.Fatal("expected simulation screen")
	}
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		app.commandsMu.RLock()
		defer app.commandsMu.RUnlock()
		return app.model.FilePreview.Open && app.model.FilePreview.Phase == ui.FilePreviewPhaseDone
	})
	app.render()
	cursor := app.model.Primary.Cursor
	shown := func(needle string) bool {
		return strings.Contains(mouseScreenText(t, app), needle)
	}
	if !shown("row-000") {
		t.Fatal("first row not painted before the wheel")
	}

	_, rendered := app.handleMouse(mouseEv(x, y, tcell.WheelDown))
	if !rendered || app.model.FilePreview.Scroll != config.MouseWheelRows {
		t.Fatalf("rendered=%v scroll=%d want true/%d", rendered, app.model.FilePreview.Scroll, config.MouseWheelRows)
	}
	if shown("row-000") {
		t.Fatal("wheel did not repaint the preview")
	}
	if app.model.Primary.Cursor != cursor || app.model.ActivePanel != ui.PrimaryPanel {
		t.Fatalf("cursor=%d active=%d, want driver untouched", app.model.Primary.Cursor, app.model.ActivePanel)
	}

	app.handleMouse(mouseEv(x, y, tcell.WheelUp))
	app.handleMouse(mouseEv(x, y, tcell.WheelUp))
	if app.model.FilePreview.Scroll != 0 {
		t.Fatalf("scroll=%d want clamped to 0", app.model.FilePreview.Scroll)
	}
}

func TestMouseWheelMovesQuickViewDirOverlay(t *testing.T) {
	app, x, y := mouseQuickViewApp(t, "aaa-kettle")
	for _, n := range []string{"amber", "bronze", "copper", "drift", "ember", "flint", "grove", "hazel"} {
		writeFile(t, filepath.Join(app.panelByID(ui.PrimaryPanel).Path.String(), "aaa-kettle", n+".txt"))
	}
	app.previewCtrl.ApplyQuickViewPreviewImmediately()
	if !app.model.QuickViewDirOverlayActive {
		t.Fatal("expected directory overlay")
	}
	screen, ok := app.screen.(tcell.SimulationScreen)
	if !ok {
		t.Fatal("expected simulation screen")
	}
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		return !app.model.QuickViewDirOverlay.ListingPending
	})
	ov := &app.model.QuickViewDirOverlay
	ov.Cursor = 0
	driver := app.model.Primary.Cursor

	if _, rendered := app.handleMouse(mouseEv(x, y, tcell.WheelDown)); !rendered {
		t.Fatal("wheel over the overlay did not render")
	}
	if ov.Cursor != config.MouseWheelRows {
		t.Fatalf("overlay cursor=%d want %d", ov.Cursor, config.MouseWheelRows)
	}
	if app.model.Primary.Cursor != driver || !app.model.QuickViewDirOverlayActive {
		t.Fatalf("driver cursor=%d want %d, overlay active=%v", app.model.Primary.Cursor, driver, app.model.QuickViewDirOverlayActive)
	}
}

func TestMouseWheelIgnoresPendingQuickViewPreview(t *testing.T) {
	app, x, y := mouseQuickViewApp(t, "walnut.txt")
	cursor := app.model.Primary.Cursor
	app.commandsMu.Lock()
	app.model.FilePreview.Open = true
	app.model.FilePreview.Phase = ui.FilePreviewPhasePending
	app.model.FilePreview.Scroll = 7
	app.commandsMu.Unlock()
	if _, rendered := app.handleMouse(mouseEv(x, y, tcell.WheelDown)); rendered {
		t.Fatal("wheel over a pending preview rendered")
	}
	if app.model.FilePreview.Scroll != 7 || app.model.Primary.Cursor != cursor {
		t.Fatalf("scroll=%d cursor=%d want 7/%d", app.model.FilePreview.Scroll, app.model.Primary.Cursor, cursor)
	}
}
