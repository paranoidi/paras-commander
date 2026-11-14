package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestSlashOpensNavigateToDialogPrefilledWithRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lantern.txt"))
	app := newApp(t, newScreen(t, 100, 30), dir)

	app.handleKey(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))

	d := app.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogNavigateTo {
		t.Fatalf("file dialog = %+v, want open navigate-to dialog", d)
	}
	if len(d.Fields) != 1 || d.Fields[0].Value != "/" || d.Fields[0].Cursor != 1 || !d.Fields[0].PathPicker {
		t.Fatalf("fields = %+v, want one path field prefilled with \"/\"", d.Fields)
	}
	if app.activePanel().Filter.Editing || app.activePanel().Filter.Query != "" {
		t.Fatalf("quick filter = %+v, want untouched", app.activePanel().Filter)
	}
}

func TestNavigateToDialogEnterMovesActivePanel(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "orchard", "meadow")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	start := t.TempDir()
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, start)

	app.handleKey(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	// The field starts as "/": type the rest of the absolute path after it.
	typeRunes(t, app, target[1:])
	app.model.FileDialog.Fields[0].Completion.Clear()
	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatalf("dialog still open: %+v", app.model.FileDialog)
	}
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool {
		p := app.activePanel()
		return p.PathString() == target && !p.ListingPending
	})
	if got := app.activePanel().PathString(); got != target {
		t.Fatalf("active panel path = %q, want %q", got, target)
	}
}

func TestNavigateToDialogMissingPathLeavesPanel(t *testing.T) {
	start := t.TempDir()
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, start)

	app.handleKey(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	typeRunes(t, app, filepath.Join(start, "harbor-missing")[1:])
	app.model.FileDialog.Fields[0].Completion.Clear()
	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool {
		return !app.activePanel().ListingPending
	})
	if got := app.activePanel().PathString(); got != start {
		t.Fatalf("active panel path = %q, want unchanged %q", got, start)
	}
}
