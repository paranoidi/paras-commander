package ops

import (
	"context"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// transferRun holds immutable inputs for copy/move execution paths.
type transferRun struct {
	ctx         context.Context
	sources     []pathloc.Path
	destination pathloc.Path
	opts        Options
	throttle    ProgressEmitThrottle
	progress    ProgressCallback
	resolver    ConflictResolver
	diskWait    DiskWaitFunc
	// planOptional is the pre-built plan when non-nil; nil means build during execution.
	planOptional []PlanItem
}

func (r transferRun) executeCopy() (doneFiles int, doneBytes int64, transferred []pathloc.Path, err error) {
	return executeCopyWithPlan(r.ctx, r.planOptional, r.sources, r.destination, r.opts, r.throttle, r.progress, r.resolver, r.diskWait)
}

func (r transferRun) executeMoveCopyPhase() (int, int64, error) {
	return executeMoveCopyPhase(r.ctx, r.planOptional, r.sources, r.destination, r.opts, r.throttle, r.progress, r.resolver, r.diskWait)
}
