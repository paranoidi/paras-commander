package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func TestAuxiliaryScreensOpenFromJobsView(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.jobsCtrl.OpenJobsView()
	if !app.tryDispatchAuxiliaryScreens(keymap.ActionCommandsOpen) {
		t.Fatal("commands.open should be consumed from jobs view")
	}
	if app.model.ViewMode != ui.ViewCommands {
		t.Fatalf("ViewMode = %v, want ViewCommands", app.model.ViewMode)
	}
}

func TestAuxiliaryScreensOpenFromCommandsViewViaKey(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.commandsCtrl.OpenView()
	ev := tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModAlt)
	if app.actionFromKeyEvent(ev) != keymap.ActionJobsOpen {
		t.Fatalf("Alt+J in commands view = %q, want jobs.open", app.actionFromKeyEvent(ev))
	}
	_, rendered := app.handleKey(ev)
	if !rendered {
		t.Fatal("handleKey should render")
	}
	if app.model.ViewMode != ui.ViewJobs {
		t.Fatalf("ViewMode = %v, want ViewJobs", app.model.ViewMode)
	}
}

func TestJobsViewMenuIncludesDisplay(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.jobsCtrl.OpenJobsView()
	var hasDisplay bool
	for _, def := range menu.ActiveDefinitions(app.model.MenuDefinitions) {
		if def.ID == menu.TopDisplay {
			hasDisplay = true
			break
		}
	}
	if !hasDisplay {
		t.Fatal("jobs view menu bar missing Display")
	}
}
