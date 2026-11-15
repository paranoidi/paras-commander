package app

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestActiveFooterKeysFilterDialogSavedPatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lantern.txt"))
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)
	app := newTestApp(t, screen, Options{CWD: func() (string, error) { return dir, nil }, Config: config.Default()})

	app.dialogCtrl.OpenFilterDialog()
	hints := func() map[string]string {
		out := map[string]string{}
		for _, k := range app.activeFooterKeys() {
			out[k.Hint] = k.KeyLabel
		}
		return out
	}
	got := hints()
	for hint, key := range map[string]string{"Load": "F2", "History": "F3", "Save": "F5"} {
		if got[hint] != key {
			t.Fatalf("main screen footer = %v, want %s on %s", got, hint, key)
		}
	}

	app.dialogCtrl.HandleFilterDialogKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if app.model.FilterDialog.Phase != dialog.FilterPhaseSavePrompt {
		t.Fatalf("phase = %v, want save prompt", app.model.FilterDialog.Phase)
	}
	if got := hints(); got["Overwrite"] != "F5" || got["Load"] != "" {
		t.Fatalf("save prompt footer = %v, want only Overwrite", got)
	}
}
