package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawUniqueTreeRowStatesFollowFileList(t *testing.T) {
	styles := theme.Default()
	rect := Rect{X: 0, Y: 0, Width: 60, Height: 12}
	for _, c := range treeRowStateCases(t, styles) {
		t.Run(c.name, func(t *testing.T) {
			screen := newTreeTestScreen(t, 60, 12)
			view := &UniqueViewState{Root: "/srv/meadow", Files: []UniqueFile{
				{Path: "/srv/meadow/harbor/lantern.txt", Size: 2},
				{Path: "/srv/meadow/clover.txt", Size: 1},
			}}
			rows := UniqueRows(view)
			if !rows[0].Value.IsDir {
				t.Fatalf("first row = %+v, want the harbor directory", rows[0].Value)
			}
			view.Pane.Selected = len(rows) - 1
			if c.cursor {
				view.Pane.Selected = 0
			}
			drawUniqueTree(screen, rect, view, rows, uniqueTreeParams{Focused: true, Hover: c.hover}, styles)
			r := rows[0]
			iconX := rect.X + 2 + len([]rune(dedupConnectorPrefix(styles, r.Depth, r.LastChild, r.AncestorHasNext)))
			assertTreeRowState(t, screen, c, iconX, rect.X+rect.Width-2, rect.Y+2)
		})
	}
}
