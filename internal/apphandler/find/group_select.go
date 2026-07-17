package find

import "github.com/paranoidi/paras-commander/internal/panel"

// GroupSelectMode is select vs unselect for the group-select dialog.
type GroupSelectMode string

const (
	GroupSelectModeSelect   GroupSelectMode = "select"
	GroupSelectModeUnselect GroupSelectMode = "unselect"
)

// GroupSelectRequest applies a pattern match to find dialog marks.
type GroupSelectRequest struct {
	Mode          GroupSelectMode
	Pattern       string
	FilesOnly     bool
	DirsOnly      bool
	CaseSensitive bool
	PatternMode   panel.GroupPatternMode
}
