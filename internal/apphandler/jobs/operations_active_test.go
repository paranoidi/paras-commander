package jobs

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/oplog"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// TestOpsActiveLabel: the Operations pane names the item in progress, worded like its record,
// only while the job transfers and nothing narrows the rows.
func TestOpsActiveLabel(t *testing.T) {
	t.Parallel()
	log := oplog.New("thicket", oplog.Config{Sources: 2})
	t.Cleanup(log.Close)
	log.Append(oplog.Record{Result: oplog.Done, Label: "lantern/acorn.txt", Source: 0})
	running := ui.JobEntry{
		Status:          string(jobs.StatusRunning),
		Destination:     "/meadow",
		CurrentPath:     "/harbor/lantern/bramble.bin",
		CurrentDestPath: "/meadow/lantern/bramble.bin",
		SourcePhases:    []jobs.SourcePhase{jobs.SourceStarted, jobs.SourceQueued},
	}
	all := opsViewKey{log: log, mask: oplog.MaskAll, source: oplog.NoSource}
	if got := opsActiveLabel(running, all); got != "lantern/bramble.bin" {
		t.Fatalf("label = %q, want the destination-relative label", got)
	}
	cases := []struct {
		name string
		edit func(*ui.JobEntry, *opsViewKey)
	}{
		{"paused", func(e *ui.JobEntry, _ *opsViewKey) { e.Status = string(jobs.StatusPaused) }},
		{"scanning", func(e *ui.JobEntry, _ *opsViewKey) { e.Status = string(jobs.StatusScanning) }},
		{"fuzzy query", func(_ *ui.JobEntry, k *opsViewKey) { k.query = "acorn" }},
		{"result filter", func(_ *ui.JobEntry, k *opsViewKey) { k.mask = 1 << oplog.Failed }},
		{"other source selected", func(_ *ui.JobEntry, k *opsViewKey) { k.source = 1 }},
		{"just finished", func(e *ui.JobEntry, _ *opsViewKey) {
			e.CurrentPath, e.CurrentDestPath = "/harbor/lantern/acorn.txt", "/meadow/lantern/acorn.txt"
		}},
	}
	for _, tc := range cases {
		e, k := running, all
		tc.edit(&e, &k)
		if got := opsActiveLabel(e, k); got != "" {
			t.Errorf("%s: label = %q, want none", tc.name, got)
		}
	}
	k := all
	k.source = 0
	if got := opsActiveLabel(running, k); got != "lantern/bramble.bin" {
		t.Errorf("active source selected: label = %q", got)
	}
	del := ui.JobEntry{Status: string(jobs.StatusRunning), CurrentPath: "/harbor/thistle"}
	if got := opsActiveLabel(del, all); got != "/harbor/thistle" {
		t.Errorf("delete label = %q, want the source path like its records", got)
	}
}
