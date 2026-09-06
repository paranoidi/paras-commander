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
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: cfg,
		Theme:  th,
		Paths:  config.Paths{ConfigDir: cfgDir},
	})
	if err := app.model.Primary.Load(dir); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestActiveFooterKeysMetaDialogShowsF9EditConfig(t *testing.T) {
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
	app.metaCtrl.OpenDialog(ui.PrimaryPanel)
	if !app.model.MetaDialog.Open {
		t.Fatal("meta dialog should be open")
	}

	keys := app.activeFooterKeys()
	if len(keys) != 3 {
		t.Fatalf("meta dialog footer len = %d, want Esc + F9 + F10", len(keys))
	}
	if keys[1].Key != tcell.KeyF9 || keys[1].Hint != menu.FunctionKeyEditConfig.Hint {
		t.Fatalf("second footer key = %+v, want F9 Edit config", keys[1])
	}
}

func TestMetaDialogF9RefreshesDocumentationBeforeEditor(t *testing.T) {
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
	app.metaCtrl.OpenDialog(ui.PrimaryPanel)

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

	app.metaCtrl.HandleDialogKey(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))

	if !strings.Contains(app.model.Message, "updated documentation") {
		t.Fatalf("Message = %q, want updated documentation notice", app.model.Message)
	}
}

func TestMetaDialogF9ReloadsEntriesAfterEdit(t *testing.T) {
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
	app.metaCtrl.OpenDialog(ui.PrimaryPanel)
	if len(app.model.MetaDialog.Entries) != 1 {
		t.Fatalf("entries len = %d, want lines", len(app.model.MetaDialog.Entries))
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

	app.metaCtrl.HandleDialogKey(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))

	if !app.model.MetaDialog.Open {
		t.Fatal("meta dialog should stay open after F4 edit")
	}
	if len(app.model.MetaDialog.Entries) != 2 {
		t.Fatalf("entries len = %d, want lines + size", len(app.model.MetaDialog.Entries))
	}
	if app.model.MetaDialog.Entries[1].Name != "size" {
		t.Fatalf("second entry = %+v, want size", app.model.MetaDialog.Entries[1])
	}
}

func TestUserMenuDialogF9RefreshesDocumentationBeforeEditor(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	menuBody := `[always]
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
		if !strings.Contains(string(b), "# User function menu") {
			t.Fatalf("canonical menu doc missing before editor:\n%s", b)
		}
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))

	if !strings.Contains(app.model.Message, "updated documentation") {
		t.Fatalf("Message = %q, want updated documentation notice", app.model.Message)
	}
}

func TestUserMenuDialogF9ReloadsEntriesAfterEdit(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	menuBody := `[always]
key = "a"
title = "Always"
command = "true"
`
	if err := os.WriteFile(menuPath, []byte(menuBody), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testUserMenuApp(t, dir, cfgDir)
	app.openUserMenu()
	if len(app.model.LeaderMenu.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(app.model.LeaderMenu.Items))
	}

	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		if path != menuPath {
			t.Fatalf("editor path = %q, want %q", path, menuPath)
		}
		updated := menuBody + `
[also]
key = "b"
title = "Also"
command = "true"
`
		return os.WriteFile(path, []byte(updated), 0o644)
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))

	if !app.model.LeaderMenu.Open {
		t.Fatal("user menu dialog should stay open after F9 edit")
	}
	if len(app.model.LeaderMenu.Items) != 2 {
		t.Fatalf("items len = %d, want 2", len(app.model.LeaderMenu.Items))
	}
	if app.model.LeaderMenu.Items[1].Label != "Also" {
		t.Fatalf("second item = %+v, want Also", app.model.LeaderMenu.Items[1])
	}
}

func TestUserMenuDialogF9ClosesOnInvalidFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	menuPath := filepath.Join(cfgDir, config.DefaultUserMenuFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	menuBody := `[always]
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

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("user menu dialog should close after invalid menu.toml")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyCritical {
		t.Fatalf("MessageUrgency = %v, want critical", app.model.MessageUrgency)
	}
}

func TestMetaDialogF9KeepsCheckedWhenFileInvalid(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	metaPath := filepath.Join(cfgDir, config.DefaultMetaFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaPath, []byte(`[[entry]]
name = "lines"
description = "Line count"
file = "wc -l"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testMetaDialogApp(t, dir, cfgDir)
	app.metaCtrl.OpenDialog(ui.PrimaryPanel)
	app.model.MetaDialog.Checked[0] = true
	app.model.MetaDialog.Focus = 0

	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		return os.WriteFile(path, []byte("not valid meta\n"), 0o644)
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.metaCtrl.HandleDialogKey(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))

	if !app.model.MetaDialog.Open {
		t.Fatal("meta dialog should stay open")
	}
	if len(app.model.MetaDialog.Entries) != 0 {
		t.Fatalf("entries = %+v, want empty after invalid meta.toml", app.model.MetaDialog.Entries)
	}
}
