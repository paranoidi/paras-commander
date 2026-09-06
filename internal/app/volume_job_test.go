package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/pathloc"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func TestJobVolumeDevsSameDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	panel := filepath.Join(dir, "browse")
	src := filepath.Join(dir, "file.dat")
	dst := filepath.Join(dir, "dst")
	if err := os.MkdirAll(panel, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	job := &jobs.Job{
		Sources:     pathloc.PathsForTest(src),
		Destination: pathloc.MustParse(dst),
	}
	job.ComputeVolumeDevs()
	dev, ok := diskusage.PathDevice(panel)
	if !ok {
		t.Fatal("PathDevice failed for panel dir")
	}
	if !job.HasVolumeDev(dev) {
		t.Fatal("panel on same volume as job sources should conflict")
	}
}

func TestJobVolumeDevsUseContainingDirectories(t *testing.T) {
	t.Parallel()
	other := t.TempDir()
	job := &jobs.Job{
		Sources:     pathloc.PathsForTest(filepath.Join(other, "ghost.dat")),
		Destination: pathloc.MustParse(filepath.Join(other, "dst")),
	}
	job.ComputeVolumeDevs()
	dev, ok := diskusage.PathDevice(other)
	if !ok {
		t.Fatal("PathDevice failed for temp dir")
	}
	// Sources are resolved through their parent directory (one stat per distinct directory
	// instead of one per source), so a source that does not exist yet still contributes the
	// volume it would live on.
	if !job.HasVolumeDev(dev) {
		t.Fatalf("source's containing directory volume should be cached, got %v", job.VolumeDevs)
	}
}

func TestJobVolumeDevsMissingParentCachesNothing(t *testing.T) {
	t.Parallel()
	gone := filepath.Join(t.TempDir(), "absent")
	job := &jobs.Job{
		Sources:     pathloc.PathsForTest(filepath.Join(gone, "sub", "ghost.dat")),
		Destination: pathloc.MustParse(filepath.Join(gone, "sub", "dst")),
	}
	job.ComputeVolumeDevs()
	if len(job.VolumeDevs) != 0 {
		t.Fatalf("job paths under a nonexistent directory must cache no volume devs, got %v", job.VolumeDevs)
	}
}
