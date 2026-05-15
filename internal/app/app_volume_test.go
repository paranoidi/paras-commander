package app

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func testAppMinimal(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: config.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	return app
}

func TestApplyVolumeSpaceRefreshStalePathDropped(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	app.model.Left.Path = "/panel/current"
	app.model.Left.VolumeAvailBytes = 100
	app.model.Left.VolumeTotalBytes = 200
	app.model.Left.VolumeSpaceOK = true

	changed := app.applyVolumeSpaceRefresh(volumeSpaceRefreshPayload{
		PanelID: ui.LeftPanel,
		Path:    "/panel/old",
		Avail:   50,
		Total:   100,
		OK:      true,
	})
	if changed {
		t.Fatal("stale path should not apply")
	}
	if app.model.Left.VolumeAvailBytes != 100 {
		t.Fatalf("avail = %d, want 100", app.model.Left.VolumeAvailBytes)
	}
}

func TestApplyVolumeSpaceRefreshUpdatesPanel(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	path := filepath.Clean(app.model.Left.Path)
	changed := app.applyVolumeSpaceRefresh(volumeSpaceRefreshPayload{
		PanelID: ui.LeftPanel,
		Path:    path,
		Avail:   42,
		Total:   100,
		OK:      true,
	})
	if !changed {
		t.Fatal("expected change on first apply")
	}
	if app.model.Left.VolumeAvailBytes != 42 {
		t.Fatalf("avail = %d, want 42", app.model.Left.VolumeAvailBytes)
	}
	again := app.applyVolumeSpaceRefresh(volumeSpaceRefreshPayload{
		PanelID: ui.LeftPanel,
		Path:    path,
		Avail:   42,
		Total:   100,
		OK:      true,
	})
	if again {
		t.Fatal("identical values should not report change")
	}
}
