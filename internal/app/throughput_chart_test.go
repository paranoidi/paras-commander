package app

import (
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func TestSampleActiveJobThroughput_onePerColumn(t *testing.T) {
	t.Parallel()
	s := jobs.NewState()
	col := 100 * time.Millisecond
	s.SetThroughputChart(col, 10*time.Second, true)
	job := &jobs.Job{ID: "j1", Type: jobs.TypeCopy, Status: jobs.StatusQueued, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")}
	s.AddJob(job)
	s.ApplyEvent(jobs.Event{Type: jobs.EventStarted, JobID: job.ID})

	t0 := time.Unix(0, 0)
	if s.SampleActiveJobThroughput(t0) {
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
	if !s.SampleActiveJobThroughput(t1) {
		t.Fatal("expected one column after interval")
	}
	active = s.ActiveJob()
	if len(active.ThroughputStrip) != 1 {
		t.Fatalf("len=%d want 1", len(active.ThroughputStrip))
	}

	t2 := t1.Add(col)
	s.ApplyEvent(jobs.Event{Type: jobs.EventProgress, JobID: job.ID, DoneBytes: 2000})
	if !s.SampleActiveJobThroughput(t2) {
		t.Fatal("expected second column")
	}
	active = s.ActiveJob()
	if len(active.ThroughputStrip) != 2 {
		t.Fatalf("len=%d want 2", len(active.ThroughputStrip))
	}

	// Oversampled ticks between column boundaries must be no-ops, however much progress arrived.
	s.ApplyEvent(jobs.Event{Type: jobs.EventProgress, JobID: job.ID, DoneBytes: 5000})
	if s.SampleActiveJobThroughput(t2.Add(col / 4)) {
		t.Fatal("sub-column tick should not close a column")
	}
	active = s.ActiveJob()
	if len(active.ThroughputStrip) != 2 {
		t.Fatalf("progress must not grow strip: len=%d", len(active.ThroughputStrip))
	}
}

func TestThroughputTickInterval(t *testing.T) {
	t.Parallel()
	if got := throughputTickInterval(400 * time.Millisecond); got != 100*time.Millisecond {
		t.Fatalf("400ms column -> %v want 100ms", got)
	}
	// Smallest configurable column (80ms) floors at the minimum tick rather than 20ms.
	if got := throughputTickInterval(80 * time.Millisecond); got != throughputTickMinInterval {
		t.Fatalf("80ms column -> %v want %v", got, throughputTickMinInterval)
	}
	// The floor must never overshoot the column itself.
	if got := throughputTickInterval(30 * time.Millisecond); got != 30*time.Millisecond {
		t.Fatalf("30ms column -> %v want 30ms", got)
	}
}
