package dialog

import (
	"fmt"
	"strings"

	"github.com/paranoidi/paras-commander/internal/findcrit"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
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

// IndexFindScope is where Find in index searches.
type IndexFindScope int

const (
	// IndexScopeWhole is the whole index, whatever directory the panel shows.
	IndexScopeWhole IndexFindScope = iota
	// IndexScopeCurrent is the panel directory and below.
	IndexScopeCurrent
	// IndexScopeSelected is the panel's selected directories and below.
	IndexScopeSelected
	indexScopeCount
)

// IndexScopeLabels are the Search in radio labels, indexed by IndexFindScope.
var IndexScopeLabels = [indexScopeCount]string{"Whole index", "Current directory", "Selected directories"}

// IndexFindAdvanced is the advanced section of Find in index: the query match mode and the scope.
// It outlives the dialog for the session (the handler keeps it between opens).
type IndexFindAdvanced struct {
	// Expanded shows the section; collapsed, the settings that differ from the default are
	// summarized on the separator above the results.
	Expanded bool
	Mode     findcrit.IndexMode
	Scope    IndexFindScope
}

// Clear resets the mode and the scope, keeping Expanded.
func (a *IndexFindAdvanced) Clear() { *a = IndexFindAdvanced{Expanded: a.Expanded} }

// Summary is the settings summary for the separator above the results, fitted to width cells
// ("" when everything is the default).
func (a IndexFindAdvanced) Summary(styles theme.Theme, width int) string {
	var terms []string
	if tag := a.Mode.Tag(); tag != "" {
		terms = append(terms, tag)
	}
	switch a.Scope {
	case IndexScopeCurrent:
		terms = append(terms, "current directory")
	case IndexScopeSelected:
		terms = append(terms, "selected directories")
	}
	if len(terms) == 0 || width <= 0 {
		return ""
	}
	return primitive.TruncateRight(styles.IconDialogCriteria()+" "+strings.Join(terms, " · "), width)
}

// IndexFindDialogState is the "Find in index" dialog on a
// fileindex:// panel. Its widgets are described by IndexFindDialogView; Focus names the focused
// one (the IndexFind* IDs, OK or Cancel; empty = the query, which also drives the result list).
type IndexFindDialogState struct {
	Open bool
	// Parked: hidden with its state kept, like FindDialogState.Parked.
	Parked  bool
	PanelID int
	// FromIndexPanel is true when the dialog opened from a fileindex:// panel: results then
	// navigate and select as fileindex:// paths, else as plain host paths.
	FromIndexPanel bool
	// Root is the host path of the panel directory the search runs under.
	Root        string
	Query       string
	QueryCursor int // rune offset of caret within Query
	QueryScroll int // first visible rune offset for horizontal scrolling
	OnlyDirs    bool
	OnlyFiles   bool
	// IncludeHidden keeps results under dot-prefixed path components.
	IncludeHidden bool
	// MaxDepth is the committed depth limit (path segments below Root; 0 = unlimited) and
	// DepthInput the depth field's edit buffer: "" or one digit 1-9.
	MaxDepth   int
	DepthInput string
	// Advanced is the match mode and scope; AdvancedMnemonic is the letter of the Advanced
	// checkbox label highlighted as its key (0 = none).
	Advanced         IndexFindAdvanced
	AdvancedMnemonic rune
	// ShowSelectedScope is true when the panel had selected directories at open: the scope
	// Selected directories (limiting the search to SelectionDirRoots, host paths) is offered.
	ShowSelectedScope bool
	SelectionDirRoots []string
	Results           []IndexFindResult
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

// AdvancedFocused reports whether focus is on a widget of the advanced section (the result list's
// keys do not reach it then, so the list is drawn inert). The Advanced checkbox on the query row
// is not in the section.
func (s *IndexFindDialogState) AdvancedFocused() bool {
	return strings.HasPrefix(string(s.Focus), indexFindAdvPrefix)
}

// Apply acts on a radio or checkbox event: the match mode, the scope and the result filters.
// The depth only means something below a directory, so choosing Whole index clears it.
func (s *IndexFindDialogState) Apply(id dlg.ID) {
	if m, ok := IndexFindAdvModeOf(id); ok {
		s.Advanced.Mode = m
		if m == findcrit.IndexBoolean {
			s.OnlyDirs = false // FTS5 matches files only
		}
		return
	}
	if sc, ok := IndexFindAdvScopeOf(id); ok {
		s.Advanced.Scope = sc
		if sc == IndexScopeWhole {
			s.MaxDepth, s.DepthInput = 0, ""
		}
		return
	}
	switch id {
	case IndexFindHidden:
		s.IncludeHidden = !s.IncludeHidden
	case IndexFindOnlyDirs:
		s.OnlyDirs = !s.OnlyDirs
		s.OnlyFiles = s.OnlyFiles && !s.OnlyDirs
	case IndexFindOnlyFiles:
		s.OnlyFiles = !s.OnlyFiles
		s.OnlyDirs = s.OnlyDirs && !s.OnlyFiles
	}
}

// ResetAdvanced returns the match mode and the scope to their defaults.
func (s *IndexFindDialogState) ResetAdvanced() {
	s.Advanced.Clear()
	s.Apply(IndexFindAdvScopeID(IndexScopeWhole))
}

// NoMatches reports whether a typed query finished with no results (the query input is then
// drawn in error style, as Find's).
func (s *IndexFindDialogState) NoMatches() bool {
	return strings.TrimSpace(s.Query) != "" && !s.Searching && s.Err == "" && len(s.Results) == 0
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
