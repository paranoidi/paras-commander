package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	metactrl "github.com/paranoidi/paras-commander/internal/apphandler/meta"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fschange"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// waitForMetaFSWake polls the screen until the sink's wake arrives.
func waitForMetaFSWake(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !app.screen.HasPendingEvent() {
			time.Sleep(time.Millisecond)
			continue
		}
		if in, ok := app.screen.PollEvent().(*tcell.EventInterrupt); ok {
			if _, ok := in.Data().(metaFSWakePayload); ok {
				return
			}
		}
	}
	t.Fatal("no meta wake was posted")
}

func TestMetaFSSink_applyDoesNotPostSynchronously(t *testing.T) {
	dir := t.TempDir()
	app := testColumnsDialogApp(t, dir, filepath.Join(dir, "config"))
	sink := &metaFSSink{app: app}

	for range 3 {
		sink.Apply([]fschange.Change{{Kind: fschange.Removed, Path: filepath.Join(dir, "harbor.txt")}})
	}
	if app.screen.HasPendingEvent() {
		t.Fatalf("Apply posted within the %v window; dialog operations call it from the event loop", config.MetaFSChangeDebounce)
	}
	waitForMetaFSWake(t, app)

	got := asyncWakes(app).metaFS.drain()
	if len(got) != 3 {
		t.Fatalf("queued %d batches, want 3", len(got))
	}
}

func TestMetaFSSink_unboundIgnoresChanges(t *testing.T) {
	(&metaFSSink{}).Apply([]fschange.Change{{Kind: fschange.Removed, Path: "/vol/harbor.txt"}})
}

// TestRefreshIndexViews_doesNotRerunShellColumns: an index change must not restart a panel's run,
// which would recompute every row of its shell columns.
func TestRefreshIndexViews_doesNotRerunShellColumns(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "[[entry]]\nname = \"words\"\ndescription = \"Words\"\nfile = \"echo meadow\"\ncache = true\n"
	if err := os.WriteFile(filepath.Join(cfgDir, config.DefaultMetaFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	harbor := filepath.Join(dir, "harbor.txt")
	if err := os.WriteFile(harbor, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	app := testColumnsDialogApp(t, dir, cfgDir)
	app.model.ColumnsDialog = dialog.ColumnsDialogState{
		Open:    true,
		PanelID: ui.PrimaryPanel,
		Entries: []dialog.MetaEntry{{Name: "words", Description: "Words"}},
		Checked: []bool{true},
	}
	app.metaCtrl.ActivateSelection()

	cell := func() string { return app.model.MetaResults[ui.PrimaryPanel][0].Results[harbor] }
	pump := func(until time.Time, done func() bool) (loads int) {
		for time.Now().Before(until) && !done() {
			if !app.screen.HasPendingEvent() {
				time.Sleep(time.Millisecond)
				continue
			}
			in, ok := app.screen.PollEvent().(*tcell.EventInterrupt)
			if !ok {
				continue
			}
			switch d := in.Data().(type) {
			case metactrl.LoadPayload:
				loads++
				app.metaCtrl.HandleLoad(d)
			case metactrl.WakePayload:
				app.metaCtrl.HandleWake(d)
			}
		}
		return loads
	}
	pump(time.Now().Add(5*time.Second), func() bool { return cell() == "meadow" })
	if cell() != "meadow" {
		t.Fatalf("cell = %q, the column never resolved", cell())
	}

	app.refreshIndexViews()

	if loads := pump(time.Now().Add(100*time.Millisecond), func() bool { return false }); loads != 0 {
		t.Fatalf("an index change restarted the run (%d meta load)", loads)
	}
	if cell() != "meadow" {
		t.Fatalf("cell = %q, want the value kept", cell())
	}
}
