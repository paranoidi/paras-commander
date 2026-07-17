package ops

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
)

// DuplicatePlan describes a validated same-directory directory copy.
type DuplicatePlan struct {
	SourcePath string
	NewName    string
	DestPath   string
}

// ValidateDuplicateSource requires exactly one entry (file or directory,
// selection or cursor).
func ValidateDuplicateSource(p *panel.State) (localfs.Entry, error) {
	source, err := ResolveSource(p)
	if err != nil {
		return localfs.Entry{}, err
	}
	if len(source.Entries) > 1 {
		return localfs.Entry{}, &Error{Op: "duplicate", Text: "select a single file or directory"}
	}
	return source.Entries[0], nil
}

// PlanDuplicate validates a same-directory copy of a directory under a new basename.
func PlanDuplicate(source localfs.Entry, newName string, panelPath string) (DuplicatePlan, error) {
	trimmed := strings.TrimSpace(newName)
	srcLoc, destLoc, err := planSiblingTarget(source.Path, panelPath, trimmed, siblingTargetRules{
		op:           "duplicate",
		sameNameText: "new name must differ from the original",
		rejectDotDot: true,
	})
	if err != nil {
		return DuplicatePlan{}, err
	}
	return DuplicatePlan{
		SourcePath: srcLoc.String(),
		NewName:    trimmed,
		DestPath:   destLoc.String(),
	}, nil
}
