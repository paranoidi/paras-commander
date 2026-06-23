package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestRunMetaCommand_expandsF(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/my file.txt"
	out, err := runMetaCommand(context.Background(), "echo %f", path, dir)
	if err != nil {
		t.Fatalf("runMetaCommand: %v", err)
	}
	if out != path {
		t.Fatalf("out = %q, want %q", out, path)
	}
}

func TestRunMetaCommand_success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runMetaCommand(context.Background(), "echo hello", dir+"/file", dir)
	if err != nil {
		t.Fatalf("runMetaCommand: %v", err)
	}
	if out != "hello" {
		t.Fatalf("out = %q, want hello", out)
	}
}

func TestRunMetaCommand_failure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := runMetaCommand(context.Background(), "exit 1", dir+"/file", dir)
	if err == nil {
		t.Fatal("expected error for failing command")
	}
}

func TestApplyMetaWakeResult_updatesCorrectColumn(t *testing.T) {
	app := &App{}
	app.model.MetaResults[0] = []ui.MetaColumnState{
		{EntryName: "a", Results: map[string]string{"/p": ""}},
		{EntryName: "b", Results: map[string]string{"/p": ""}},
	}
	app.applyMetaWakeResult(metaWakePayload{panelID: 0, entryName: "b", path: "/p", value: "ok"})
	if got := app.model.MetaResults[0][1].Results["/p"]; got != "ok" {
		t.Fatalf("column b = %q, want ok", got)
	}
	if got := app.model.MetaResults[0][0].Results["/p"]; got != "" {
		t.Fatalf("column a = %q, want empty", got)
	}
}

func TestActivateMetaSelection_preservesCheckedState(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	metaPath := filepath.Join(cfgDir, config.DefaultMetaFileName)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	metaBody := `[[entry]]
name = "mkvinfo"
description = "MKV info"
file = "echo meta"
`
	if err := os.WriteFile(metaPath, []byte(metaBody), 0o644); err != nil {
		t.Fatal(err)
	}

	app := testMetaDialogApp(t, dir, cfgDir)
	app.model.MetaDialog = ui.MetaDialogState{
		Open:    true,
		PanelID: ui.PrimaryPanel,
		Entries: []ui.MetaEntry{{Name: "mkvinfo", Description: "MKV info"}},
		Checked: []bool{true},
		Focus:   0,
	}
	app.activateMetaSelection()

	if len(app.metaActiveEntries[ui.PrimaryPanel]) != 1 || app.metaActiveEntries[ui.PrimaryPanel][0] != "mkvinfo" {
		t.Fatalf("metaActiveEntries = %v, want [mkvinfo]", app.metaActiveEntries[ui.PrimaryPanel])
	}
	if len(app.model.MetaResults[ui.PrimaryPanel]) != 1 {
		t.Fatalf("MetaResults len = %d, want 1", len(app.model.MetaResults[ui.PrimaryPanel]))
	}
	if app.model.MetaResults[ui.PrimaryPanel][0].EntryName != "mkvinfo" {
		t.Fatalf("column = %+v, want mkvinfo", app.model.MetaResults[ui.PrimaryPanel][0])
	}

	app.openMetaDialog(ui.PrimaryPanel)
	if len(app.model.MetaDialog.Checked) != 1 || !app.model.MetaDialog.Checked[0] {
		t.Fatalf("reopen Checked = %v, want [true]", app.model.MetaDialog.Checked)
	}
}

func TestOpenMetaFileEditor_clearsMetaCache(t *testing.T) {
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
	app.metaCache = map[string]map[string]string{
		"lines": {filepath.Join(dir, "file.txt"): "42"},
	}

	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		if path != metaPath {
			t.Fatalf("editor path = %q, want %q", path, metaPath)
		}
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	if !app.openMetaFileEditor(metaPath) {
		t.Fatal("openMetaFileEditor should succeed")
	}
	app.metaCacheMu.RLock()
	empty := len(app.metaCache) == 0
	app.metaCacheMu.RUnlock()
	if !empty {
		t.Fatalf("metaCache = %#v, want cleared", app.metaCache)
	}
}

func TestRunMetaCommand_cancelled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runMetaCommand(ctx, "echo hello", dir+"/file", dir)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
