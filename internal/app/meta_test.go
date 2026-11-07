package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	metactrl "github.com/paranoidi/paras-commander/internal/apphandler/meta"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
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
	}
	app.metaCtrl.ActivateSelection()

	if len(app.model.MetaResults[ui.PrimaryPanel]) != 1 {
		t.Fatalf("MetaResults len = %d, want 1", len(app.model.MetaResults[ui.PrimaryPanel]))
	}
	if app.model.MetaResults[ui.PrimaryPanel][0].EntryName != "mkvinfo" {
		t.Fatalf("column = %+v, want mkvinfo", app.model.MetaResults[ui.PrimaryPanel][0])
	}

	app.metaCtrl.OpenDialog(ui.PrimaryPanel)
	if c := app.model.MetaDialog.Checked; len(c) != 1 || !c[0] {
		t.Fatalf("reopen Checked = %v, want [true]", c)
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

	app.runExternalEditor = func(_ context.Context, path string, _ bool) error {
		if path != metaPath {
			t.Fatalf("editor path = %q, want %q", path, metaPath)
		}
		return nil
	}

	if !app.metaCtrl.OpenFileEditor(metaPath) {
		t.Fatal("OpenFileEditor should succeed")
	}
}

func TestMetaResultWideningColumnToastsLostColumn(t *testing.T) {
	app := testAppMinimal(t)
	sim, ok := app.screen.(tcell.SimulationScreen)
	if !ok {
		t.Fatal("test app screen is not a simulation screen")
	}
	sim.SetSize(120, 30)
	app.config.UI.Zoom.ActivePanel = false
	app.model.UseNerdfontIcons = false
	app.model.Primary.ListFormat = panel.ListFormatMtime
	// A 60-cell panel leaves 34 cells for name and meta: two 5-cell titles leave exactly 20.
	const path = "/harbor/lantern"
	app.model.MetaResults[ui.PrimaryPanel] = []ui.MetaColumnState{
		{EntryName: "heron", ColumnTitle: "Heron", ResultsGen: 1, Results: map[string]string{path: "a"}},
		{EntryName: "otter", ColumnTitle: "Otter", ResultsGen: 1, Results: map[string]string{path: "b"}},
	}
	app.metaCtrl.HandleRenderFlush()
	app.model.Message = ""

	app.metaCtrl.HandleWake(metactrl.WakePayload{PanelID: ui.PrimaryPanel, EntryName: "otter", Path: path, Value: "wide-pebble"})
	app.metaCtrl.HandleRenderFlush()
	if want := "Column Otter no longer fits on available space"; app.model.Message != want {
		t.Fatalf("message = %q, want %q", app.model.Message, want)
	}
}
