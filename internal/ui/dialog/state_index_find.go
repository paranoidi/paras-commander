package dialog

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// IndexFindResult is one row of the find-in-index dialog: the real host path the index records
// and the line shown for it (relative to the dialog's root).
type IndexFindResult struct {
	Path  string
	Rel   string
	IsDir bool
	Size  int64
}

// IndexFindDialogState is the "Find in index" dialog (and its jump-to-directory form) on a
// fileindex:// panel. Its widgets are described by IndexFindDialogView; Focus names the focused
// one (the IndexFind* IDs, OK or Cancel; empty = the query, which also drives the result list).
type IndexFindDialogState struct {
	Open    bool
	PanelID int
	// Root is the host path of the panel directory the search runs under.
	Root        string
	Query       string
	QueryCursor int // rune offset of caret within Query
	QueryScroll int // first visible rune offset for horizontal scrolling
	// Words picks the FTS5 filename search instead of the name glob.
	Words     bool
	OnlyDirs  bool
	OnlyFiles bool
	// IncludeHidden keeps results under dot-prefixed path components.
	IncludeHidden bool
	// MaxDepth is the committed depth limit (path segments below Root; 0 = unlimited) and
	// DepthInput the depth field's edit buffer: "" or one digit 1-9.
	MaxDepth   int
	DepthInput string
	// ShowSearchSelectionsOption is true when the panel had selected directories at open;
	// SearchOnlySelections then limits the search to SelectionDirRoots (host paths).
	ShowSearchSelectionsOption bool
	SearchOnlySelections       bool
	SelectionDirRoots          []string
	// JumpMode is "Jump to directory": name search over directories only, no marking.
	JumpMode bool
	Results  []IndexFindResult
	// Truncated is true when more results matched than Results holds.
	Truncated bool
	Searching bool
	// Err is the last query error (an FTS5 syntax error), shown instead of results.
	Err string
	// Gen identifies the latest query; a result carrying another Gen is stale.
	Gen        int
	Selected   int
	ListScroll int
	// Marked holds the host paths marked for the panel selection; MarkedDirs says which of them
	// are directories (marks outlive the result list they were made in).
	Marked      map[string]bool
	MarkedDirs  map[string]bool
	MarkedSizes map[string]int64
	Focus       dlg.ID
}

// MarkedCount is the number of marked results.
func (s *IndexFindDialogState) MarkedCount() int {
	n := 0
	for _, on := range s.Marked {
		if on {
			n++
		}
	}
	return n
}

// Apply acts on a radio event: the mode and the result filters.
func (s *IndexFindDialogState) Apply(id dlg.ID) {
	switch id {
	case IndexFindName:
		s.Words = false
	case IndexFindWords:
		s.Words, s.OnlyDirs = true, false // FTS5 matches files only
	case IndexFindHidden:
		s.IncludeHidden = !s.IncludeHidden
	case IndexFindSelections:
		s.SearchOnlySelections = !s.SearchOnlySelections
	case IndexFindOnlyDirs:
		s.OnlyDirs = !s.OnlyDirs
		s.OnlyFiles = s.OnlyFiles && !s.OnlyDirs
	case IndexFindOnlyFiles:
		s.OnlyFiles = !s.OnlyFiles
		s.OnlyDirs = s.OnlyDirs && !s.OnlyFiles
	}
}

// MarkedLabel is the marked-selection summary for the button separator ("" = nothing marked).
// ponytail: only files carry an indexed size, so a marked directory counts as one item and adds no
// bytes; sum the files under it from the index if that undercount matters.
func (s *IndexFindDialogState) MarkedLabel() string {
	n := s.MarkedCount()
	if n == 0 {
		return ""
	}
	var total int64
	for p, on := range s.Marked {
		if on {
			total += s.MarkedSizes[p]
		}
	}
	word := "items"
	if n == 1 {
		word = "item"
	}
	return fmt.Sprintf("%d %s (%s)", n, word, findFormatSelectionByteSize(total))
}
