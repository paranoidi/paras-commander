package jobbridge

import (
	"unsafe"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
)

func init() {
	if unsafe.Sizeof(jobs.PlanItem{}) != unsafe.Sizeof(ops.PlanItem{}) {
		panic("jobs.PlanItem and ops.PlanItem must stay the same size")
	}
}
