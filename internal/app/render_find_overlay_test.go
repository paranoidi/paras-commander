package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestRenderFindDialogUpdateDoesNotCoverGroupSelect(t *testing.T) {
	app := newFindDialogTestApp(t)
	screen, ok := app.screen.(tcell.SimulationScreen)
	if !ok {
		t.Fatal("expected simulation screen")
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone)); quit {
		t.Fatal("F6 quit")
	}
	if !app.model.FindDialog.Open || !app.model.GroupSelect.Open {
		t.Fatal("want Find open under Group Select")
	}
	if app.inputMode() != InputModeGroupSelect {
		t.Fatalf("input mode = %d, want Group Select as the top layer", app.inputMode())
	}

	app.render()
	if !screenHasText(screen, "Select group") {
		t.Fatal("full render should show the Group Select title")
	}
	want := ui.HashScreenLogical(app.screen)

	// Rank / disk-size wakes both go through renderFindDialogUpdate.
	app.renderFindDialogUpdate()
	if !screenHasText(screen, "Select group") {
		t.Fatal("find-rank or disk-size wake must not paint Find over Group Select")
	}
	if got := ui.HashScreenLogical(app.screen); got != want {
		t.Fatal("find wake while Group Select is open must pixel-match a full render")
	}

	if app.paintFindDialogOverlay() {
		t.Fatal("Find-only paint must decline while Find is not the top input layer")
	}

	app.closeGroupSelect()
	if !app.paintFindDialogOverlay() {
		t.Fatal("Find-only paint should succeed when Find is the top input layer")
	}
}

func screenHasText(screen tcell.SimulationScreen, want string) bool {
	w, h := screen.Size()
	for y := 0; y < h; y++ {
		if strings.Contains(screenLine(screen, y, w), want) {
			return true
		}
	}
	return false
}
