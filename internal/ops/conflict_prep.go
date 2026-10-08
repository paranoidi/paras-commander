package ops

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// ConflictAction is the resolver's verdict for one existing destination.
type ConflictAction int

const (
	// ActionSkip leaves the destination untouched and the source in place.
	ActionSkip ConflictAction = iota
	// ActionOverwrite replaces the destination.
	ActionOverwrite
	// ActionIdentical means source and destination hold the same content: nothing is copied and
	// a move removes the source.
	ActionIdentical
	// ActionRename writes the source under the first free "name (N).ext" sibling.
	ActionRename
)

// ConflictResolution is a ConflictResolver's answer.
type ConflictResolution struct {
	Action ConflictAction
}

// uniqueSiblingName returns the first free "stem (N).ext" next to dst (N starting at 1).
func uniqueSiblingName(ctx context.Context, dst pathloc.Path) (pathloc.Path, error) {
	base := dst.Base()
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	parent := dst.Parent()
	for n := 1; ; n++ {
		if err := ctx.Err(); err != nil {
			return pathloc.Path{}, err
		}
		cand, err := parent.Join(stem + " (" + strconv.Itoa(n) + ")" + ext)
		if err != nil {
			return pathloc.Path{}, err
		}
		if _, err := statEntry(ctx, cand); isNotExist(err) {
			return cand, nil
		} else if err != nil {
			return pathloc.Path{}, err
		}
	}
}

// resolveConflict asks the conflict resolver what to do with an existing destination. For
// ActionRename it also returns the free sibling path to write to. It errors when canceling or
// when no resolver is set.
func resolveConflict(ctx context.Context, src, dst pathloc.Path, resolver ConflictResolver, facts FileConflictFacts) (ConflictAction, pathloc.Path, error) {
	if resolver == nil {
		return ActionSkip, dst, fmt.Errorf("destination %q already exists and no conflict resolver configured", dst)
	}
	res, err := resolver(ctx, src.String(), dst.String(), facts)
	if err != nil {
		return ActionSkip, dst, err
	}
	if res.Action == ActionRename {
		newDst, err := uniqueSiblingName(ctx, dst)
		return ActionRename, newDst, err
	}
	return res.Action, dst, nil
}
