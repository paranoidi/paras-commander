package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/statuscmds"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

const statusTestTOML = `show = ["walnut"]

[[command]]
name = "walnut"
command = "echo harbor"

[[command]]
name = "pebble"
label = "Pebble"
command = "echo gravel"
`

func TestStatusCommandsDialogSavesShowAndRestarts(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir, "status.toml")
	if err := os.WriteFile(path, []byte(statusTestTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	opts := testOptions(dir)
	opts.Paths.ConfigDir = configDir
	app := newTestApp(t, screen, opts)
	app.config.UI.KeyRepeatDebounceMS = 0

	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool {
		return slices.Equal(app.statusCommandTexts(), []string{"harbor"})
	})
	staleGen := app.statusCmdGen

	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModAlt))
	st := &app.model.StatusCommandsDialog
	if !st.Open {
		t.Fatal("status commands dialog not open after Alt+A")
	}
	if len(st.Rows) != len(statuscmds.Builtins())+2 {
		t.Fatalf("rows = %+v, want built-ins + walnut + pebble", st.Rows)
	}
	app.applyStatusCommandsDialogEvent(dlg.Event{ID: "status:walnut"})
	app.applyStatusCommandsDialogEvent(dlg.Event{ID: "status:pebble"})
	app.applyStatusCommandsDialogEvent(dlg.Event{ID: dlg.OK})
	if st.Open {
		t.Fatal("dialog still open after OK")
	}

	set, err := statuscmds.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !slices.Equal(set.Show, []string{"pebble"}) {
		t.Fatalf("saved show = %v, want [pebble]", set.Show)
	}
	if app.applyStatusCommandResult(statusCommandResultPayload{Gen: staleGen, Index: 0, Text: "stale"}) {
		t.Fatal("stale-generation result applied")
	}
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool {
		return slices.Equal(app.statusCommandTexts(), []string{"gravel"})
	})
}
