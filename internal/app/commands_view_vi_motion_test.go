package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func openCommandsViewWithEntries(t *testing.T, app *App, n int) {
	t.Helper()
	var last int
	for i := 0; i < n; i++ {
		last = app.commandsCtrl.AppendEntry(ui.CommandRunEntry{Kind: ui.CommandRunKindUserMenu, UserCommandLine: "true"})
	}
	app.commandsCtrl.OpenViewAt(last)
	if app.model.ViewMode != ui.ViewCommands {
		t.Fatalf("ViewMode = %v, want ViewCommands", app.model.ViewMode)
	}
}

// TestCommandsViewViMotionHJKLOnlyWhenModeOn verifies 'j'/'k' move the commands-list selection
// like Down/Up only while vi-motion mode is on, mirroring the browser's own hjkl gating.
func TestCommandsViewViMotionHJKLOnlyWhenModeOn(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	openCommandsViewWithEntries(t, app, 2)
	app.model.CommandsView.Selected = 1

	app.commandsCtrl.HandleViewKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	if app.model.CommandsView.Selected != 1 {
		t.Fatalf("vi-motion off: 'k' moved selection to %d, want unchanged 1", app.model.CommandsView.Selected)
	}

	app.model.ViMotionMode = true
	app.commandsCtrl.HandleViewKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	if app.model.CommandsView.Selected != 0 {
		t.Fatalf("vi-motion on: 'k' selection = %d, want 0", app.model.CommandsView.Selected)
	}
}

// TestCommandsViewViMotionKMovesSelectionNotKill verifies that with vi-motion mode on, a bare
// 'k' moves the selection (nav) rather than firing commands.kill, even though 'k' is Kill's
// leader-menu letter: RemapViMotionKey consumes h/j/k/l before the leader-letter lookup ever
// sees them, so hjkl always wins over a letter-mnemonic collision (matching the browser's own
// precedence). Kill stays reachable via the `:` menu, F9 menu, and its S-F8 chord.
func TestCommandsViewViMotionKMovesSelectionNotKill(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	openCommandsViewWithEntries(t, app, 2)
	app.model.CommandsView.Selected = 1
	app.model.CommandsList[1].Phase = ui.CommandRunRunning

	app.model.ViMotionMode = true
	app.model.Message = ""
	app.commandsCtrl.HandleViewKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))

	if app.model.CommandsView.Selected != 0 {
		t.Fatalf("'k' with vi-motion on: selection = %d, want 0 (nav, not kill)", app.model.CommandsView.Selected)
	}
	if app.model.CommandsList[1].Phase != ui.CommandRunRunning {
		t.Fatalf("'k' with vi-motion on must not kill the running command, Phase = %v", app.model.CommandsList[1].Phase)
	}
}

// TestCommandsViewViMotionLeaderLetterOnlyWhenModeOn verifies a bound leader-menu letter (here
// 'x', commands.close) fires its action directly only while vi-motion mode is on.
func TestCommandsViewViMotionLeaderLetterOnlyWhenModeOn(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	openCommandsViewWithEntries(t, app, 1)

	app.commandsCtrl.HandleViewKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if app.model.ViewMode != ui.ViewCommands {
		t.Fatal("vi-motion off: 'x' must not close the commands view")
	}

	app.model.ViMotionMode = true
	app.commandsCtrl.HandleViewKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("vi-motion on: 'x' should dispatch commands.close directly, ViewMode = %v", app.model.ViewMode)
	}
}

// TestCommandsViewQuickFilterThroughHandleKey drives the Commands list filter through the
// full key path: every typed rune must reach the Commands filter (not the panel filter), Esc
// closes only the filter, and a second Esc closes the view.
func TestCommandsViewQuickFilterThroughHandleKey(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.commandsCtrl.AppendEntry(ui.CommandRunEntry{Kind: ui.CommandRunKindUserMenu, UserCommandLine: "lantern.sh"})
	last := app.commandsCtrl.AppendEntry(ui.CommandRunEntry{Kind: ui.CommandRunKindUserMenu, UserCommandLine: "orchard.sh"})
	app.commandsCtrl.OpenViewAt(last)
	app.model.CommandsView.Selected = last

	for _, r := range "lant" {
		_, _ = app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if q := app.model.CommandsView.Filter.Query; q != "lant" {
		t.Fatalf("filter query = %q, want %q", q, "lant")
	}
	if app.model.CommandsView.Selected != 0 {
		t.Fatalf("Selected = %d, want 0 (lantern.sh)", app.model.CommandsView.Selected)
	}

	_, _ = app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.CommandsView.Filter.UIActive() || app.model.ViewMode != ui.ViewCommands {
		t.Fatalf("first Esc: filter active=%v view=%v, want filter closed and view open",
			app.model.CommandsView.Filter.UIActive(), app.model.ViewMode)
	}
	_, _ = app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.ViewMode == ui.ViewCommands {
		t.Fatal("second Esc: Commands view still open")
	}
}
