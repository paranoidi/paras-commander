package app

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestApplyVolumeSpaceRefreshStalePathDropped(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	app.model.Left.Path = pathloc.MustParse("/panel/current")
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
	path := filepath.Clean(app.model.Left.Path.String())
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
