package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestAltGOpensGitFilterMenu(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser

	altG := tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModAlt)
	if id := app.actionFromKeyEvent(altG); id != keymap.ActionPanelGitFilterMenu {
		t.Fatalf("Alt-G resolves to %q, want %q", id, keymap.ActionPanelGitFilterMenu)
	}
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelGitFilterMenu)
	if !app.gitFilterMenuOpen() {
		t.Fatalf("LeaderMenu = %+v, want git filter menu open", app.model.LeaderMenu)
	}
}

func TestAltGTogglesGitFilterMenuClosed(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser

	app.toggleGitFilterMenu()
	if !app.gitFilterMenuOpen() {
		t.Fatal("expected git filter menu open")
	}

	altG := tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModAlt)
	_, rendered := app.handleKey(altG)
	if !rendered {
		t.Fatal("second Alt-G should render after closing the git filter menu")
	}
	if app.model.LeaderMenu.Open {
		t.Fatal("second Alt-G should close the git filter menu")
	}
}
