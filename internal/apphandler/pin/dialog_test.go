package pin

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// fakePinHost is a minimal Host stub for pin dialog tests that don't need a real *App.
type fakePinHost struct{}

func (fakePinHost) LayoutForTerminalSize(w, h int) ui.Layout      { return ui.Layout{Width: w, Height: h} }
func (fakePinHost) SetTransientMessage(string, ui.MessageUrgency) {}
func (fakePinHost) SetErrorMessage(string, error)                 {}
func (fakePinHost) NavigatePanelToPath(int, string, string) error { return nil }
func (fakePinHost) ActivePanel() *panel.State                     { return &panel.State{} }
func (fakePinHost) PanelByID(int) *panel.State                    { return &panel.State{} }
func (fakePinHost) PanelViewportRows(int) int                     { return 20 }
func (fakePinHost) InQuickFilterUI() bool                         { return false }
func (fakePinHost) CancelActiveQuickFilter()                      {}
func (fakePinHost) Config() config.Config                         { return config.Default() }
func (fakePinHost) SyncFilteredListRanks(lines []string, _ string, _ int, _ bool) ([]int, [][]search.Range) {
	ranked := make([]int, len(lines))
	for i := range lines {
		ranked[i] = i
	}
	return ranked, make([][]search.Range, len(lines))
}
func (fakePinHost) ClampFilteredListSelection(selected *int, rankedLen int) {
	if *selected < 0 || rankedLen == 0 {
		*selected = 0
	} else if *selected >= rankedLen {
		*selected = rankedLen - 1
	}
}
func (fakePinHost) HandleFilteredListSelectionKey(*tcell.EventKey, int, *int, int, func() int, func()) bool {
	return false
}

func newPinTestHandler(t *testing.T) *Handler {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(120, 40)
	return &Handler{
		host:   fakePinHost{},
		screen: screen,
		model:  &ui.Model{},
	}
}

func TestPinDialogMissingScanAppliesAndIgnoresStaleGen(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "otter")
	missing := filepath.Join(root, "narwhal")

	h := newPinTestHandler(t)
	h.model.PinnedItems = []ui.PinnedItem{
		{Path: existing, IsDir: true},
		{Path: missing, IsDir: true},
	}

	h.OpenDialog()
	if !h.model.PinDialog.Open {
		t.Fatal("expected pin dialog open")
	}
	for _, it := range h.model.PinnedItems {
		if it.PathMissing {
			t.Fatalf("item %+v: PathMissing true right after open, want false (scan is async)", it)
		}
	}

	staleGen := h.missingGen - 1

	// Stale gen: must be ignored even though it would flip the existing path missing.
	h.ApplyMissing(dialogctrl.PathsMissingPayload{
		Target:  "pin",
		Gen:     staleGen,
		Missing: map[string]bool{existing: true},
	})
	if h.model.PinnedItems[0].PathMissing {
		t.Fatal("stale-gen payload was applied, want ignored")
	}

	// Matching gen: the nonexistent path flips to missing, the existing one stays not-missing.
	h.ApplyMissing(dialogctrl.PathsMissingPayload{
		Target: "pin",
		Gen:    h.missingGen,
		Missing: map[string]bool{
			existing: false,
			missing:  true,
		},
	})
	if h.model.PinnedItems[0].PathMissing {
		t.Errorf("existing item PathMissing = true, want false")
	}
	if !h.model.PinnedItems[1].PathMissing {
		t.Errorf("missing item PathMissing = false, want true")
	}

	h.CloseDialog()
	h.ApplyMissing(dialogctrl.PathsMissingPayload{
		Target:  "pin",
		Gen:     h.missingGen, // even the current gen must be ignored once closed
		Missing: map[string]bool{existing: true},
	})
	if h.model.PinnedItems[0].PathMissing {
		t.Fatal("payload applied after dialog closed, want ignored")
	}
}
