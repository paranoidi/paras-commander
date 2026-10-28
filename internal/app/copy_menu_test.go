package app

import (
	"errors"
	"os"
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
	if len(app.model.LeaderMenu.Items) != 6 {
		t.Fatalf("items len = %d, want 6", len(app.model.LeaderMenu.Items))
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

func gitMenuItem(app *App) ui.LeaderMenuItem {
	for _, it := range app.model.LeaderMenu.Items {
		if it.Key == 'g' {
			return it
		}
	}
	return ui.LeaderMenuItem{}
}

// testCopyMenuGitApp is testCopyMenuApp with the file moved into a
// subdirectory (the panel's cwd) of a fake git work tree.
func testCopyMenuGitApp(t *testing.T) *App {
	t.Helper()
	clipboard.Reset()
	prevWriter := clipboard.OSWriter
	clipboard.OSWriter = func(string) error { return nil }
	t.Cleanup(func() { clipboard.OSWriter = prevWriter })
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".git", "HEAD"))
	sub := filepath.Join(root, "orchard", "lantern")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "meadow.txt"))
	app := newTestApp(t, newScreen(t, 80, 50), Options{
		CWD:    func() (string, error) { return sub, nil },
		Config: config.Default(),
		Paths:  config.Paths{ConfigDir: filepath.Join(root, "config")},
	})
	app.model.ViewMode = ui.ViewBrowser
	for i := 0; i < app.model.Primary.VisibleEntryCount(); i++ {
		if e, _, ok := app.model.Primary.VisibleEntry(i); ok && e.Name == "meadow.txt" {
			app.model.Primary.Cursor = i
			break
		}
	}
	return app
}

func TestCopyMenuGitRowDisabledOutsideRepo(t *testing.T) {
	app, _ := testCopyMenuApp(t)
	app.toggleCopyMenu()
	it := gitMenuItem(app)
	if it.Key != 'g' || !it.Disabled {
		t.Fatalf("expected disabled g row outside a work tree, got %+v", it)
	}
	app.dispatch(keymap.ActionClipboardGitMenu)
	if app.model.LeaderMenu.CopyGit {
		t.Fatal("git submenu must not open outside a work tree")
	}
}

func TestCopyMenuGitSubmenuInsideRepo(t *testing.T) {
	app := testCopyMenuGitApp(t)
	app.toggleCopyMenu()
	if gitMenuItem(app).Disabled {
		t.Fatal("g row should be enabled inside a work tree")
	}
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))
	st := app.model.LeaderMenu
	if !st.Open || !st.CopyMenu || !st.CopyGit || len(st.Items) != 2 {
		t.Fatalf("expected open git submenu with 2 rows, got %+v", st)
	}
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))
	if got, want := clipboard.LastSet(), filepath.Join("orchard", "lantern", "meadow.txt"); got != want {
		t.Fatalf("clipboard = %q, want %q", got, want)
	}

	app.toggleCopyMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	if got, want := clipboard.LastSet(), filepath.Join("orchard", "lantern"); got != want {
		t.Fatalf("clipboard = %q, want %q", got, want)
	}
}

func TestCopyMenuGitSubmenuLeftReturnsToTop(t *testing.T) {
	for _, vi := range []bool{false, true} {
		app := testCopyMenuGitApp(t)
		app.model.ViMotionMode = vi
		app.toggleCopyMenu()
		app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))
		ev := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
		if vi {
			ev = tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone)
		}
		app.handleLeaderMenuKey(ev)
		st := app.model.LeaderMenu
		if !st.Open || !st.CopyMenu || st.CopyGit || len(st.Items) != 6 {
			t.Fatalf("vi=%v: expected top copy menu, got %+v", vi, st)
		}
	}
}

func TestCopyMenuLeftCloses(t *testing.T) {
	for _, vi := range []bool{false, true} {
		app, _ := testCopyMenuApp(t)
		app.model.ViMotionMode = vi
		app.toggleCopyMenu()
		ev := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)
		if vi {
			ev = tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone)
		}
		app.handleLeaderMenuKey(ev)
		if app.model.LeaderMenu.Open {
			t.Fatalf("vi=%v: copy menu still open after Left", vi)
		}
	}
}

func copyMenuItemByKey(app *App, key rune) ui.LeaderMenuItem {
	for _, it := range app.model.LeaderMenu.Items {
		if it.Key == key {
			return it
		}
	}
	return ui.LeaderMenuItem{}
}

func TestCopyMenuFileContentsOnlyInFullscreenTextPreview(t *testing.T) {
	app, filePath := testCopyMenuApp(t)
	if err := os.WriteFile(filePath, []byte("lantern harbor\nquiet orchard\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.toggleCopyMenu()
	if !copyMenuItemByKey(app, 'o').Disabled {
		t.Fatal("file contents row should be dimmed in the browser")
	}
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone))
	if got := clipboard.LastSet(); got != "" {
		t.Fatalf("clipboard = %q, want empty outside preview", got)
	}

	app.model.ViewMode = ui.ViewFilePreview
	app.model.FullscreenFilePreview.Path = filePath
	app.toggleCopyMenu()
	if copyMenuItemByKey(app, 'o').Disabled {
		t.Fatal("file contents row should be enabled in fullscreen text preview")
	}
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone))
	if got := clipboard.LastSet(); got != "lantern harbor\nquiet orchard\n" {
		t.Fatalf("clipboard = %q", got)
	}

	clipboard.Reset()
	if err := os.WriteFile(filePath, []byte("pebble\x00crane"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.copyToClipboard(keymap.ActionClipboardCopyFileContents)
	if got := clipboard.LastSet(); got != "" {
		t.Fatalf("binary file copied: %q", got)
	}
}
