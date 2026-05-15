package ui

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func TestPanelTouchedByJobs(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	nasRoot := filepath.Join(tmp, "nas", "media")
	local := filepath.Join(tmp, "local")
	marks := []JobPathMark{{
		Type:        string(jobs.TypeCopy),
		Status:      string(jobs.StatusRunning),
		Sources:     []string{nasRoot},
		Destination: local,
		DestIsDir:   true,
	}}

	if !PanelTouchedByJobs(nasRoot, marks) {
		t.Fatal("panel at source root should be touched")
	}
	if !PanelTouchedByJobs(filepath.Join(nasRoot, "a", "b"), marks) {
		t.Fatal("panel under source should be touched")
	}
	if !PanelTouchedByJobs(local, marks) {
		t.Fatal("panel at destination should be touched")
	}
	if PanelTouchedByJobs(filepath.Join(tmp, "elsewhere"), marks) {
		t.Fatal("unrelated panel should not be touched")
	}
	finished := []JobPathMark{{
		Status:  string(jobs.StatusCompleted),
		Sources: []string{nasRoot},
	}}
	if PanelTouchedByJobs(nasRoot, finished) {
		t.Fatal("finished job should not touch panel")
	}
}
