package jobbridge

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestPlanItemMatchesOps(t *testing.T) {
	src := pathloc.MustParse("/src")
	dst := pathloc.MustParse("/dst")
	o := ops.PlanItem{Src: src, Dst: dst, IsDir: true, IsSymlink: false, FileSize: 42}
	j := PlanItemsFromOps([]ops.PlanItem{o})[0]
	if j != jobs.PlanItem(o) {
		t.Fatalf("PlanItemsFromOps = %+v, want %+v", j, jobs.PlanItem(o))
	}
	back := PlanItemsToOps([]jobs.PlanItem{j})[0]
	if back != o {
		t.Fatalf("PlanItemsToOps = %+v, want %+v", back, o)
	}
}
