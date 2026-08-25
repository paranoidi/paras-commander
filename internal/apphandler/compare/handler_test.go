package compare

import (
	"fmt"
	"sync/atomic"
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

type compareHandlerHost struct{}

func (compareHandlerHost) NavigatePanelToPath(int, string, string) error { return nil }
func (compareHandlerHost) TogglePanelSelection(int, string) bool         { return false }
func (compareHandlerHost) SetTransientMessage(string, ui.MessageUrgency) {}
func (compareHandlerHost) ClearTransientMessage()                        {}
func (compareHandlerHost) CompareMenuDefinitions() []menu.Definition     { return nil }
func (compareHandlerHost) BrowserMenuDefinitions() []menu.Definition     { return nil }

func TestMoveColumnFocusMapsLeftRightToPrimarySecondary(t *testing.T) {
	model := &ui.Model{
		CompareView: ui.CompareViewState{FocusColumn: ui.CompareColumnSecondary},
		CompareSnapshot: comparepkg.Snapshot{
			Rows: []comparepkg.Row{
				{PrimaryRel: "a.txt", SecondaryRel: "b.txt", Kind: comparepkg.KindContentDiff, HashDone: true},
			},
		},
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})

	h.MoveColumnFocus(-1)
	if model.CompareView.FocusColumn != ui.CompareColumnPrimary {
		t.Fatalf("left: FocusColumn = %v, want primary", model.CompareView.FocusColumn)
	}

	h.MoveColumnFocus(1)
	if model.CompareView.FocusColumn != ui.CompareColumnSecondary {
		t.Fatalf("right: FocusColumn = %v, want secondary", model.CompareView.FocusColumn)
	}
}

func TestOpenSetsPrimaryColumnFocus(t *testing.T) {
	model := &ui.Model{
		Primary:   panelStateAt(pathloc.MustParse("/a")),
		Secondary: panelStateAt(pathloc.MustParse("/b")),
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})
	h.Open()
	defer h.Close()

	if model.CompareView.FocusColumn != ui.CompareColumnPrimary {
		t.Fatalf("open FocusColumn = %v, want primary", model.CompareView.FocusColumn)
	}
	if !model.CompareView.IgnoreEmpty {
		t.Fatal("open IgnoreEmpty = false, want true")
	}
}

func panelStateAt(p pathloc.Path) panel.State {
	s := panel.State{}
	s.Path = p
	return s
}

func TestCloseFiresOnCloseOnce(t *testing.T) {
	model := &ui.Model{
		Primary:   panelStateAt(pathloc.MustParse("/alpha")),
		Secondary: panelStateAt(pathloc.MustParse("/beta")),
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})
	var calls atomic.Int32
	if !h.OpenPaths(pathloc.MustParse("/alpha"), pathloc.MustParse("/beta"), false, func() {
		calls.Add(1)
	}) {
		t.Fatal("OpenPaths failed")
	}
	h.Close()
	h.Close()
	if calls.Load() != 1 {
		t.Fatalf("onClose calls = %d, want 1", calls.Load())
	}
	if model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want browser", model.ViewMode)
	}
}

func TestRefreshDoesNotFireOnClose(t *testing.T) {
	model := &ui.Model{
		Primary:   panelStateAt(pathloc.MustParse("/alpha")),
		Secondary: panelStateAt(pathloc.MustParse("/beta")),
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})
	var calls atomic.Int32
	if !h.OpenPaths(pathloc.MustParse("/alpha"), pathloc.MustParse("/beta"), false, func() {
		calls.Add(1)
	}) {
		t.Fatal("OpenPaths failed")
	}
	h.Refresh()
	if calls.Load() != 0 {
		t.Fatalf("Refresh fired onClose %d times, want 0", calls.Load())
	}
	if model.ViewMode != ui.ViewCompare {
		t.Fatalf("ViewMode = %v, want compare", model.ViewMode)
	}
	h.Close()
	if calls.Load() != 1 {
		t.Fatalf("Close onClose calls = %d, want 1", calls.Load())
	}
}

func TestOpenPathsOverlapLeavesViewUntouched(t *testing.T) {
	model := &ui.Model{
		ViewMode: ui.ViewDedup,
		DedupView: ui.DedupViewState{
			Marked: map[string]bool{"x": true},
		},
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})
	parent := pathloc.MustParse("/warehouse")
	child := pathloc.MustParse("/warehouse/cellar")
	if h.OpenPaths(parent, child, false, func() { t.Fatal("onClose should not run") }) {
		t.Fatal("OpenPaths on overlapping trees: ok = true, want false")
	}
	if model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v, want dedup", model.ViewMode)
	}
	if !model.DedupView.Marked["x"] {
		t.Fatal("dedup view state was modified")
	}
}

func TestDiscardReturnSkipsOnClose(t *testing.T) {
	model := &ui.Model{}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})
	if !h.OpenPaths(pathloc.MustParse("/alpha"), pathloc.MustParse("/beta"), false, func() {
		t.Fatal("onClose should not run")
	}) {
		t.Fatal("OpenPaths failed")
	}
	h.DiscardReturn()
	h.Close()
	if model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want browser", model.ViewMode)
	}
}

func TestEnsureSelectionVisibleScrollsWhenCursorPastViewport(t *testing.T) {
	const visible = 5
	rows := make([]comparepkg.Row, 12)
	for i := range rows {
		name := fmt.Sprintf("row-%02d.txt", i)
		rows[i] = comparepkg.Row{PrimaryRel: name, SecondaryRel: name, Kind: comparepkg.KindEqual, HashDone: true}
	}
	model := &ui.Model{
		CompareView:     ui.CompareViewState{Selected: 0, ListScroll: 0},
		CompareSnapshot: comparepkg.Snapshot{Rows: rows, Phase: comparepkg.PhaseDone},
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})

	// Cursor on last painted row (index visible-1) must not scroll yet.
	model.CompareView.Selected = visible - 1
	h.EnsureSelectionVisible(visible)
	if model.CompareView.ListScroll != 0 {
		t.Fatalf("at bottom of first page: ListScroll = %d, want 0", model.CompareView.ListScroll)
	}

	// One more Down must scroll so the selection stays in view.
	model.CompareView.Selected = visible
	h.EnsureSelectionVisible(visible)
	if model.CompareView.ListScroll != 1 {
		t.Fatalf("past viewport: ListScroll = %d, want 1", model.CompareView.ListScroll)
	}
	if model.CompareView.Selected < model.CompareView.ListScroll ||
		model.CompareView.Selected >= model.CompareView.ListScroll+visible {
		t.Fatalf("selected %d outside [%d, %d)", model.CompareView.Selected,
			model.CompareView.ListScroll, model.CompareView.ListScroll+visible)
	}
}

func TestEnsureSelectionVisibleClampsMaxScroll(t *testing.T) {
	rows := make([]comparepkg.Row, 10)
	for i := range rows {
		rows[i] = comparepkg.Row{PrimaryRel: "f.txt", Kind: comparepkg.KindPrimaryOnly, HashDone: true}
	}
	model := &ui.Model{
		CompareView:     ui.CompareViewState{Selected: 9, ListScroll: 100},
		CompareSnapshot: comparepkg.Snapshot{Rows: rows, Phase: comparepkg.PhaseDone},
	}
	h := New(Deps{Host: compareHandlerHost{}, Model: model})
	h.EnsureSelectionVisible(4)
	if model.CompareView.ListScroll != 6 {
		t.Fatalf("ListScroll = %d, want 6 (max 10-4)", model.CompareView.ListScroll)
	}
}
