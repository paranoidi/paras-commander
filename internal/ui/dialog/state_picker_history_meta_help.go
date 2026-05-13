package dialog

import "github.com/paranoidi/paras-commander/internal/search"

// PathPickerPurpose selects what happens when the user confirms a path in the picker.
type PathPickerPurpose int

const (
	// PathPickerPurposeNavigate jumps the active panel to the selected directory (bookmarks menu).
	PathPickerPurposeNavigate PathPickerPurpose = iota
	// PathPickerPurposeApplyTransferDestination writes the path into the copy/move destination field.
	PathPickerPurposeApplyTransferDestination
	// PathPickerPurposeApplyFileDialogField writes the path into FileDialog.Fields[FileFieldIndex].Value.
	PathPickerPurposeApplyFileDialogField
)

// PathPickerItem is one fuzzy-listed row (display Line + filesystem Path).
type PathPickerItem struct {
	Line string
	Path string
}

// PathPickerState is a fuzzy-filtered list dialog (bookmarks, quick path, etc.).
type PathPickerState struct {
	Open           bool
	Title          string
	Purpose        PathPickerPurpose
	FileFieldIndex int // when Purpose == PathPickerPurposeApplyFileDialogField
	Query          string
	QueryCursor    int // rune offset of caret within Query (0..len(runes))
	QueryScroll    int // first visible rune offset for horizontal scrolling
	Items          []PathPickerItem
	Ranked         []int // indices into Items (rank order)
	MatchRanges    [][]search.Range
	Selected       int // index into Ranked
	ListScroll     int // first visible row index into Ranked
	Focus          int // 0=list+query, 1=OK, 2=Cancel
	// QueryPathInvalid is true after a debounced check when the filter looks like a path and os.Lstat fails.
	QueryPathInvalid bool
	// QueryPathCheckPending is true until debounced validation runs after Query changed.
	QueryPathCheckPending bool
}

// HistoryDialogState is a fuzzy picker over one panel’s navigation history paths.
type HistoryDialogState struct {
	Open         bool
	PanelID      int      // LeftPanel or RightPanel
	Paths        []string // snapshot when dialog opened
	CurrentIndex int      // snapshot HistoryIndex when dialog opened
	DisplayLines []string // per-row UI text ("* path" / "  path"); len == len(Paths)
	Query        string
	Ranked       []int            // indices into Paths / DisplayLines
	MatchRanges  [][]search.Range // len == len(Paths); highlights on DisplayLines
	Selected     int              // index into Ranked
	ListScroll   int
	Focus        int // 0=list+query, 1=OK, 2=Cancel
}

// MetaEntry is one selectable command in the meta picker dialog.
type MetaEntry struct {
	Name        string
	Description string
}

// MetaDialogState is the radio-button picker for selecting a meta command to run on panel entries.
// Entries always has "None" as first item (index 0). Focus 0..len(Entries)-1 are radio rows;
// len(Entries) is OK; len(Entries)+1 is Cancel.
type MetaDialogState struct {
	Open     bool
	PanelID  int
	Entries  []MetaEntry // first entry is always {Name:"none", Description:"None (clear)"}
	Selected int         // index into Entries (0 = None)
	Focus    int         // 0..len(Entries)-1 radio items, len = OK, len+1 = Cancel
}

// HelpEntry is one row in the full-screen help view.
type HelpEntry struct {
	ActionID string // keymap action id (e.g. file.copy)
	Title    string // "Copy"
	Keys     string // "F5"
	Section  string // "File operations"
	Context  string // optional context, e.g. "Browser"
	Search   string // concatenated text for fuzzy matching
}

// HelpViewState holds state for the centered help dialog with fuzzy search.
type HelpViewState struct {
	Open        bool
	Query       string
	Entries     []HelpEntry
	Ranked      []int            // indices into Entries (rank order)
	MatchRanges [][]search.Range // len == len(Entries); highlight ranges on Search
	Selected    int              // index into Ranked
	ListScroll  int              // first visible row index into Ranked
	Focus       int              // 0=list+fiter, 1=Close button
}
