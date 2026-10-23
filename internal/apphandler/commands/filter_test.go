package commands

import (
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type filterHost struct{ Host }

func (filterHost) LayoutForTerminalSize(w, h int) ui.Layout {
	return ui.Layout{Width: w, Height: h, Primary: ui.Rect{Width: w, Height: h}}
}
func (filterHost) FilterOptions() quickfilter.Options {
	return quickfilter.Options{CaseInsensitive: true}
}
func (filterHost) ActionFromKeyEvent(*tcell.EventKey) string { return "" }
func (filterHost) DispatchLeaderLetter(*tcell.EventKey) bool { return false }

func TestCommandsQuickFilterJumpsToMacroTarget(t *testing.T) {
	h, _ := newOutputDialogHandler(t)
	h.host, h.mu = filterHost{}, &sync.RWMutex{}
	h.model.UserHomeDir = "/home/walnut"
	for _, f := range []string{"/home/walnut/meadow/ribbon.txt", "/home/walnut/harbor/lantern.txt", "/tmp/orchard.txt"} {
		h.AppendEntry(ui.CommandRunEntry{
			UserCommandLine: "tool %f",
			Macros:          []cmdmacro.MacroUse{{Letter: 'f', Values: []string{f}}},
		})
	}
	for _, r := range "lant" {
		h.HandleViewKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if got := h.model.CommandsView.Selected; got != 1 {
		t.Fatalf("Selected = %d, want 1", got)
	}
	if !h.Filter().UIActive() {
		t.Fatal("filter not open")
	}
	if len(h.Filter().Ranges(1)) == 0 {
		t.Fatal("no highlight ranges on the matched row")
	}
	h.CancelFilter()
	if h.Filter().UIActive() {
		t.Fatal("filter still open after CancelFilter")
	}
	if h.model.CommandsView.Selected != 1 {
		t.Fatal("cancel moved the cursor")
	}
}

// openNewestFirst appends n runs with distinct commands and opens the view.
func openNewestFirst(t *testing.T, n int) *Handler {
	t.Helper()
	h, _ := newOutputDialogHandler(t)
	h.host, h.mu = filterHost{}, &sync.RWMutex{}
	for _, w := range []string{"meadow", "lantern", "orchard", "ribbon", "walnut"}[:n] {
		h.AppendEntry(ui.CommandRunEntry{UserCommandLine: w})
	}
	h.model.CommandsView.Selected = n - 1
	return h
}

func TestCommandsViewNavigationIsVisual(t *testing.T) {
	h := openNewestFirst(t, 5)
	sel := func() int { return h.model.CommandsView.Selected }
	key := func(k tcell.Key) { h.HandleViewKey(tcell.NewEventKey(k, 0, tcell.ModNone)) }

	key(tcell.KeyUp)
	if sel() != 4 {
		t.Fatalf("Up at newest: Selected = %d, want 4 (clamped)", sel())
	}
	key(tcell.KeyDown)
	if sel() != 3 {
		t.Fatalf("Down: Selected = %d, want 3 (older)", sel())
	}
	key(tcell.KeyUp)
	if sel() != 4 {
		t.Fatalf("Up: Selected = %d, want 4 (newer)", sel())
	}
	key(tcell.KeyEnd)
	if sel() != 0 {
		t.Fatalf("End: Selected = %d, want 0 (bottom of screen = oldest)", sel())
	}
	key(tcell.KeyHome)
	if sel() != 4 {
		t.Fatalf("Home: Selected = %d, want 4 (top of screen = newest)", sel())
	}
	key(tcell.KeyPgDn)
	if sel() != 0 {
		t.Fatalf("PgDn: Selected = %d, want 0 (clamped)", sel())
	}
}

func TestCommandsViewOpenSelectsNewestAndKeepsSelectionOnAppend(t *testing.T) {
	h := openNewestFirst(t, 3)
	h.model.CommandsView.Selected = 0
	h.OpenView()
	if got := h.model.CommandsView.Selected; got != 2 {
		t.Fatalf("OpenView Selected = %d, want 2 (newest)", got)
	}
	h.MoveSelection(1) // one row down: older
	h.AppendEntry(ui.CommandRunEntry{UserCommandLine: "ribbon"})
	if got := h.model.CommandsView.Selected; got != 1 {
		t.Fatalf("Selected after append = %d, want 1 (same run)", got)
	}
}

func TestCommandsFilterCycleMovesDownTheScreen(t *testing.T) {
	h := openNewestFirst(t, 5)
	h.model.CommandsList[0].UserCommandLine = "alpha"
	h.model.CommandsList[2].UserCommandLine = "alpha"
	h.model.CommandsList[4].UserCommandLine = "alpha"
	for _, r := range "alpha" {
		h.HandleViewKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	h.model.CommandsView.Selected = 4
	h.CycleFilterMatch(1)
	if got := h.model.CommandsView.Selected; got != 2 {
		t.Fatalf("next match = %d, want 2 (down the screen)", got)
	}
	h.CycleFilterMatch(-1)
	if got := h.model.CommandsView.Selected; got != 4 {
		t.Fatalf("previous match = %d, want 4", got)
	}
}
