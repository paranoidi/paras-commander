package ops

import (
	"context"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// PlanSource names where a transfer's plan comes from, so ExecuteCopyFrom/ExecuteMoveFrom can
// pick the matching Execute* variant in one place instead of that choice being re-derived at
// every call site. The zero value means "build the plan as part of this call" (ExecuteCopy /
// ExecuteMove); Chan set (paired with ChanErr) means a background producer is streaming the plan
// (ExecuteCopyUsingPlanChan / ExecuteMoveWithPlanChan); Slice set means the plan was already
// built synchronously (ExecuteCopyUsingPlan / ExecuteMoveWithPlan). At most one of Chan/Slice
// should be set; Chan takes priority if both are.
type PlanSource struct {
	Chan    <-chan PlanItem
	ChanErr func() error
	Slice   []PlanItem
}

// ExecuteCopyFrom runs a copy using whichever plan source src names.
func ExecuteCopyFrom(ctx context.Context, src PlanSource, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	switch {
	case src.Chan != nil:
		return ExecuteCopyUsingPlanChan(ctx, src.Chan, src.ChanErr, sources, destination, opts, throttle, progress, resolver, diskWait)
	case src.Slice != nil:
		return ExecuteCopyUsingPlan(ctx, src.Slice, sources, destination, opts, throttle, progress, resolver, diskWait)
	default:
		return ExecuteCopy(ctx, sources, destination, opts, throttle, progress, resolver, diskWait)
	}
}

// ExecuteMoveFrom runs a move using whichever plan source src names.
func ExecuteMoveFrom(ctx context.Context, src PlanSource, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	switch {
	case src.Chan != nil:
		return ExecuteMoveWithPlanChan(ctx, src.Chan, src.ChanErr, sources, destination, opts, throttle, progress, resolver, diskWait)
	case src.Slice != nil:
		return ExecuteMoveWithPlan(ctx, src.Slice, sources, destination, opts, throttle, progress, resolver, diskWait)
	default:
		return ExecuteMove(ctx, sources, destination, opts, throttle, progress, resolver, diskWait)
	}
}
