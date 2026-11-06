package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
)

// nav.home while already in the home directory reloads it and leaves the cursor where it was.
func TestNavHomeInHomeKeepsCursor(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"cherry", "damson", "elder"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, home)
	app.model.UserHomeDir = home
	selectEntryByName(t, app, "damson")
	row := app.activePanel().Cursor

	app.dispatch(keymap.ActionNavHome)
	if app.activePanel().ListingPending {
		applyNextInterruptEvent(t, app, screen)
	}

	entry, ok := app.activePanel().CurrentEntry()
	if !ok || entry.Name != "damson" || app.activePanel().Cursor != row {
		t.Fatalf("cursor = %d on %q, want %d on damson", app.activePanel().Cursor, entry.Name, row)
	}
}
