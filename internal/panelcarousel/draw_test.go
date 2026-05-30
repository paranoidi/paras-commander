package panelcarousel

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

func TestSubtreeSelectionMarkUsesSelectedForeground(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 80, 10
	screen.SetSize(width, height)

	styles := theme.Default()
	wantFG, ok := styles.PanelFileIconFG["panel.active.row.cursor"]
	if !ok {
		t.Fatal("default theme missing panel.active.row.cursor icon")
	}

	root := "/vol"
	parentPath := root + "/parent"
	childPath := parentPath + "/nested.txt"
	center := panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "parent", Path: parentPath, Type: localfs.EntryDirectory},
		},
		Cursor:        0,
		SelectedPaths: map[string]bool{childPath: true},
	}

	frame := geom.Rect{X: 0, Y: 0, Width: width, Height: height}
	DrawBody(screen, BodyParams{
		Frame:               frame,
		Center:              center,
		Styles:              styles,
		FileListActive:      true,
		ShowIcons:           false,
		HeaderStyle:         styles.PanelActiveHeader,
		HeaderCarouselStyle: styles.PanelActiveHeaderCarousel,
		SurfaceStyle:        styles.PanelActiveSurface,
	})

	cols := SplitColumns(frame, true)
	centerCol := cols[1]
	rowY := centerCol.Y
	markCol := -1
	for col := centerCol.X; col < centerCol.X+centerCol.Width; col++ {
		ch, _, _ := screen.Get(col, rowY)
		r, _ := utf8.DecodeRuneInString(ch)
		if r == '\u25cb' {
			markCol = col
			break
		}
	}
	if markCol < 0 {
		t.Fatal("subtree selection mark ○ not found on directory row")
	}
	_, markStyle, _ := screen.Get(markCol, rowY)
	gotFG, _, _ := markStyle.Decompose()
	if gotFG != wantFG {
		t.Fatalf("○ foreground = %v, want cursor-row icon %v", gotFG, wantFG)
	}
}
