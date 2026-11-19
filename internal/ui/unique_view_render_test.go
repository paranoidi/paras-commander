package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/theme"
)

// Like the main file list: the cursor row's folder icon takes the cursor icon color.
func TestDrawUniqueTreeCursorDirFolderIconUsesCursorIconColor(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(60, 12)

	styles := theme.Default()
	view := &UniqueViewState{Root: "/srv/meadow", Files: []UniqueFile{
		{Path: "/srv/meadow/harbor/lantern.txt", Size: 2},
		{Path: "/srv/meadow/clover.txt", Size: 1},
	}}
	rows := UniqueRows(view)
	if !rows[0].Value.IsDir {
		t.Fatalf("first row = %+v, want the harbor directory", rows[0].Value)
	}
	view.Pane.Selected = 0
	rect := Rect{X: 0, Y: 0, Width: 60, Height: 12}
	drawUniqueTree(screen, rect, view, rows, uniqueTreeParams{Focused: true}, styles)

	want, ok := styles.CursorIconForeground(styles.PanelListingCursorIconKey(theme.PanelListingCursorOpts{FileListActive: true}))
	if !ok {
		t.Fatal("default theme has no active cursor icon color")
	}
	dirFG, _, _ := styles.PanelRowDirectory.Decompose()
	if want == dirFG {
		t.Fatal("test requires distinct cursor icon and panel.row.directory foregrounds")
	}

	r := rows[0]
	iconX := rect.X + 2 + len([]rune(dedupConnectorPrefix(styles, r.Depth, r.LastChild, r.AncestorHasNext)))
	rowY := rect.Y + 2 // border, column header
	if fg, _, _ := cellStyleAt(screen, iconX, rowY).Decompose(); fg != want {
		t.Fatalf("cursor dir folder icon fg %v, want cursor icon fg %v", fg, want)
	}
}

func TestDrawUniqueTreeHoverFollowsFileList(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(60, 12)

	styles := theme.Default()
	view := &UniqueViewState{Root: "/srv/meadow", Files: []UniqueFile{
		{Path: "/srv/meadow/harbor/lantern.txt", Size: 2},
		{Path: "/srv/meadow/clover.txt", Size: 1},
	}}
	rows := UniqueRows(view)
	view.Pane.Selected = len(rows) - 1 // cursor off the hovered harbor row
	rect := Rect{X: 0, Y: 0, Width: 60, Height: 12}
	drawUniqueTree(screen, rect, view, rows, uniqueTreeParams{Focused: true, Hover: ListRowHover{Active: true, Row: 0}}, styles)

	r := rows[0]
	iconX := rect.X + 2 + len([]rune(dedupConnectorPrefix(styles, r.Depth, r.LastChild, r.AncestorHasNext)))
	assertListingHoverRow(t, screen, styles, iconX, rect.X+rect.Width-2, rect.Y+2)
}
