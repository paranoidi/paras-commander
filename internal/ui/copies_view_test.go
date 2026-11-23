package ui

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawCopiesListShowsPlainPaths(t *testing.T) {
	styles := theme.Default()
	rect := Rect{X: 0, Y: 0, Width: 70, Height: 10}
	screen := newTreeTestScreen(t, 70, 10)
	view := &CopiesViewState{Origin: "lantern.txt", Files: []UniqueFile{
		{Path: "/srv/meadow/lantern.txt", Size: 2048},
		{Path: "/srv/orchard/clover/lantern.txt", Size: 2048},
	}}
	view.Pane.AppendFilterRune('l', []string{"lantern.txt", "lantern.txt"}, quickfilter.Options{})
	view.SetMark(view.Files[1], true)
	view.FocusPanel = true // the title row shows the title, not the filter
	drawCopiesList(screen, rect, view, &Model{}, styles, false)

	if title := tcelltest.TextAt(screen, 0, 0, rect.Width); !strings.Contains(title, "Other copies of lantern.txt (2)") || !strings.Contains(title, "1 marked") {
		t.Errorf("title = %q", title)
	}
	for i, f := range view.Files {
		y := CopiesListTop(rect) + i
		if got := strings.TrimSpace(tcelltest.TextAt(screen, 1, y, rect.Width-2)); got != f.Path {
			t.Errorf("row %d = %q, want %q", i, got, f.Path)
		}
		// The name match is highlighted where the name starts in the path.
		_, style, _ := screen.Get(2+len(f.Path)-len("lantern.txt"), y)
		wantFG, _, _ := styles.FuzzyHighlight.Decompose()
		if fg, _, _ := style.Decompose(); fg != wantFG {
			t.Errorf("row %d: the match is not highlighted", i)
		}
	}
}

// A cursor path the list shortens shows in full above the footer, but only while the list has focus.
func TestDrawCopiesPathHint(t *testing.T) {
	const w, h = 80, 24
	styles := theme.Default()
	draw := func(path string, focusPanel, blocked bool, orientation SplitOrientation) string {
		screen := newTreeTestScreen(t, w, h)
		layout := CalculateLayoutWithOrientation(w, h, true, PanelPaneSplit{}, orientation, 0, 0, false, false)
		model := &Model{}
		model.CopiesView = CopiesViewState{Files: []UniqueFile{{Path: path}}, FocusPanel: focusPanel}
		drawCopiesView(screen, layout, model, styles, blocked)
		return tcelltest.TextAt(screen, 0, layout.Footer.Y-1, w)
	}
	long := "/srv/meadow/orchard/clover/juniper/lantern-harbor.txt"
	if got := draw(long, false, false, SplitHorizontal); !strings.Contains(got, " "+long+" ") {
		t.Errorf("no full path above the footer: %q", got)
	}
	for name, got := range map[string]string{
		"panel focused": draw(long, true, false, SplitHorizontal),
		"dialog open":   draw(long, false, true, SplitHorizontal),
		"stacked":       draw(long, false, false, SplitVertical),
		"path fits":     draw("/srv/lantern.txt", false, false, SplitHorizontal),
	} {
		if strings.Contains(got, "/srv") {
			t.Errorf("%s: hint shown: %q", name, got)
		}
	}
	tooLong := "/" + strings.Repeat("meadow/", 14) + "lantern.txt"
	if got := draw(tooLong, false, false, SplitHorizontal); !strings.Contains(got, "lantern.txt") {
		t.Errorf("a path wider than the screen lost its name: %q", got)
	}
}

func TestCopiesViewMarksKeepBytes(t *testing.T) {
	view := &CopiesViewState{Files: []UniqueFile{{Path: "/a/x", Size: 5}, {Path: "/b/x", Size: 7}}}
	view.SetMark(view.Files[0], true)
	view.SetMark(view.Files[1], true)
	view.SetMark(view.Files[0], true) // no double count
	if view.MarkedBytes != 12 || len(view.MarkedFiles()) != 2 {
		t.Fatalf("marked = %d bytes, %v", view.MarkedBytes, view.MarkedFiles())
	}
	view.SetMark(view.Files[0], false)
	if view.MarkedBytes != 7 || len(view.Marked) != 1 {
		t.Fatalf("after unmark: %d bytes, %v", view.MarkedBytes, view.Marked)
	}
}
