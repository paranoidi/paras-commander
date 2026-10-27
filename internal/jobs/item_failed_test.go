package jobs

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func failedItemFor(src, dst string) FailedItem {
	return FailedItem{
		Item:  ops.PlanItem{Src: pathloc.PathsForTest(src)[0], Dst: pathloc.PathsForTest(dst)[0], FileSize: 7},
		Error: "permission denied",
	}
}

// TestApplyEventItemFailedAppends: EventItemFailed is recorded on the job by ApplyEvent.
func TestApplyEventItemFailedAppends(t *testing.T) {
	s := NewState()
	job := &Job{ID: "ember", Type: TypeDelete, Status: StatusQueued, Sources: pathloc.PathsForTest("/x")}
	s.AddJob(job)
	fi := failedItemFor("/harbor/lantern", "/meadow/lantern")
	s.ApplyEvent(Event{Type: EventItemFailed, JobID: "ember", FailedItem: &fi})
	got := s.AllJobs()
	if len(got) != 1 || len(got[0].FailedItems) != 1 || got[0].FailedItems[0] != fi {
		t.Fatalf("FailedItems = %+v, want [%+v]", got[0].FailedItems, fi)
	}
}

// TestRetryCloneWithFailedItemsSkipsPreScan: a copy retry re-runs only the failed items.
func TestRetryCloneWithFailedItemsSkipsPreScan(t *testing.T) {
	fi := failedItemFor("/harbor/willow", "/meadow/willow")
	j := &Job{ID: "birch", Type: TypeCopy, Status: StatusFailed, Sources: pathloc.PathsForTest("/harbor"),
		Destination: pathloc.PathsForTest("/meadow")[0], FailedItems: []FailedItem{fi}}
	clone := j.RetryClone()
	if len(clone.Plan) != 1 || clone.Plan[0] != fi.Item {
		t.Fatalf("Plan = %+v, want the failed item", clone.Plan)
	}
	if clone.NeedsPreScan() {
		t.Fatal("NeedsPreScan() = true, want false for a failed-items retry")
	}
	if clone.TotalFiles != 1 || clone.TotalBytes != 7 || !clone.TotalsComplete {
		t.Fatalf("totals = %d files %d bytes (complete %v), want final 1 and 7", clone.TotalFiles, clone.TotalBytes, clone.TotalsComplete)
	}
	if len(clone.FailedItems) != 0 {
		t.Fatalf("clone FailedItems = %+v, want none", clone.FailedItems)
	}
	plain := (&Job{ID: "birch", Type: TypeCopy, Sources: j.Sources}).RetryClone()
	if !plain.NeedsPreScan() || plain.TotalsComplete {
		t.Fatal("a copy retry without failed items must pre-scan for its totals")
	}
}

// TestRunJobFailsWhenItemsWereSkipped: a transfer that reports skipped items and returns nil
// ends the job failed with the item count.
func TestRunJobFailsWhenItemsWereSkipped(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	defer close(stop)
	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) BlockerAnswer) error {
		for _, n := range []string{"cedar", "maple"} {
			fi := failedItemFor("/harbor/"+n, "/meadow/"+n)
			emit(Event{Type: EventItemFailed, JobID: job.ID, FailedItem: &fi})
		}
		return nil
	})
	s.StartWorker(stop)
	s.AddJob(&Job{ID: "oak", Type: TypeDelete, Status: StatusQueued, Sources: pathloc.PathsForTest("/x"), TotalFiles: 1})

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for EventFailed")
		case ev := <-s.Events():
			if ev.Type == EventFailed && ev.JobID == "oak" {
				if ev.Error != "2 item(s) failed" {
					t.Fatalf("Error = %q, want %q", ev.Error, "2 item(s) failed")
				}
				return
			}
			if ev.Type == EventCompleted {
				t.Fatal("job completed despite skipped items")
			}
		}
	}
}

// TestRecordOperationKeepsItemSource: records take the source the executor tagged the item
// with, and a retry appends to the same per-source log.
func TestRecordOperationKeepsItemSource(t *testing.T) {
	s := NewState()
	job := &Job{ID: "quarry", Type: TypeCopy, Status: StatusQueued,
		Sources:     pathloc.PathsForTest("/harbor/lantern", "/meadow/copper"),
		Destination: pathloc.PathsForTest("/orchard")[0]}
	s.AddJob(job)
	item := func(src string, source int32) ops.PlanItem {
		return ops.PlanItem{Src: pathloc.PathsForTest(src)[0], Source: source}
	}
	job.RecordOperation(item("/harbor/lantern/acorn.bin", 0), ops.ItemDone, "")
	job.RecordOperation(item("/meadow/copper/thistle", 1), ops.ItemFailed, "permission denied")
	job.RetryClone().RecordOperation(item("/meadow/copper/clover", 1), ops.ItemSkipped, "exists")

	per := job.Ops.SourceStats()
	if len(per) != 2 || per[0].Done != 1 || per[1].Failed != 1 || per[1].Skipped != 1 {
		t.Fatalf("per-source stats = %+v", per)
	}
}

func TestStartSourceAdvancesPhases(t *testing.T) {
	s := NewState()
	job := &Job{ID: "lantern", Type: TypeUndo, Status: StatusQueued,
		Sources: pathloc.PathsForTest("/harbor/acorn", "/harbor/bramble", "/harbor/clover")}
	s.AddJob(job)
	job.StartSource(2)
	job.StartSource(2) // repeated start of the current source changes nothing
	job.StartSource(0)
	job.StartSource(7) // out of range is ignored
	if got := job.SourcePhases(); !slices.Equal(got, []SourcePhase{SourceStarted, SourceQueued, SourceFinished}) {
		t.Fatalf("phases = %v", got)
	}
	if got := job.RetryClone().SourcePhases(); len(got) != 3 || got[2] != SourceFinished {
		t.Fatalf("retry phases = %v, want the shared progress", got)
	}
	single := &Job{ID: "willow", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/harbor/acorn")}
	s.AddJob(single)
	single.StartSource(0)
	if single.SourcePhases() != nil {
		t.Fatal("a single-source job has no per-source phases")
	}
}
