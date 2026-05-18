package jobs

import (
	"testing"
	"time"
)

func TestComputeQueueETAsCumulative(t *testing.T) {
	now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	started := now.Add(-30 * time.Second)

	running := &Job{
		ID:             "run",
		Status:         StatusRunning,
		StartedAt:      started,
		TotalBytes:     1_000_000,
		DoneBytes:      500_000,
		TotalFiles:     100,
		DoneFiles:      50,
		ETABytesPerSec: 30_000,
		ETAFilesPerSec: 5,
	}
	queued := &Job{
		ID:         "q1",
		Status:     StatusQueued,
		TotalBytes: 600_000,
		TotalFiles: 60,
	}
	queued2 := &Job{
		ID:         "q2",
		Status:     StatusQueued,
		TotalBytes: 300_000,
		TotalFiles: 30,
	}

	etas := ComputeQueueETAs([]*Job{running, queued, queued2}, now)
	if etas["run"] == "" || etas["run"] == "—" {
		t.Fatalf("running ETA = %q, want positive duration", etas["run"])
	}
	if etas["q1"] == "" || etas["q1"] == "—" {
		t.Fatalf("queued ETA = %q, want cumulative estimate", etas["q1"])
	}
	dRun, err := time.ParseDuration(etas["run"])
	if err != nil {
		t.Fatalf("parse running: %v", err)
	}
	dQ1, err := time.ParseDuration(etas["q1"])
	if err != nil {
		t.Fatalf("parse q1: %v", err)
	}
	dQ2, err := time.ParseDuration(etas["q2"])
	if err != nil {
		t.Fatalf("parse q2: %v", err)
	}
	if dQ1 <= dRun {
		t.Fatalf("q1 ETA %v should exceed running remain %v", dQ1, dRun)
	}
	if dQ2 <= dQ1 {
		t.Fatalf("q2 ETA %v should exceed q1 ETA %v", dQ2, dQ1)
	}
}

func TestComputeQueueETAsScanningShowsDash(t *testing.T) {
	now := time.Now()
	etas := ComputeQueueETAs([]*Job{{ID: "s", Status: StatusScanning}}, now)
	if etas["s"] != "—" {
		t.Fatalf("scanning ETA = %q, want —", etas["s"])
	}
}
