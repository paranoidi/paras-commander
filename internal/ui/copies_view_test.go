package ui

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawCopiesListShowsNameDirectoryAndSize(t *testing.T) {
	styles := theme.Default()
	rect := Rect{X: 0, Y: 0, Width: 70, Height: 10}
	screen := newTreeTestScreen(t, 70, 10)
	view := &CopiesViewState{Origin: "lantern.txt", Files: []UniqueFile{
		{Path: "/srv/meadow/lantern.txt", Size: 2048},
		{Path: "/srv/orchard/clover/lantern.txt", Size: 2048},
	}}
	view.SetMark(view.Files[1], true)
	drawCopiesList(screen, rect, view, Model{AuxListHover: ListRowHover{}}, styles, false)

	var text []string
	for y := range rect.Height {
		text = append(text, tcelltest.TextAt(screen, 0, y, rect.Width))
	}
	all := strings.Join(text, "\n")
	for _, want := range []string{"Other copies of lantern.txt (2)", "Name", "Directory", "Size", "/srv/meadow", "/srv/orchard/clover", " 2K", "1 marked"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
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
