package app

import (
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func TestCloseActiveJobThroughputColumn_onePerTick(t *testing.T) {
	t.Parallel()
	s := jobs.NewState()
	col := 100 * time.Millisecond
	s.SetThroughputChart(col, 10*time.Second, true)
	job := &jobs.Job{ID: "j1", Type: jobs.TypeCopy, Status: jobs.StatusQueued, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")}
	s.AddJob(job)
	s.ApplyEvent(jobs.Event{Type: jobs.EventStarted, JobID: job.ID})

	t0 := time.Unix(0, 0)
	if s.CloseActiveJobThroughputColumn(t0) {
		t.Fatal("first tick should only anchor")
	}
	active := s.ActiveJob()
	if active == nil {
		t.Fatal("expected active job")
	}
	if len(active.ThroughputStrip) != 0 {
		t.Fatalf("strip = %v want empty", active.ThroughputStrip)
	}

	t1 := t0.Add(col)
	s.ApplyEvent(jobs.Event{Type: jobs.EventProgress, JobID: job.ID, DoneBytes: 1000})
	if !s.CloseActiveJobThroughputColumn(t1) {
		t.Fatal("expected one column after interval")
	}
	active = s.ActiveJob()
	if len(active.ThroughputStrip) != 1 {
		t.Fatalf("len=%d want 1", len(active.ThroughputStrip))
	}

	t2 := t1.Add(col)
	s.ApplyEvent(jobs.Event{Type: jobs.EventProgress, JobID: job.ID, DoneBytes: 2000})
	if !s.CloseActiveJobThroughputColumn(t2) {
		t.Fatal("expected second column")
	}
	active = s.ActiveJob()
	if len(active.ThroughputStrip) != 2 {
		t.Fatalf("len=%d want 2", len(active.ThroughputStrip))
	}

	// Progress alone must not advance the strip.
	s.ApplyEvent(jobs.Event{Type: jobs.EventProgress, JobID: job.ID, DoneBytes: 5000})
	if s.CloseActiveJobThroughputColumn(t2) {
		t.Fatal("same instant should not close twice")
	}
	active = s.ActiveJob()
	if len(active.ThroughputStrip) != 2 {
		t.Fatalf("progress must not grow strip: len=%d", len(active.ThroughputStrip))
	}
}
