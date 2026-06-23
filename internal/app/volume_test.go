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
	app.model.Primary.Path = pathloc.MustParse("/panel/current")
	app.model.Primary.VolumeAvailBytes = 100
	app.model.Primary.VolumeTotalBytes = 200
	app.model.Primary.VolumeSpaceOK = true

	changed := app.applyVolumeSpaceRefresh(volumeSpaceRefreshPayload{
		PanelID: ui.PrimaryPanel,
		Path:    "/panel/old",
		Avail:   50,
		Total:   100,
		OK:      true,
	})
	if changed {
		t.Fatal("stale path should not apply")
	}
	if app.model.Primary.VolumeAvailBytes != 100 {
		t.Fatalf("avail = %d, want 100", app.model.Primary.VolumeAvailBytes)
	}
}

func TestApplyVolumeSpaceRefreshUpdatesPanel(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	path := filepath.Clean(app.model.Primary.Path.String())
	changed := app.applyVolumeSpaceRefresh(volumeSpaceRefreshPayload{
		PanelID: ui.PrimaryPanel,
		Path:    path,
		Avail:   42,
		Total:   100,
		OK:      true,
	})
	if !changed {
		t.Fatal("expected change on first apply")
	}
	if app.model.Primary.VolumeAvailBytes != 42 {
		t.Fatalf("avail = %d, want 42", app.model.Primary.VolumeAvailBytes)
	}
	again := app.applyVolumeSpaceRefresh(volumeSpaceRefreshPayload{
		PanelID: ui.PrimaryPanel,
		Path:    path,
		Avail:   42,
		Total:   100,
		OK:      true,
	})
	if again {
		t.Fatal("identical values should not report change")
	}
}
