package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/clipboard"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func testCopyMenuApp(t *testing.T) (*App, string) {
	t.Helper()
	clipboard.Reset()
	prevWriter := clipboard.OSWriter
	clipboard.OSWriter = func(string) error { return nil }
	t.Cleanup(func() { clipboard.OSWriter = prevWriter })
	dir := t.TempDir()
	filePath := filepath.Join(dir, "meadow.txt")
	writeFile(t, filePath)
	screen := newScreen(t, 80, 50)
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: config.Default(),
		Paths:  config.Paths{ConfigDir: filepath.Join(dir, "config")},
	})
	app.model.ViewMode = ui.ViewBrowser
	for i := 0; i < app.model.Primary.VisibleEntryCount(); i++ {
		entry, _, ok := app.model.Primary.VisibleEntry(i)
		if ok && entry.Name == "meadow.txt" {
			app.model.Primary.Cursor = i
			break
		}
	}
	return app, filePath
}

func TestOpenCopyMenuShowsCopyActions(t *testing.T) {
	app, _ := testCopyMenuApp(t)
	app.toggleCopyMenu()
	if !app.model.LeaderMenu.Open {
		t.Fatal("expected copy menu open")
	}
	if !app.model.LeaderMenu.CopyMenu {
		t.Fatal("expected CopyMenu=true")
	}
	if app.model.LeaderMenu.UserMenu {
		t.Fatal("expected UserMenu=false")
	}
	if len(app.model.LeaderMenu.Items) != 4 {
		t.Fatalf("items len = %d, want 4", len(app.model.LeaderMenu.Items))
	}
}

func TestCopyMenuFilenameKeyCopiesBasename(t *testing.T) {
	app, _ := testCopyMenuApp(t)
	app.toggleCopyMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone))
	if app.model.LeaderMenu.Open {
		t.Fatal("copy menu should close after f")
	}
	if got := clipboard.LastSet(); got != "meadow.txt" {
		t.Fatalf("clipboard = %q, want meadow.txt", got)
	}
}

func TestCopyMenuFileURLKeyCopiesPath(t *testing.T) {
	app, filePath := testCopyMenuApp(t)
	app.toggleCopyMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))
	if got := clipboard.LastSet(); got != filePath {
		t.Fatalf("clipboard = %q, want %q", got, filePath)
	}
}

func TestCopyMenuDirURLKeyCopiesParent(t *testing.T) {
	app, filePath := testCopyMenuApp(t)
	app.toggleCopyMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	wantDir := filepath.Dir(filePath)
	if got := clipboard.LastSet(); got != wantDir {
		t.Fatalf("clipboard = %q, want %q", got, wantDir)
	}
}

func TestCopyMenuNameWithoutExtKey(t *testing.T) {
	app, _ := testCopyMenuApp(t)
	app.toggleCopyMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if got := clipboard.LastSet(); got != "meadow" {
		t.Fatalf("clipboard = %q, want meadow", got)
	}
}

func TestQuoteKeyOpensCopyMenu(t *testing.T) {
	app, _ := testCopyMenuApp(t)
	id, ok := app.keys.Global.Lookup(tcell.NewEventKey(tcell.KeyRune, '"', tcell.ModNone))
	if !ok || id != keymap.ActionAppCopyMenu {
		t.Fatalf("lookup = %q %v, want %q", id, ok, keymap.ActionAppCopyMenu)
	}
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionAppCopyMenu)
	if !app.copyMenuOpen() {
		t.Fatalf("LeaderMenu = %+v, want copy menu open", app.model.LeaderMenu)
	}
}

func TestQuoteKeyTogglesCopyMenuClosed(t *testing.T) {
	app, _ := testCopyMenuApp(t)
	app.toggleCopyMenu()
	if !app.copyMenuOpen() {
		t.Fatal("expected copy menu open")
	}

	quote := tcell.NewEventKey(tcell.KeyRune, '"', tcell.ModNone)
	_, rendered := app.handleKey(quote)
	if !rendered {
		t.Fatal("second \" should render after closing the copy menu")
	}
	if app.model.LeaderMenu.Open {
		t.Fatal("second \" should close the copy menu")
	}
}

func TestCopyMenuEmptySelectionOnParentRowWarns(t *testing.T) {
	app, _ := testCopyMenuApp(t)
	for i := 0; i < app.model.Primary.VisibleEntryCount(); i++ {
		entry, _, ok := app.model.Primary.VisibleEntry(i)
		if ok && entry.Name == ".." {
			app.model.Primary.Cursor = i
			break
		}
	}
	app.copyToClipboard(keymap.ActionClipboardCopyFilename)
	if app.model.Message == "" {
		t.Fatal("expected warning when copying filename on .. row")
	}
}

func TestCopyToClipboardWarnsWhenClipboardToolUnavailable(t *testing.T) {
	app, _ := testCopyMenuApp(t)
	clipboard.OSWriter = func(string) error { return errors.New("no clipboard tool available") }
	app.copyToClipboard(keymap.ActionClipboardCopyFilename)
	if !strings.Contains(app.model.Message, "Copy failed") {
		t.Fatalf("message = %q, want copy failure wording", app.model.Message)
	}
}
