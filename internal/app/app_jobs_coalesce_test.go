package app

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func TestCoalesceJobEventBatchKeepsLastProgressPerJob(t *testing.T) {
	t.Parallel()
	batch := []jobs.Event{
		{Type: jobs.EventProgress, JobID: "a", DoneBytes: 1},
		{Type: jobs.EventStarted, JobID: "a"},
		{Type: jobs.EventProgress, JobID: "a", DoneBytes: 99},
		{Type: jobs.EventProgress, JobID: "b", DoneBytes: 5},
		{Type: jobs.EventProgress, JobID: "b", DoneBytes: 50},
	}
	got := coalesceJobEventBatch(batch)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (latest progress per job + started)", len(got))
	}
	var aBytes, bBytes int64
	var sawStarted bool
	for _, ev := range got {
		switch {
		case ev.Type == jobs.EventStarted && ev.JobID == "a":
			sawStarted = true
		case ev.Type == jobs.EventProgress && ev.JobID == "a":
			aBytes = ev.DoneBytes
		case ev.Type == jobs.EventProgress && ev.JobID == "b":
			bBytes = ev.DoneBytes
		}
	}
	if !sawStarted {
		t.Fatal("missing started event")
	}
	if aBytes != 99 {
		t.Fatalf("job a DoneBytes = %d, want 99", aBytes)
	}
	if bBytes != 50 {
		t.Fatalf("job b DoneBytes = %d, want 50", bBytes)
	}
}
