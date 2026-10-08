package jobbridge

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestEventUpdatesMarks(t *testing.T) {
	t.Parallel()
	if !EventUpdatesMarks(jobs.EventJobResumed) {
		t.Fatal("EventJobResumed should refresh path marks")
	}
	if EventUpdatesMarks(jobs.EventProgress) {
		t.Fatal("EventProgress should not refresh path marks")
	}
}

func TestUniqueParents(t *testing.T) {
	t.Parallel()
	paths, err := pathloc.ParseAll([]string{"/tmp/a/x.txt", "/tmp/a/y.txt", "/tmp/b/z.txt"})
	if err != nil {
		t.Fatal(err)
	}
	got := uniqueParents(paths)
	if len(got) != 2 {
		t.Fatalf("uniqueParents = %d entries, want 2", len(got))
	}
	if got[0].String() != "/tmp/a" || got[1].String() != "/tmp/b" {
		t.Fatalf("uniqueParents = %v, want [/tmp/a /tmp/b]", got)
	}
}

func TestTransferFinalTotalsSurviveFullEventChannel(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "willow.txt")
	payload := []byte("hello world")
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	srcLoc, err := pathloc.File(src)
	if err != nil {
		t.Fatal(err)
	}

	s := jobs.NewState()
	cfg := config.Default()
	s.SetTransferFunc(TransferFunc(cfg.Operations, cfg.Jobs, cfg.Dedup.ChunkBytes, nil))
	stop := make(chan struct{})
	defer close(stop)

	job := &jobs.Job{
		ID:         "del-1",
		Type:       jobs.TypeDelete,
		Status:     jobs.StatusQueued,
		Sources:    []pathloc.Path{srcLoc},
		TotalFiles: 1,
	}

	errCh := make(chan string, 1)
	go func() {
		deadline := time.After(5 * time.Second)
		for {
			select {
			case ev := <-s.Events():
				s.ApplyEvent(ev)
				if ev.Type == jobs.EventCompleted && ev.JobID == job.ID {
					if ev.DoneFiles < 1 {
						errCh <- fmt.Sprintf("EventCompleted DoneFiles = %d, want at least 1", ev.DoneFiles)
						return
					}
					snap := s.Snapshot()
					if len(snap) != 1 {
						errCh <- fmt.Sprintf("snapshot len = %d, want 1", len(snap))
						return
					}
					if snap[0].DoneFiles < 1 {
						errCh <- fmt.Sprintf("job DoneFiles = %d, want at least 1 after dropped progress", snap[0].DoneFiles)
						return
					}
					errCh <- ""
					return
				}
			case <-deadline:
				errCh <- "timeout draining full event channel to EventCompleted"
				return
			}
		}
	}()

	for i := 0; i < 100; i++ {
		s.QueueTestEvent(jobs.Event{Type: jobs.EventProgress, JobID: job.ID, Status: jobs.StatusRunning, DoneFiles: 0, DoneBytes: 0})
	}

	s.StartWorker(stop)
	s.AddJob(job)
	if msg := <-errCh; msg != "" {
		t.Fatal(msg)
	}
}
