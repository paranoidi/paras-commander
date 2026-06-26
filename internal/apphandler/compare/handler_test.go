package compare

import (
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
}

func panelStateAt(p pathloc.Path) panel.State {
	s := panel.State{}
	s.Path = p
	return s
}
