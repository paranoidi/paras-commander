package ops

import (
	"context"
	"fmt"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

type (
	softStopKey   struct{}
	renameNoteKey struct{}
)

// errSoftStopped ends an operation at an item boundary after a soft stop. It wraps
// context.Canceled so every cancel path (job status, wrapping, mapOpsCanceled) treats it as one.
var errSoftStopped = fmt.Errorf("soft stop: %w", context.Canceled)

// WithSoftStop returns a context whose item-boundary checks (itemBoundaryErr) stop the operation
// once requested reports true. The file being worked on finishes first. requested must not block
// on locks the operation's caller holds.
func WithSoftStop(ctx context.Context, requested func() bool) context.Context {
	return context.WithValue(ctx, softStopKey{}, requested)
}

// withoutSoftStop masks a soft stop for work that must run to completion, such as removing
// sources that were already copied.
func withoutSoftStop(ctx context.Context) context.Context {
	return context.WithValue(ctx, softStopKey{}, (func() bool)(nil))
}

// itemBoundaryErr is the check between items: ctx.Err() when canceled, errSoftStopped when a soft
// stop is requested, else nil. Never call it inside a single file's work.
func itemBoundaryErr(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if requested, _ := ctx.Value(softStopKey{}).(func() bool); requested != nil && requested() {
		return errSoftStopped
	}
	return nil
}

// withRenameNote makes resolveConflict report each keep-both rename (from, to) to note, so a
// caller can learn the final destination of a transferred file.
func withRenameNote(ctx context.Context, note func(from, to pathloc.Path)) context.Context {
	return context.WithValue(ctx, renameNoteKey{}, note)
}

func noteRename(ctx context.Context, from, to pathloc.Path) {
	if note, _ := ctx.Value(renameNoteKey{}).(func(from, to pathloc.Path)); note != nil {
		note(from, to)
	}
}
