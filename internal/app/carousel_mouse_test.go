package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// carouselMouseApp builds a full-width carousel panel over root/meadow, which holds a few
// directories and files; root holds meadow and the sibling directories walnut and pepper.
func carouselMouseApp(t *testing.T) (app *App, root, mid string) {
	t.Helper()
	root = t.TempDir()
	mid = filepath.Join(root, "meadow")
	for _, d := range []string{"meadow", "walnut", "pepper"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{"lantern", "harbor", "velvet"} {
		if err := os.MkdirAll(filepath.Join(mid, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"canyon.txt", "ribbon.txt", "orchid.txt"} {
		writeFile(t, filepath.Join(mid, f))
	}
	app = newApp(t, newScreen(t, 200, 30), mid)
	app.config.UI.Mouse = true
	app.config.UI.KeyRepeatDebounceMS = 0
	app.model.HideInactivePanel = true
	app.model.Primary.CarouselMode = true
	return app, root, mid
}

func carouselFrame(app *App) ui.Rect {
	w, h := app.screen.Size()
	lay := app.layoutForTerminalSize(w, h)
	file, _ := app.panelFileFrame(lay, ui.PrimaryPanel)
	return file
}

// carouselRowCell finds a screen cell on listing row idx of carousel column col.
func carouselRowCell(t *testing.T, app *App, col, idx int) (int, int) {
	t.Helper()
	file := carouselFrame(app)
	for y := file.Y; y < file.Y+file.Height; y++ {
		for x := file.X; x < file.X+file.Width; x++ {
			if hit, _, ok := ui.CarouselEntryAt(file, &app.model, ui.PrimaryPanel, app.effectiveHideInactivePanel(), x, y); ok && hit.Column == col && hit.Index == idx {
				return x, y
			}
		}
	}
	t.Fatalf("no cell on column %d row %d", col, idx)
	return 0, 0
}

// carouselColumnCell finds a screen cell on the list rows of carousel column col.
func carouselColumnCell(t *testing.T, app *App, col int) (int, int) {
	t.Helper()
	file := carouselFrame(app)
	for y := file.Y; y < file.Y+file.Height; y++ {
		for x := file.X; x < file.X+file.Width; x++ {
			if c, _, ok := ui.CarouselColumnAt(file, &app.model, ui.PrimaryPanel, app.effectiveHideInactivePanel(), x, y); ok && c == col {
				return x, y
			}
		}
	}
	t.Fatalf("no cell on column %d", col)
	return 0, 0
}

func syntheticListing(path string, n int) panel.ListingSnapshot {
	words := []string{"anchor", "breeze", "copper", "falcon", "garden", "marble"}
	snap := panel.ListingSnapshot{Path: pathloc.MustParse(path)}
	for i := range n {
		name := fmt.Sprintf("%s%02d", words[i%len(words)], i)
		snap.Entries = append(snap.Entries, localfs.Entry{Name: name, Path: filepath.Join(path, name), Type: localfs.EntryDirectory})
	}
	return snap
}

func TestCarouselMouseCenterRowClickMovesCursor(t *testing.T) {
	app, _, _ := carouselMouseApp(t)
	app.render()
	x, y := carouselRowCell(t, app, 1, 2)
	click(app, x, y, tcell.Button1)
	if got := app.model.Primary.Cursor; got != 2 {
		t.Fatalf("cursor = %d want 2", got)
	}
}

func TestCarouselMouseParentClickNavigatesToParent(t *testing.T) {
	app, root, _ := carouselMouseApp(t)
	snap := syntheticListing(root, 0)
	for _, n := range []string{"meadow", "pepper", "walnut"} {
		snap.Entries = append(snap.Entries, localfs.Entry{Name: n, Path: filepath.Join(root, n), Type: localfs.EntryDirectory})
	}
	app.model.Primary.CarouselSideCache.Parent, app.model.Primary.CarouselSideCache.ParentOK = snap, true
	app.render()
	x, y := carouselRowCell(t, app, 0, 2)
	click(app, x, y, tcell.Button1)
	drainInterruptEventsUntil(t, app, app.screen.(tcell.SimulationScreen), 2*time.Second, func() bool {
		return app.model.Primary.PathString() == root
	})
	if got := app.model.Primary.PathString(); got != root {
		t.Fatalf("path = %q want %q", got, root)
	}
	if e, ok := app.model.Primary.CurrentEntry(); !ok || e.Name != "walnut" {
		t.Fatalf("cursor entry = %q want walnut", e.Name)
	}
}

func TestCarouselMouseHeaderTitleSorts(t *testing.T) {
	app, _, _ := carouselMouseApp(t)
	app.render()
	file := carouselFrame(app)
	hideInactive := app.effectiveHideInactivePanel()
	hx := -1
	for x := file.X + 1; x < file.X+file.Width-1; x++ {
		if c, onTitle, _ := ui.PanelHeaderColumnAt(file, &app.model, ui.PrimaryPanel, hideInactive, x, file.Y+1); onTitle && c.Mode == panel.SortSize {
			hx = x
			break
		}
	}
	if hx < 0 {
		t.Fatal("no Size title on the carousel header")
	}
	click(app, hx, file.Y+1, tcell.Button1)
	if got := app.model.Primary.Sort.Mode; got != panel.SortSize {
		t.Fatalf("sort mode = %v want size", got)
	}
}

func TestCarouselMouseWheelScrollsSideColumns(t *testing.T) {
	app, root, mid := carouselMouseApp(t)
	app.model.Primary.CarouselSideCache.Parent, app.model.Primary.CarouselSideCache.ParentOK = syntheticListing(root, 80), true
	app.model.Primary.CarouselSideCache.Child, app.model.Primary.CarouselSideCache.ChildOK = syntheticListing(filepath.Join(mid, "lantern"), 80), true
	if !app.model.Primary.SelectVisibleEntry("lantern") {
		t.Fatal("lantern not listed")
	}
	app.render()
	cursor := app.model.Primary.Cursor
	for col, snap := range map[int]*panel.ListingSnapshot{
		0: &app.model.Primary.CarouselSideCache.Parent,
		2: &app.model.Primary.CarouselSideCache.Child,
	} {
		x, y := carouselColumnCell(t, app, col)
		before := snap.Scroll
		app.handleMouse(mouseEv(x, y, tcell.WheelDown))
		if got := snap.Scroll; got != before+config.MouseWheelRows {
			t.Fatalf("column %d scroll = %d want %d", col, got, before+config.MouseWheelRows)
		}
		app.handleMouse(mouseEv(x, y, tcell.WheelUp))
		if got := snap.Scroll; got != before {
			t.Fatalf("column %d scroll after wheel up = %d want %d", col, got, before)
		}
		if snap.Cursor != 0 || app.model.Primary.Cursor != cursor {
			t.Fatalf("column %d wheel moved a cursor", col)
		}
	}
	x, y := carouselColumnCell(t, app, 1)
	app.handleMouse(mouseEv(x, y, tcell.WheelDown))
	if got := app.model.Primary.Cursor; got != cursor+config.MouseWheelRows {
		t.Fatalf("center wheel cursor = %d want %d", got, cursor+config.MouseWheelRows)
	}
}
