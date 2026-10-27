package jobs

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/oplog"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TestRecordOperationTagsSource: each record names the source its item lies under, and the
// tag survives a retry (same log, same index).
func TestRecordOperationTagsSource(t *testing.T) {
	s := NewState()
	job := &Job{ID: "quarry", Type: TypeCopy, Status: StatusQueued,
		Sources:     pathloc.PathsForTest("/harbor/lantern", "/harbor/willow.txt", "/meadow/copper"),
		Destination: pathloc.PathsForTest("/orchard")[0]}
	s.AddJob(job)
	item := func(src string) ops.PlanItem {
		return ops.PlanItem{Src: pathloc.PathsForTest(src)[0], Dst: pathloc.PathsForTest("/orchard" + src)[0]}
	}
	job.RecordOperation(item("/harbor/lantern/deep/acorn.bin"), ops.ItemDone, "")
	job.RecordOperation(item("/harbor/willow.txt"), ops.ItemDone, "")
	job.RecordOperation(item("/meadow/copper/thistle"), ops.ItemFailed, "permission denied")
	job.RecordOperation(item("/harbor/lanterns/other"), ops.ItemDone, "") // a sibling name, not under a source
	job.RetryClone().RecordOperation(item("/meadow/copper/clover"), ops.ItemSkipped, "exists")

	per, last := job.Ops.SourceStats()
	if len(per) != 3 || last != 2 {
		t.Fatalf("SourceStats = %+v, last %d", per, last)
	}
	if per[0].Done != 1 || per[1].Done != 1 || per[2].Failed != 1 || per[2].Skipped != 1 {
		t.Fatalf("per-source stats = %+v", per)
	}
	if n := job.Ops.ViewLen(oplog.MaskAll, oplog.NoSource); n != 5 {
		t.Fatalf("all rows = %d, want 5 (the unattributed row included)", n)
	}
}

func TestSourceIndexSingleSourceIsNil(t *testing.T) {
	if x := newSourceIndex(pathloc.PathsForTest("/harbor")); x != nil {
		t.Fatal("a single-source job needs no index")
	}
	var x *sourceIndex
	if got := x.indexOf(pathloc.PathsForTest("/harbor/acorn")[0]); got != oplog.NoSource {
		t.Fatalf("nil index = %d", got)
	}
}
