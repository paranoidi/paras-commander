package find

import (
	"fmt"
	"regexp"

	"github.com/paranoidi/paras-commander/internal/panel"
)

// GroupSelectMode is select vs unselect for the group-select dialog.
type GroupSelectMode string

const (
	GroupSelectModeSelect   GroupSelectMode = "select"
	GroupSelectModeUnselect GroupSelectMode = "unselect"
)

// GroupSelectRequest applies a pattern and/or file-type match to find dialog marks. Types (the
// OR of the checked presets, nil = none) always matches the result's basename; a request with
// neither Pattern nor Types matches nothing.
type GroupSelectRequest struct {
	Mode          GroupSelectMode
	Pattern       string
	FilesOnly     bool
	DirsOnly      bool
	CaseSensitive bool
	FullPath      bool
	PatternMode   panel.GroupPatternMode
	Types         *regexp.Regexp
	TypeLabel     string // checked type names, for messages
}

// Empty reports whether the request restricts nothing (no pattern and no types).
func (r GroupSelectRequest) Empty() bool { return r.Pattern == "" && r.Types == nil }

// Describe names what the request matches for transient messages: the quoted pattern, the type
// label, or both.
func (r GroupSelectRequest) Describe() string {
	switch {
	case r.Pattern != "" && r.TypeLabel != "":
		return fmt.Sprintf("%q (%s)", r.Pattern, r.TypeLabel)
	case r.Pattern != "":
		return fmt.Sprintf("%q", r.Pattern)
	}
	return r.TypeLabel
}
