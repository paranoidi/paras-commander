package dialog

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// JumpSource is where Jump to directory takes its directories from.
type JumpSource int

const (
	// JumpDisk walks the disk under the panel directory (local panels).
	JumpDisk JumpSource = iota
	// JumpIndex queries the file index (fileindex:// panels).
	JumpIndex
)

// JumpResult is one directory row: its host path, the line shown for it (relative to the
// dialog's root) and the query's match ranges in that line.
type JumpResult struct {
	Path  string
	Rel   string
	Match []search.Range
}

// JumpDialogState is the "Jump to directory" dialog: a fuzzy name search over the directories
// below a panel directory whose Enter changes the panel to the highlighted one. Its widgets are
// described by JumpDialogView; Focus names the focused one (the Jump* IDs, OK or Cancel; empty =
// the query, which also drives the result list).
type JumpDialogState struct {
	Open    bool
	PanelID int
	Source  JumpSource
	// Root is the host path of the panel directory the search runs under.
	Root        string
	Query       string
	QueryCursor int // rune offset of caret within Query
	QueryScroll int // first visible rune offset for horizontal scrolling
	// MaxDepth is the committed depth limit (path segments below Root; 0 = unlimited) and
	// DepthInput the depth field's edit buffer: "" or one digit 1-9.
	MaxDepth   int
	DepthInput string
	// IncludeHidden adds dot-directories and git-ignored directories.
	IncludeHidden bool
	// StayOnVolume keeps the disk walk on the panel directory's file system (JumpDisk only).
	StayOnVolume bool

	Results []JumpResult
	// Truncated is set when more directories matched than the list holds ([ui.find].max_results).
	Truncated bool
	// Searching is set while a query (index) or a match pass (disk) for the current query runs.
	Searching bool
	// Scanning, Scanned, Depth, LevelDone and LevelTotal are the disk walk's progress: the
	// directories found so far and the breadth-first level being read.
	Scanning                     bool
	Scanned                      int
	Depth, LevelDone, LevelTotal int
	// Err is the last walk or query error.
	Err string

	Selected   int
	ListScroll int
	Focus      dlg.ID
}

// NoMatches reports whether a typed query finished with no results (the query input is then
// drawn in error style).
func (s *JumpDialogState) NoMatches() bool {
	return strings.TrimSpace(s.Query) != "" && !s.Searching && !s.Scanning && s.Err == "" && len(s.Results) == 0
}

// Cursor is the result under the list cursor.
func (s *JumpDialogState) Cursor() (JumpResult, bool) {
	if s.Selected < 0 || s.Selected >= len(s.Results) {
		return JumpResult{}, false
	}
	return s.Results[s.Selected], true
}

// PanelPath is the path r has in a panel: fileindex:// when the dialog searches the index, else
// the host path (pins and row marks use it).
func (s *JumpDialogState) PanelPath(r JumpResult) string {
	loc, err := pathloc.ForPanel(r.Path, s.Source == JumpIndex)
	if err != nil {
		return r.Path
	}
	return loc.String()
}
