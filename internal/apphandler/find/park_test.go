package find

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Parking hides Find with its state kept; restoring shows it as it
// was, the panel reports the parked find meanwhile.
func TestFindParkKeepsStateAndReopens(t *testing.T) {
	t.Parallel()
	model := editTestModel(1)
	model.FindDialog.Query = "lantern"
	h := newTestFindHandler(&fakeFindHost{}, model)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	h.screen = screen
	h.Park()
	if model.FindDialog.Open || !h.Hidden() || !h.live() || !model.FindParkedOn(ui.PrimaryPanel) {
		t.Fatalf("park: open=%v hidden=%v live=%v", model.FindDialog.Open, h.Hidden(), h.live())
	}
	h.RestoreParked()
	st := model.FindDialog
	if !st.Open || st.Parked || st.Query != "lantern" || st.Selected != 1 || len(st.Entries) != 2 {
		t.Fatalf("reopen lost state: %+v", st)
	}
}

func TestFindIsParkKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id   string
		jump bool
		want bool
	}{
		{keymap.ActionPanelFindDialog, false, true},
		{keymap.ActionFileIndexFind, false, true},
		{keymap.ActionPanelJumpDialog, false, false},
		{keymap.ActionPanelJumpDialog, true, true},
		{keymap.ActionPanelFindDialog, true, false},
	}
	for _, c := range cases {
		if got := IsParkKey(c.id, c.jump); got != c.want {
			t.Errorf("IsParkKey(%s, %v) = %v", c.id, c.jump, got)
		}
	}
}
