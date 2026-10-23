package jobbridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// journalEvent runs a copy of lantern into a directory that may already hold a lantern and
// returns the EventUndoJournal the transfer emitted (nil when none).
func journalEvent(t *testing.T, preexisting bool, answer jobs.BlockerAnswer) *jobs.Event {
	t.Helper()
	dir := t.TempDir()
	src, dstDir := filepath.Join(dir, "lantern"), filepath.Join(dir, "harbor")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if preexisting {
		if err := os.WriteFile(filepath.Join(dstDir, "lantern"), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	transfer := TransferFunc(cfg.Operations, cfg.Jobs, 4, nil, nil, nil)
	job := &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeCopy, Status: jobs.StatusRunning,
		Sources:     []pathloc.Path{pathloc.FileMust(src)},
		Destination: pathloc.FileMust(dstDir),
	}
	var got *jobs.Event
	err := transfer(context.Background(), job, func(ev jobs.Event) {
		if ev.Type == jobs.EventUndoJournal {
			got = &ev
		}
	}, func(jobs.BlockerRequest) jobs.BlockerAnswer { return answer })
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	return got
}

func TestCopyEmitsUndoJournal(t *testing.T) {
	t.Parallel()
	ev := journalEvent(t, false, jobs.BlockerAnswer{})
	if ev == nil || ev.Irreversible || len(ev.Undo) != 1 {
		t.Fatalf("want one reversible step, got %+v", ev)
	}
}

func TestOverwriteMarksJobIrreversible(t *testing.T) {
	t.Parallel()
	ev := journalEvent(t, true, jobs.BlockerAnswer{Decision: jobs.DecisionOverwrite})
	if ev == nil || !ev.Irreversible {
		t.Fatalf("overwrite must mark the journal irreversible, got %+v", ev)
	}
}

func TestSkippedConflictStaysReversible(t *testing.T) {
	t.Parallel()
	if ev := journalEvent(t, true, jobs.BlockerAnswer{Decision: jobs.DecisionSkip}); ev != nil {
		t.Fatalf("nothing written and nothing overwritten, got %+v", ev)
	}
}

// waitFinished applies worker events until job id finishes and returns its terminal event type.
func waitFinished(t *testing.T, s *jobs.State, id string) jobs.EventType {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case ev := <-s.Events():
			s.ApplyEvent(ev)
			if ev.JobID == id && (ev.Type == jobs.EventCompleted || ev.Type == jobs.EventFailed || ev.Type == jobs.EventCanceled) {
				return ev.Type
			}
		case <-deadline:
			t.Fatalf("timeout waiting for job %s", id)
		}
	}
}

func TestCopyThenUndoThroughWorker(t *testing.T) {
	dir := t.TempDir()
	src, dstDir := filepath.Join(dir, "lantern"), filepath.Join(dir, "harbor")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("glow"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := jobs.NewState()
	cfg := config.Default()
	s.SetTransferFunc(TransferFunc(cfg.Operations, cfg.Jobs, cfg.Dedup.ChunkBytes, nil, nil, nil))
	stop := make(chan struct{})
	defer close(stop)
	s.StartWorker(stop)

	job := &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeCopy, Status: jobs.StatusQueued,
		Sources: []pathloc.Path{pathloc.FileMust(src)}, Destination: pathloc.FileMust(dstDir), DestIsDir: true,
	}
	s.AddJob(job)
	if got := waitFinished(t, s, job.ID); got != jobs.EventCompleted {
		t.Fatalf("copy ended %s", got)
	}
	copied := filepath.Join(dstDir, "lantern")
	if _, err := os.Stat(copied); err != nil {
		t.Fatal(err)
	}
	undo, ok := s.EnqueueUndo(job.ID)
	if !ok {
		t.Fatal("completed copy should be undoable")
	}
	if got := waitFinished(t, s, undo.ID); got != jobs.EventCompleted {
		t.Fatalf("undo ended %s", got)
	}
	if _, err := os.Lstat(copied); !os.IsNotExist(err) {
		t.Fatalf("copy should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatal("source must remain")
	}
}
