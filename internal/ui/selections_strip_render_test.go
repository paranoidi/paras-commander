package ui

import (
	"os"
	"path/filepath"
	"strings"
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

	drawSelectionsStrip(screen, rect, state, false, false, SelectionsStripOpts{
		Styles: styles, ShowSelectionSizeOnBottom: true, ScrollbarStyle: uiscrollbar.StyleNone,
		ScrollbarShowInactive: true, PanelFileListActive: true,
	})

	rowY := rect.Y + 1
	wantMark := styles.SymbolFilelistSelectionSubtree()
	markCol := -1
	for col := rect.X + 1; col < rect.X+rect.Width-1; col++ {
		ch, _, _ := screen.Get(col, rowY)
		r, _ := utf8.DecodeRuneInString(ch)
		if r == wantMark {
			markCol = col
			break
		}
	}
	if markCol < 0 {
		t.Fatal("selection mark not found on file strip row")
	}
	_, markStyle, _ := screen.Get(markCol, rowY)
	gotFG, _, _ := markStyle.Decompose()
	if gotFG != wantFG {
		t.Fatalf("selection mark foreground = %v, want panel.row.selected %v", gotFG, wantFG)
	}
}

// TestSelectionsStripMarkOnCursorRow verifies the active row's selection mark follows the
// cursor row's icon color override instead of staying the static panel.row.selected color.
func TestSelectionsStripMarkOnCursorRow(t *testing.T) {
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
	wantFG, ok := styles.PanelFileIconFG["panel.active.row.cursor"]
	if !ok {
		t.Fatal("theme missing panel.active.row.cursor icon override")
	}

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
	wantMark := styles.SymbolFilelistSelectionSubtree()
	markCol := -1
	for col := rect.X + 1; col < rect.X+rect.Width-1; col++ {
		ch, _, _ := screen.Get(col, rowY)
		r, _ := utf8.DecodeRuneInString(ch)
		if r == wantMark {
			markCol = col
			break
		}
	}
	if markCol < 0 {
		t.Fatal("selection mark not found on cursor row")
	}
	_, markStyle, _ := screen.Get(markCol, rowY)
	gotFG, _, _ := markStyle.Decompose()
	if gotFG != wantFG {
		t.Fatalf("cursor row selection mark foreground = %v, want panel.active.row.cursor icon %v", gotFG, wantFG)
	}
}

func TestSelectionsStripTitleShowsMultiLocationIcon(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)

	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	bravo := filepath.Join(root, "bravo")
	for _, d := range []string{alpha, bravo} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	alphaFile := filepath.Join(alpha, "meadow.txt")
	bravoFile := filepath.Join(bravo, "canyon.txt")
	for _, f := range []string{alphaFile, bravoFile} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	styles := theme.Default()
	const width, height = 60, 8
	screen.SetSize(width, height)
	rect := Rect{X: 0, Y: 0, Width: width, Height: height}
	opts := SelectionsStripOpts{
		Styles: styles, ScrollbarStyle: uiscrollbar.StyleNone,
		ScrollbarShowInactive: true, PanelFileListActive: true,
	}

	multiState := panel.State{
		Path:                 pathloc.MustParse(alpha),
		SelectionsStripOrder: []string{alphaFile, bravoFile},
		SelectedPaths:        map[string]bool{alphaFile: true, bravoFile: true},
	}
	drawSelectionsStrip(screen, rect, multiState, true, false, opts)
	icon := styles.SymbolSelectionsMultiLocation()
	// The glyph is an end label on the top border, one frame dash before the corner: … ─ x ─┐
	iconX := rect.X + rect.Width - 4
	if got, _, _ := screen.Get(iconX, rect.Y); got != icon {
		t.Fatalf("top border col %d = %q, want multi-location icon %q", iconX, got, icon)
	}
	if got, _, _ := screen.Get(rect.X+rect.Width-2, rect.Y); got != "─" {
		t.Fatalf("expected frame dash between icon and corner, got %q", got)
	}

	for x := rect.X; x < rect.X+rect.Width; x++ {
		screen.SetContent(x, rect.Y, ' ', nil, tcell.StyleDefault)
	}
	singleState := panel.State{
		Path:                 pathloc.MustParse(alpha),
		SelectionsStripOrder: []string{alphaFile},
		SelectedPaths:        map[string]bool{alphaFile: true},
	}
	drawSelectionsStrip(screen, rect, singleState, true, false, opts)
	if rowContainsText(screen, rect.Y, rect.X, rect.X+rect.Width, icon) {
		t.Fatal("strip title should not contain the multi-location icon for a single-directory selection")
	}
}

// rowContainsText reports whether the cells on row y between [xStart, xEnd) contain needle.
func rowContainsText(screen tcell.SimulationScreen, y, xStart, xEnd int, needle string) bool {
	if needle == "" {
		return false
	}
	var sb strings.Builder
	for x := xStart; x < xEnd; x++ {
		ch, _, _ := screen.Get(x, y)
		sb.WriteString(ch)
	}
	return strings.Contains(sb.String(), needle)
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
