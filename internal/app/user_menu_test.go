package app

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

func testUserMenuApp(t *testing.T, dir, cfgDir string) *App {
	t.Helper()
	th, _ := loadTestTheme(t)
	cfg := config.Default()
	cfg.UserMenu = config.UserMenuConfig{LocalNames: []string{"menu.toml"}}
	screen := newScreen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: cfg,
		Theme:  th,
		Paths:  config.Paths{ConfigDir: cfgDir},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.model.Left.Load(dir); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestOpenUserMenuCreatesStubWhenMissing(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	app := testUserMenuApp(t, dir, cfgDir)

	var edited []string
	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		edited = append(edited, path)
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.openUserMenu()

	global := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if _, err := os.Stat(global); err != nil {
		t.Fatalf("stub not created: %v", err)
	}
	if len(edited) != 1 || edited[0] != global {
		t.Fatalf("edited = %v, want [%q]", edited, global)
	}
	if app.model.UserMenu.Open {
		t.Fatal("user menu dialog should not open during bootstrap")
	}
}

func TestEditUserMenuOpensExistingPath(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	menuBody := `[[entry]]
key = "a"
title = "Always"
command = "true"
`
	if err := os.WriteFile(menuPath, []byte(menuBody), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testUserMenuApp(t, dir, cfgDir)

	var edited []string
	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		edited = append(edited, path)
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.editUserMenu()

	if len(edited) != 1 || edited[0] != menuPath {
		t.Fatalf("edited = %v, want [%q]", edited, menuPath)
	}
	if app.model.UserMenu.Open {
		t.Fatal("edit user menu should not open dialog")
	}
}

func TestOpenUserMenuOpensDialogWhenEntriesExist(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(menuPath, []byte(`[[entry]]
key = "a"
title = "Always"
command = "true"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testUserMenuApp(t, dir, cfgDir)

	prev := externalEditorRunner
	var mu sync.Mutex
	called := false
	externalEditorRunner = func(_ context.Context, path string) error {
		mu.Lock()
		called = true
		mu.Unlock()
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.openUserMenu()

	if called {
		t.Fatal("editor should not run when menu has entries")
	}
	if !app.model.UserMenu.Open {
		t.Fatal("user menu dialog should open")
	}
}

func TestShiftF2MapsToEditUserMenu(t *testing.T) {
	km := defaultKeymap(t)
	got := lookupActionForView(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModShift), km, nil, nil, nil, ui.ViewBrowser)
	if got != keymap.ActionAppUserMenuEdit {
		t.Fatalf("got %q, want %q", got, keymap.ActionAppUserMenuEdit)
	}
}

func TestWriteMenuStubMatchesPackage(t *testing.T) {
	if usermenu.MenuStubTOML == "" {
		t.Fatal("MenuStubTOML empty")
	}
}
