package jobbridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/oplog"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TestCopyRecordsEveryFile copies many tiny files in one throttled burst (the case where
// progress samples named only a few of them) and expects one done row per file, plus a failed
// row for the unreadable file and skipped rows for files the destination already has.
func TestCopyRecordsEveryFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src, dstDir := filepath.Join(dir, "meadow"), filepath.Join(dir, "harbor")
	if err := os.MkdirAll(filepath.Join(dstDir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	const files = 300
	for i := range files {
		if err := os.WriteFile(filepath.Join(src, fmt.Sprintf("pebble-%03d.txt", i)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"pebble-000.txt", "pebble-001.txt"} {
		if err := os.WriteFile(filepath.Join(dstDir, "meadow", name), []byte("other"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	locked := filepath.Join(src, "locked.dat")
	if err := os.WriteFile(locked, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	if f, err := os.Open(locked); err == nil {
		_ = f.Close()
		t.Skip("running with permissions that can read a mode-000 file")
	}
	cfg := config.Default()
	transfer := TransferFunc(cfg.Operations, cfg.Jobs, 4, nil, nil, nil)
	job := &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeCopy, Status: jobs.StatusRunning,
		Sources:     []pathloc.Path{pathloc.FileMust(src)},
		Destination: pathloc.FileMust(dstDir),
	}
	job.Ops = oplog.New(job.ID, oplog.Config{})
	defer job.Ops.Close()
	skip := jobs.BlockerAnswer{Decision: jobs.DecisionSkipAll}
	if err := transfer(context.Background(), job, func(jobs.Event) {}, func(jobs.BlockerRequest) jobs.BlockerAnswer { return skip }); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	st := job.Ops.Stats()
	if st.Done != files-2 || st.Skipped != 2 || st.Failed != 1 {
		t.Fatalf("stats = %+v, want %d done, 2 skipped, 1 failed", st, files-2)
	}
	recs, err := job.Ops.ReadRecords(job.Ops.ViewNums(1<<oplog.Skipped, 0, 2))
	if err != nil {
		t.Fatal(err)
	}
	if recs[0].Label != "meadow/pebble-001.txt" || recs[0].Reason != "exists" {
		t.Fatalf("skipped row = %+v, want a destination-relative label", recs[0])
	}
}
