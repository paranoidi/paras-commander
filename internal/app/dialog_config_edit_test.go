package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/configdoc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func testMetaDialogApp(t *testing.T, dir, cfgDir string) *App {
	t.Helper()
	th, _ := loadTestTheme(t)
	cfg := config.Default()
	cfg.Meta = config.MetaConfig{LocalNames: []string{"meta.toml"}}
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

func TestActiveFooterKeysMetaDialogShowsF4EditConfig(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	metaPath := filepath.Join(cfgDir, config.DefaultMetaFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	metaBody := `[[entry]]
name = "lines"
description = "Line count"
file = "wc -l"
`
	if err := os.WriteFile(metaPath, []byte(metaBody), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testMetaDialogApp(t, dir, cfgDir)
	app.openMetaDialog(ui.LeftPanel)
	if !app.model.MetaDialog.Open {
		t.Fatal("meta dialog should be open")
	}

	keys := app.activeFooterKeys()
	if len(keys) != 3 {
		t.Fatalf("meta dialog footer len = %d, want Esc + F4 + F10", len(keys))
	}
	if keys[1].Key != tcell.KeyF4 || keys[1].Hint != menu.FunctionKeyEditConfig.Hint {
		t.Fatalf("second footer key = %+v, want F4 Edit config", keys[1])
	}
}

func TestMetaDialogF4RefreshesDocumentationBeforeEditor(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	metaPath := filepath.Join(cfgDir, config.DefaultMetaFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	metaBody := `[[entry]]
name = "lines"
description = "Line count"
file = "wc -l"
`
	if err := os.WriteFile(metaPath, []byte(metaBody), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testMetaDialogApp(t, dir, cfgDir)
	app.openMetaDialog(ui.LeftPanel)

	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		if path != metaPath {
			t.Fatalf("editor path = %q, want %q", path, metaPath)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), configdoc.DocEndSentinel) {
			t.Fatalf("file missing doc sentinel before editor:\n%s", b)
		}
		if !strings.Contains(string(b), metaBody) {
			t.Fatalf("user body not preserved before editor:\n%s", b)
		}
		if !strings.Contains(string(b), "# meta.toml") {
			t.Fatalf("canonical meta doc missing before editor:\n%s", b)
		}
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.handleMetaDialogKey(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))

	if !strings.Contains(app.model.Message, "updated documentation") {
		t.Fatalf("Message = %q, want updated documentation notice", app.model.Message)
	}
}

func TestMetaDialogF4ReloadsEntriesAfterEdit(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	metaPath := filepath.Join(cfgDir, config.DefaultMetaFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	metaBody := `[[entry]]
name = "lines"
description = "Line count"
file = "wc -l"
`
	if err := os.WriteFile(metaPath, []byte(metaBody), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testMetaDialogApp(t, dir, cfgDir)
	app.openMetaDialog(ui.LeftPanel)
	if len(app.model.MetaDialog.Entries) != 2 {
		t.Fatalf("entries len = %d, want none + lines", len(app.model.MetaDialog.Entries))
	}

	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		if path != metaPath {
			t.Fatalf("editor path = %q, want %q", path, metaPath)
		}
		updated := metaBody + `
[[entry]]
name = "size"
description = "Byte size"
file = "wc -c"
`
		return os.WriteFile(path, []byte(updated), 0o644)
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.handleMetaDialogKey(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))

	if !app.model.MetaDialog.Open {
		t.Fatal("meta dialog should stay open after F4 edit")
	}
	if len(app.model.MetaDialog.Entries) != 3 {
		t.Fatalf("entries len = %d, want none + lines + size", len(app.model.MetaDialog.Entries))
	}
	if app.model.MetaDialog.Entries[2].Name != "size" {
		t.Fatalf("third entry = %+v, want size", app.model.MetaDialog.Entries[2])
	}
}

func TestUserMenuDialogF4RefreshesDocumentationBeforeEditor(t *testing.T) {
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
	app.openUserMenu()

	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		if path != menuPath {
			t.Fatalf("editor path = %q, want %q", path, menuPath)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), configdoc.DocEndSentinel) {
			t.Fatalf("file missing doc sentinel before editor:\n%s", b)
		}
		if !strings.Contains(string(b), menuBody) {
			t.Fatalf("user body not preserved before editor:\n%s", b)
		}
		if !strings.Contains(string(b), "# F2 user menu") {
			t.Fatalf("canonical menu doc missing before editor:\n%s", b)
		}
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.handleUserMenuDialogKey(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))

	if !strings.Contains(app.model.Message, "updated documentation") {
		t.Fatalf("Message = %q, want updated documentation notice", app.model.Message)
	}
}

func TestUserMenuDialogF4ReloadsEntriesAfterEdit(t *testing.T) {
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
	app.openUserMenu()
	if len(app.model.UserMenu.Entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(app.model.UserMenu.Entries))
	}
	app.model.UserMenu.Selected = 0
	app.model.UserMenu.Focus = 0

	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		if path != menuPath {
			t.Fatalf("editor path = %q, want %q", path, menuPath)
		}
		updated := menuBody + `
[[entry]]
key = "b"
title = "Also"
command = "true"
`
		return os.WriteFile(path, []byte(updated), 0o644)
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.handleUserMenuDialogKey(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))

	if !app.model.UserMenu.Open {
		t.Fatal("user menu dialog should stay open after F4 edit")
	}
	if len(app.model.UserMenu.Entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(app.model.UserMenu.Entries))
	}
	if app.model.UserMenu.Entries[1].Title != "Also" {
		t.Fatalf("second entry = %+v, want Also", app.model.UserMenu.Entries[1])
	}
}

func TestUserMenuDialogF4ClosesOnInvalidFile(t *testing.T) {
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
	app.openUserMenu()

	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		return os.WriteFile(path, []byte("shell_patterns = []\n"), 0o644)
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.handleUserMenuDialogKey(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))

	if app.model.UserMenu.Open {
		t.Fatal("user menu dialog should close after invalid menu.toml")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyCritical {
		t.Fatalf("MessageUrgency = %v, want critical", app.model.MessageUrgency)
	}
}

func TestMetaDialogF4KeepsNoneWhenFileInvalid(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	metaPath := filepath.Join(cfgDir, config.DefaultMetaFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaPath, []byte(`[[entry]]
name = "lines"
file = "wc -l"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testMetaDialogApp(t, dir, cfgDir)
	app.openMetaDialog(ui.LeftPanel)
	app.model.MetaDialog.Selected = 1
	app.model.MetaDialog.Focus = 1

	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		return os.WriteFile(path, []byte("not valid meta\n"), 0o644)
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.handleMetaDialogKey(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))

	if !app.model.MetaDialog.Open {
		t.Fatal("meta dialog should stay open")
	}
	if len(app.model.MetaDialog.Entries) != 1 || app.model.MetaDialog.Entries[0].Name != "none" {
		t.Fatalf("entries = %+v, want only none after invalid meta.toml", app.model.MetaDialog.Entries)
	}
}
