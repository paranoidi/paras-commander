package ui

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func TestSelectionsStripMarkOnFileRow(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)

	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(dir, "picked.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	styles := theme.Default()
	wantFG, _, _ := styles.PanelRowSelected.Decompose()

	const width, height = 60, 8
	screen.SetSize(width, height)
	rect := Rect{X: 0, Y: 0, Width: width, Height: height}
	state := panel.State{
		Path:                 pathloc.MustParse(sub),
		SelectionsStripOrder: []string{filePath},
		SelectedPaths:        map[string]bool{filePath: true},
	}

	drawSelectionsStrip(screen, rect, state, true, false, SelectionsStripOpts{
		Styles: styles, ShowSelectionSizeOnBottom: true, ScrollbarStyle: uiscrollbar.StyleNone,
		ScrollbarShowInactive: true, PanelFileListActive: true,
	})

	rowY := rect.Y + 1
	markCol := -1
	for col := rect.X + 1; col < rect.X+rect.Width-1; col++ {
		ch, _, _ := screen.Get(col, rowY)
		r, _ := utf8.DecodeRuneInString(ch)
		if r == '\u25cb' {
			markCol = col
			break
		}
	}
	if markCol < 0 {
		t.Fatal("selection mark ○ not found on file strip row")
	}
	_, markStyle, _ := screen.Get(markCol, rowY)
	gotFG, _, _ := markStyle.Decompose()
	if gotFG != wantFG {
		t.Fatalf("○ foreground = %v, want panel.row.selected %v", gotFG, wantFG)
	}
}

func TestSelectionStripDisplayPathRelative(t *testing.T) {
	t.Parallel()
	const cur = "/home/rover/notebook"
	cases := []struct{ abs, want string }{
		{"/home/rover/notebook/current.txt", "current.txt"},
		{"/home/rover/notebook/child/inside.txt", "child/inside.txt"},
		{"/home/rover/sibling.txt", "../sibling.txt"},
	}
	for _, c := range cases {
		if got := selectionStripDisplayPath(c.abs, cur, "/home/rover", 60); got != c.want {
			t.Fatalf("selectionStripDisplayPath(%q) = %q, want %q", c.abs, got, c.want)
		}
	}
}
