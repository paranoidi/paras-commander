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
