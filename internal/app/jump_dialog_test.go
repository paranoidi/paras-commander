package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/fileindex/db"
	jumpctrl "github.com/paranoidi/paras-commander/internal/apphandler/jump"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func openJumpOnFixture(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	findKeyFixture(t, dir)
	app := newApp(t, newScreen(t, 80, 24), dir)
	app.dispatch(keymap.ActionPanelJumpDialog)
	settleJump(t, app)
	if !app.model.JumpDialog.Open {
		t.Fatal("panel.jump-dialog did not open Jump to directory")
	}
	return app, dir
}

// Jump and a parked Find are independent: opening Jump leaves the parked Find alone (no relocate
// question), and Find comes back as it was after Jump closes.
func TestJumpLeavesParkedFindAlone(t *testing.T) {
	dir := t.TempDir()
	findKeyFixture(t, dir)
	app := newApp(t, newScreen(t, 80, 24), dir)
	app.findCtrl.OpenDialog(ui.PrimaryPanel)
	app.model.FindDialog.Query = "lantern"
	app.findCtrl.Park()

	app.dispatch(keymap.ActionPanelJumpDialog)
	settleJump(t, app)
	if app.model.FindRelocate.Open || !app.model.JumpDialog.Open || !app.model.FindDialog.Parked {
		t.Fatalf("relocate=%v jump=%v parked=%v, want Jump open beside the parked Find",
			app.model.FindRelocate.Open, app.model.JumpDialog.Open, app.model.FindDialog.Parked)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	app.dispatch(keymap.ActionPanelFindDialog)
	if !app.model.FindDialog.Open || app.model.FindDialog.Query != "lantern" {
		t.Fatalf("find open=%v query=%q, want the parked Find restored", app.model.FindDialog.Open, app.model.FindDialog.Query)
	}
}

// Every open rescans: a directory created after Jump was closed shows up when it opens again.
func TestJumpRescansOnEveryOpen(t *testing.T) {
	app, dir := openJumpOnFixture(t)
	app.jumpCtrl.Close()
	if err := os.Mkdir(filepath.Join(dir, "zephyr"), 0o755); err != nil {
		t.Fatal(err)
	}
	app.dispatch(keymap.ActionPanelJumpDialog)
	settleJump(t, app)
	for _, r := range app.model.JumpDialog.Results {
		if r.Rel == "zephyr" {
			return
		}
	}
	t.Fatalf("results %+v lack the new directory", app.model.JumpDialog.Results)
}

// Shift+Right points the other panel at the cursor directory and keeps the dialog open.
func TestJumpOpenInSecondaryKeepsDialog(t *testing.T) {
	app, dir := openJumpOnFixture(t)
	app.handleKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	if !app.model.JumpDialog.Open {
		t.Fatal("open-in-panel closed the dialog")
	}
	want := filepath.Join(dir, app.model.JumpDialog.Results[0].Rel)
	screen := app.screen.(tcell.SimulationScreen)
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool {
		return app.panelByID(app.model.SecondSlotPanel()).PathString() == want
	})
}

// A match result for an earlier query is dropped.
func TestJumpDropsStaleIndexResult(t *testing.T) {
	app, _ := openJumpOnFixture(t)
	before := len(app.model.JumpDialog.Results)
	if app.jumpCtrl.HandleIndexResult(jumpctrl.IndexResultPayload{Gen: -1, Results: []dialog.JumpResult{{Path: "/x", Rel: "x"}}}) {
		t.Fatal("stale result applied")
	}
	if len(app.model.JumpDialog.Results) != before {
		t.Fatal("stale result changed the list")
	}
}

// On a fileindex:// panel Jump queries the index for directories, and Enter shows the directory
// as the index remembers it.
func TestJumpOnFileIndexPanelQueriesIndex(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	sum := strings.Repeat("c", 32)
	orchard := filepath.Join(dir, "orchard")
	if _, err := d.CommitScanBatch([]db.ScannedFile{{Dir: orchard, Name: "pebble.txt", Size: 1, Mtime: time.Now(), MD5: sum, SHA256: sum + sum}}); err != nil {
		t.Fatal(err)
	}
	a.dispatch(keymap.ActionFileIndexBrowse)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.focusedPanel()
		return p.PathString() == "fileindex://"+dir && !p.ListingPending
	})
	a.dispatch(keymap.ActionPanelJumpDialog)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.JumpDialog.Open })
	if st := a.model.JumpDialog; st.Source != dialog.JumpIndex || st.Root != dir {
		t.Fatalf("state = %+v", st)
	}
	for _, r := range "orch" {
		a.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		st := a.model.JumpDialog
		return !st.Searching && len(st.Results) == 1
	})
	if r := a.model.JumpDialog.Results[0]; r.Path != orchard || r.Rel != "orchard" {
		t.Fatalf("result = %+v", r)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return a.focusedPanel().PathString() == "fileindex://"+orchard
	})
	if a.model.JumpDialog.Open {
		t.Fatal("Enter should close the dialog")
	}
}
