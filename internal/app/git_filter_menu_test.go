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
	if !app.leaderMenuOpen(ui.LeaderMenuGitFilter) {
		t.Fatalf("LeaderMenu = %+v, want git filter menu open", app.model.LeaderMenu)
	}
}

func TestGitMenuActionsGroup(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.toggleGitFilterMenu()
	st := app.model.LeaderMenu
	if !app.leaderMenuOpen(ui.LeaderMenuGitFilter) {
		t.Fatal("expected git menu open")
	}
	want := map[rune]string{'S': keymap.ActionGitStage, 'U': keymap.ActionGitUnstage, 'R': keymap.ActionGitUntrack}
	for key, action := range want {
		i, ok := ui.LeaderMenuIndexForKey(st.Items, key)
		if !ok || i >= len(app.leaderMenuActions) || app.leaderMenuActions[i] != action {
			t.Fatalf("key %q -> row %d (ok=%v), want action %q", key, i, ok, action)
		}
	}
	// Lowercase letters still pick the filters, not the actions.
	if i, ok := ui.LeaderMenuIndexForKey(st.Items, 's'); !ok || app.leaderMenuActions[i] != "" {
		t.Fatalf("key 's' -> row %d (ok=%v), want the Staged filter row", i, ok)
	}
}

func TestAltGTogglesGitFilterMenuClosed(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser

	app.toggleGitFilterMenu()
	if !app.leaderMenuOpen(ui.LeaderMenuGitFilter) {
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

func TestFolderColorKeyTogglesMenuClosed(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	items := []ui.LeaderMenuItem{{Label: "Amber"}, {Label: "Default"}}
	app.openLeaderMenuStrip(items, ui.LeaderMenuFolderColor, "Folder color", func(int) bool { return false })
	if !app.leaderMenuOpen(ui.LeaderMenuFolderColor) || app.leaderMenuOpen(ui.LeaderMenuGitFilter) {
		t.Fatalf("LeaderMenu = %+v, want folder color menu open", app.model.LeaderMenu)
	}

	altC := tcell.NewEventKey(tcell.KeyRune, 'C', tcell.ModAlt)
	if id := app.actionFromKeyEvent(altC); id != keymap.ActionPanelFolderColorMenu {
		t.Fatalf("Alt-Shift-C resolves to %q, want %q", id, keymap.ActionPanelFolderColorMenu)
	}
	if _, rendered := app.handleKey(altC); !rendered {
		t.Fatal("second Alt-Shift-C should render after closing the folder color menu")
	}
	if app.model.LeaderMenu.Open {
		t.Fatal("second Alt-Shift-C should close the folder color menu")
	}
}
