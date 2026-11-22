package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/ui"
)

// Marking a directory near the bottom of the results tree opens (or grows) the Selected strip,
// which shrinks the tree: the cursor must stay inside the shrunken tree.
func TestDedupMarkDirKeepsCursorVisibleWhenStripGrows(t *testing.T) {
	dir := t.TempDir()
	var dirs []string
	for i := range 30 {
		d := fmt.Sprintf("meadow%02d", i)
		dirs = append(dirs, d)
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 8 {
		for _, d := range dirs {
			p := filepath.Join(dir, d, fmt.Sprintf("lantern%02d.txt", i))
			if err := os.WriteFile(p, []byte(fmt.Sprintf("copy %d", i)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	const w, h = 80, 24
	screen := newScreen(t, w, h)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)
	view := &app.model.DedupView
	key := func(k tcell.Key) { app.handleDedupViewKey(tcell.NewEventKey(k, 0, tcell.ModNone)) }

	// Walk down to the last directory row that is still on screen without marks.
	visible := app.dedupVisibleRows()
	target := -1
	for i, r := range app.model.DedupList {
		if r.Value.Kind == ui.DedupRowDir && i < visible && i > 0 {
			target = i
		}
	}
	if target < 0 {
		t.Fatalf("no directory row among the first %d rows", visible)
	}
	for view.Main.Selected < target {
		key(tcell.KeyDown)
	}
	key(tcell.KeyInsert)
	app.render()

	tree, _ := ui.DedupMainRects(app.layoutForTerminalSize(w, h).Primary, &app.model)
	rows := ui.PanelListRows(tree)
	if sel, top := view.Main.Selected, view.Main.ListScroll; sel < top || sel >= top+rows {
		t.Fatalf("cursor row %d outside the tree window [%d,%d) after marking a directory (marked %d)", sel, top, top+rows, view.MarkedCount)
	}
}
