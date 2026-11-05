package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func marksTestPanel(dir string, names ...string) panel.State {
	s := panel.State{Path: pathloc.MustParse(dir)}
	for _, n := range names {
		s.Entries = append(s.Entries, localfs.Entry{Name: n, Path: dir + "/" + n, Type: localfs.EntryFile})
	}
	return s
}

func TestOtherPanelMarksFollowCursor(t *testing.T) {
	t.Parallel()
	hideAnchor := &panel.EntryFilter{ID: "no-anchor", Match: func(e localfs.Entry, _ *panel.State) bool { return e.Name != "anchor" }}
	filtered := marksTestPanel("/meadow", "anchor", "violin")
	filtered.SetEntryFilter(hideAnchor)
	tree := marksTestPanel("/harbor", "lantern", "anchor")
	tree.ListLayout = panel.ListLayoutTree

	tests := []struct {
		name      string
		marks     bool
		primary   panel.State
		secondary panel.State
		want      bool
	}{
		{"marks off", false, marksTestPanel("/harbor", "anchor"), marksTestPanel("/meadow", "anchor"), false},
		{"same directory", true, marksTestPanel("/harbor", "anchor"), marksTestPanel("/harbor", "anchor"), false},
		{"no common names", true, marksTestPanel("/harbor", "lantern"), marksTestPanel("/meadow", "violin"), false},
		{"common name", true, marksTestPanel("/harbor", "lantern", "anchor"), marksTestPanel("/meadow", "anchor"), true},
		{"filter hides common name", true, marksTestPanel("/harbor", "anchor"), filtered, false},
		{"moved panel in tree layout", true, tree, marksTestPanel("/meadow", "violin"), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := Model{ActivePanel: PrimaryPanel, OtherPanelMatchMarks: tc.marks, Primary: tc.primary, Secondary: tc.secondary}
			if got := otherPanelMarksFollowCursor(m, PrimaryPanel, 100); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// Only rows in the other panel's viewport count: a shared name scrolled out of view cannot gain
// or lose a mark.
func TestOtherPanelMarksFollowCursorViewport(t *testing.T) {
	t.Parallel()
	other := marksTestPanel("/meadow", "violin", "cedar", "willow", "ember", "anchor")
	m := Model{ActivePanel: PrimaryPanel, OtherPanelMatchMarks: true, Primary: marksTestPanel("/harbor", "anchor"), Secondary: other}
	const rows = 2
	if otherPanelMarksFollowCursor(m, PrimaryPanel, rows) {
		t.Fatal("shared name below the viewport must not count")
	}
	m.Secondary.ScrollOffset = 3
	if !otherPanelMarksFollowCursor(m, PrimaryPanel, rows) {
		t.Fatal("shared name inside the viewport must count")
	}
	m.Secondary.ScrollOffset = 5
	if otherPanelMarksFollowCursor(m, PrimaryPanel, rows) {
		t.Fatal("scroll offset past the last row must not count")
	}
}

// A cursor-only paint leaves the other panel alone unless a mark can follow the cursor; a listing
// paint repaints it whenever the marks are on.
func TestPaintBrowserCursorMovePanelOnlyScope(t *testing.T) {
	t.Parallel()
	const w, h = 80, 24
	layout := CalculateLayoutWithOrientation(w, h, true, PanelPaneSplit{
		ActivePanel: PrimaryPanel, ActivePercent: 50, InactivePercent: 50,
	}, SplitHorizontal, 0, 0)
	styles := theme.Default()
	sentinelX, sentinelY := layout.Secondary.X+3, layout.Secondary.Y+layout.Secondary.Height-3

	run := func(t *testing.T, secondary panel.State, paint func(tcell.Screen, Layout, Model, theme.Theme, int) bool) (survived bool) {
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatalf("Init: %v", err)
		}
		t.Cleanup(screen.Fini)
		screen.SetSize(w, h)
		model := Model{
			ViewMode: ViewBrowser, ActivePanel: PrimaryPanel, ActiveSubFocus: SubFocusFileList,
			OtherPanelMatchMarks: true,
			Primary:              marksTestPanel("/harbor", "lantern", "anchor"),
			Secondary:            secondary,
		}
		Render(screen, model, styles)
		screen.Put(sentinelX, sentinelY, "Z", tcell.StyleDefault)
		model.Primary.Cursor = 1
		if !paint(screen, layout, model, styles, PrimaryPanel) {
			t.Fatal("expected partial paint to succeed")
		}
		str, _, _ := screen.Get(sentinelX, sentinelY)
		return str == "Z"
	}

	t.Run("no match keeps other panel", func(t *testing.T) {
		t.Parallel()
		if !run(t, marksTestPanel("/meadow", "violin"), PaintBrowserCursorMovePanelOnly) {
			t.Fatal("other panel was repainted although no mark can follow the cursor")
		}
	})
	t.Run("match repaints other panel", func(t *testing.T) {
		t.Parallel()
		if run(t, marksTestPanel("/meadow", "anchor"), PaintBrowserCursorMovePanelOnly) {
			t.Fatal("other panel was not repainted although a mark follows the cursor")
		}
	})
	t.Run("listing paint repaints with marks on", func(t *testing.T) {
		t.Parallel()
		if run(t, marksTestPanel("/meadow", "violin"), PaintBrowserListNavPanelOnly) {
			t.Fatal("PaintBrowserListNavPanelOnly must repaint both panels with marks on")
		}
	})
}
