package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func TestPanelSharesVolumeWithJobSameDirectory(t *testing.T) {
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
		Sources:     []string{src},
		Destination: dst,
	}
	if !panelSharesVolumeWithJob(panel, job) {
		t.Fatal("panel on same volume as job sources should conflict")
	}
}

func TestPanelSharesVolumeWithJobDifferentTempDirs(t *testing.T) {
	t.Parallel()
	panel := t.TempDir()
	other := t.TempDir()
	job := &jobs.Job{
		Sources:     []string{filepath.Join(other, "file.dat")},
		Destination: filepath.Join(other, "dst"),
	}
	if panelSharesVolumeWithJob(panel, job) {
		t.Fatal("unrelated temp dirs should not share volume conflict on typical setups")
	}
}
