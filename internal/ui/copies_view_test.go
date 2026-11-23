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
