package uitest

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Screen returns an initialized simulation screen sized to w×h; Fini runs on cleanup.
func Screen(t *testing.T, w, h int) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init(): %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(w, h)
	return screen
}

// Theme returns the built-in default theme.
func Theme(t *testing.T) theme.Theme {
	t.Helper()
	return theme.Default()
}

// ModelOption adjusts MinimalBrowserModel.
type ModelOption func(*ui.Model)

// WithLeftPath sets the left panel path (must be an existing directory).
func WithLeftPath(path string) ModelOption {
	return func(m *ui.Model) {
		m.Left.Path = pathloc.MustParse(path)
	}
}

// WithRightPath sets the right panel path (must be an existing directory).
func WithRightPath(path string) ModelOption {
	return func(m *ui.Model) {
		m.Right.Path = pathloc.MustParse(path)
	}
}

// WithActivePanel sets ActivePanel (ui.LeftPanel or ui.RightPanel).
func WithActivePanel(id int) ModelOption {
	return func(m *ui.Model) {
		m.ActivePanel = id
	}
}

// MinimalBrowserModel returns a renderable twin-panel browser model rooted at a temp directory.
func MinimalBrowserModel(t *testing.T, opts ...ModelOption) ui.Model {
	t.Helper()
	root := t.TempDir()
	left, err := panel.New(root)
	if err != nil {
		t.Fatalf("panel.New(%q): %v", root, err)
	}
	right, err := panel.New(root)
	if err != nil {
		t.Fatalf("panel.New(%q): %v", root, err)
	}
	m := ui.Model{
		Left:        left,
		Right:       right,
		ActivePanel: ui.LeftPanel,
		ViewMode:    ui.ViewBrowser,
		UserHomeDir: root,
	}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}
