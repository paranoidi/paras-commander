package dialog

import (
	"context"

	"github.com/paranoidi/paras-commander/internal/ops"
)

// elevatedCtx returns a context whose Elevator asks the user (Deps.PromptElevate) whether a
// permission-denied local step may be retried as root. Without a prompt or with
// [operations] elevate off it is a plain background context.
func (h *Handler) elevatedCtx() context.Context {
	return ops.WithElevator(context.Background(), ops.NewElevator(h.host.Config().Operations.Elevate, h.promptElevate))
}

// canElevate reports whether a synchronous local file operation that failed with err should be
// rerun on a worker goroutine with an Elevator, where the retry prompt can be shown.
func (h *Handler) canElevate(err error) bool {
	return h.promptElevate != nil && h.host.Config().Operations.Elevate && ops.ElevationCandidate(err)
}
