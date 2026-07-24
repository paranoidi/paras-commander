package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

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
	app.model.MetaDialog = dialog.MetaDialogState{
		Open:    true,
		PanelID: ui.PrimaryPanel,
		Entries: []dialog.MetaEntry{{Name: "mkvinfo", Description: "MKV info"}},
		Checked: []bool{true},
		Focus:   0,
	}
	app.metaCtrl.ActivateSelection()

	if len(app.model.MetaResults[ui.PrimaryPanel]) != 1 {
		t.Fatalf("MetaResults len = %d, want 1", len(app.model.MetaResults[ui.PrimaryPanel]))
	}
	if app.model.MetaResults[ui.PrimaryPanel][0].EntryName != "mkvinfo" {
		t.Fatalf("column = %+v, want mkvinfo", app.model.MetaResults[ui.PrimaryPanel][0])
	}

	app.metaCtrl.OpenDialog(ui.PrimaryPanel)
	if len(app.model.MetaDialog.Checked) != 1 || !app.model.MetaDialog.Checked[0] {
		t.Fatalf("reopen Checked = %v, want [true]", app.model.MetaDialog.Checked)
	}
}

func TestOpenMetaFileEditor_succeeds(t *testing.T) {
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

	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		if path != metaPath {
			t.Fatalf("editor path = %q, want %q", path, metaPath)
		}
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	if !app.metaCtrl.OpenFileEditor(metaPath) {
		t.Fatal("OpenFileEditor should succeed")
	}
}
