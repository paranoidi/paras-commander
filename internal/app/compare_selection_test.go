package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// compareSelectionApp is an app whose main panels sit on separate trees, with one file
// selected in the left panel (left) and one stashed in the right panel (right).
func compareSelectionApp(t *testing.T) (app *App, left, right string) {
	t.Helper()
	leftDir, rightDir := t.TempDir(), t.TempDir()
	left, right = filepath.Join(leftDir, "meadow.txt"), filepath.Join(rightDir, "harbor.txt")
	for _, p := range []string{left, right} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app = newApp(t, newScreen(t, 80, 24), leftDir)
	app.model.Primary.Path = pathloc.MustParse(leftDir)
	app.model.Secondary.Path = pathloc.MustParse(rightDir)
	app.model.Primary.AddSelection(left)
	return app, left, right
}

func TestCompareOpensAtOnceWithoutSelection(t *testing.T) {
	app, _, _ := compareSelectionApp(t)
	app.model.Primary.ClearSelection()
	app.openComparePanels()
	if app.model.CompareSelection.Open || app.model.ViewMode != ui.ViewCompare {
		t.Fatalf("dialog=%v view=%v, want Compare open without asking", app.model.CompareSelection.Open, app.model.ViewMode)
	}
}

func TestCompareWithSelectionAsksFirst(t *testing.T) {
	app, _, _ := compareSelectionApp(t)
	app.openComparePanels()
	d := app.model.CompareSelection
	if !d.Open || d.Selected != 1 || d.Focus != dialog.CompareSelectionStash {
		t.Fatalf("dialog = %+v, want open on Stash with 1 selected", d)
	}
	if app.model.ViewMode == ui.ViewCompare {
		t.Fatal("Compare opened before the selection was resolved")
	}
	if app.inputMode() != InputModeDialog {
		t.Fatalf("input mode = %v, want dialog", app.inputMode())
	}
}

func TestCompareSelectionDrop(t *testing.T) {
	app, _, _ := compareSelectionApp(t)
	app.openComparePanels()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModAlt))
	if app.model.CompareSelection.Open || app.model.ViewMode != ui.ViewCompare {
		t.Fatalf("dialog=%v view=%v, want Compare open", app.model.CompareSelection.Open, app.model.ViewMode)
	}
	if n := app.model.Primary.SelectedPathCount(); n != 0 {
		t.Fatalf("live selection = %d, want 0", n)
	}
	if !app.model.Primary.StashEmpty() {
		t.Fatal("Drop must not stash")
	}
}

func TestCompareSelectionStashMergesIntoExistingStash(t *testing.T) {
	app, left, _ := compareSelectionApp(t)
	other := filepath.Join(filepath.Dir(left), "lantern.txt")
	if err := os.WriteFile(other, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &app.model.Primary
	p.ClearSelection()
	p.AddSelection(other)
	p.StashSaveFromSelection()
	p.AddSelection(left)

	app.openComparePanels()
	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)) // Stash has focus
	if app.model.ViewMode != ui.ViewCompare {
		t.Fatalf("view = %v, want Compare", app.model.ViewMode)
	}
	if p.SelectedPathCount() != 0 || p.StashPathCount() != 2 {
		t.Fatalf("live=%d stash=%d, want 0 and 2", p.SelectedPathCount(), p.StashPathCount())
	}
}

func TestCompareSelectionCancelKeepsSelection(t *testing.T) {
	app, _, _ := compareSelectionApp(t)
	app.openComparePanels()
	app.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if app.model.CompareSelection.Open || app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("dialog=%v view=%v, want browser", app.model.CompareSelection.Open, app.model.ViewMode)
	}
	if app.model.Primary.SelectedPathCount() != 1 || app.compareSelectionOpen != nil {
		t.Fatal("Cancel must keep the selection and drop the pending open")
	}
}

// The right panel's selection counts too: Compare's right column acts on it.
func TestCompareSelectionCountsBothPanels(t *testing.T) {
	app, _, right := compareSelectionApp(t)
	app.model.Secondary.AddSelection(right)
	app.openComparePanels()
	if d := app.model.CompareSelection; d.Selected != 2 {
		t.Fatalf("selected = %d, want 2", d.Selected)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModAlt))
	if app.model.Primary.StashPathCount() != 1 || app.model.Secondary.StashPathCount() != 1 {
		t.Fatalf("stash left=%d right=%d, want 1 each", app.model.Primary.StashPathCount(), app.model.Secondary.StashPathCount())
	}
}
