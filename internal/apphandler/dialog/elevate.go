package dialog

import (
	"context"

	"github.com/paranoidi/paras-commander/internal/fschange"
	"github.com/paranoidi/paras-commander/internal/ops"
)

// fsCtx returns a background context carrying the fschange sink of a dialog operation (ADR 0023):
// the shared sink and the fileindex (ADR 0028).
func (h *Handler) fsCtx() context.Context {
	return fschange.WithSink(context.Background(), fschange.Multi(h.fsSink, h.indexSink))
}

// elevatedCtx is fsCtx with an Elevator that asks the user (Deps.PromptElevate) whether a
// permission-denied local step may be retried as root. Without a prompt or with [operations]
// elevate off the Elevator never prompts.
func (h *Handler) elevatedCtx() context.Context {
	return ops.WithElevator(h.fsCtx(), ops.NewElevator(h.host.Config().Operations.Elevate, h.promptElevate))
}

// canElevate reports whether a synchronous local file operation that failed with err should be
// rerun on a worker goroutine with an Elevator, where the retry prompt can be shown.
func (h *Handler) canElevate(err error) bool {
	return h.promptElevate != nil && h.host.Config().Operations.Elevate && ops.ElevationCandidate(err)
}
