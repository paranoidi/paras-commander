package dialog

import (
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/search"
)

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
	// QueryCompletionSuffix is ghost text after the caret (Tab accepts into Query).
	QueryCompletionSuffix string
	// QueryCompletionIsDir is true when accepting should append a trailing slash.
	QueryCompletionIsDir bool
}

// FindEntry is one indexed path in the recursive find dialog.
type FindEntry struct {
	RelLine string
	IsDir   bool
	Type    localfs.EntryType
}

// AbsPath reconstructs the absolute path of the entry from the dialog's root path.
func (e FindEntry) AbsPath(rootPath string) string {
	return filepath.Join(rootPath, filepath.FromSlash(e.RelLine))
}

// FindDialogState is a fuzzy picker over recursively indexed paths under a panel root.
// Focus without selections checkbox: 0=list+filter, 1=stay-on-volume, 2=only-directories, 3=OK, 4=Cancel.
// With selections checkbox: 0=list+filter, 1=stay-on-volume, 2=only-directories, 3=search-selections, 4=OK, 5=Cancel.
type FindDialogState struct {
	Open                bool
	PanelID             int
	RootPath            string
	ShowHidden          bool
	StayOnCurrentVolume bool
	// OnlyDirectories hides non-directory entries from the ranked result list (instant filter).
	OnlyDirectories    bool
	ListingDevice      uint64
	ListingDeviceValid bool
	// ShowSearchSelectionsOption is true when the panel had selected directories at open.
	ShowSearchSelectionsOption bool
	// SearchOnlySelections limits indexing to SelectionDirRoots when true.
	SearchOnlySelections bool
	// SelectionDirRoots is a pruned snapshot of selected directory paths at dialog open.
	SelectionDirRoots []string
	Entries           []FindEntry
	Query             string
	QueryCursor       int // rune offset of caret within Query (0..len(runes))
	QueryScroll       int // first visible rune offset for horizontal scrolling
	Ranked            []int
	MatchRanges       map[int][]search.Range // sparse: only entries with actual match ranges
	Selected          int
	ListScroll        int
	Focus             int
	Indexing          bool
	IndexedCount      int
	IndexDone         bool
	IndexErr          string
	// RankPending is true while an async background rank is in progress or scheduled.
	RankPending bool
	// MarkedPaths holds paths toggled selected in the dialog (applied to the panel on OK).
	MarkedPaths map[string]bool
}

// FindDialogHasSelectionsCheckbox reports whether the search-selections row is shown.
func (s FindDialogState) FindDialogHasSelectionsCheckbox() bool {
	return s.ShowSearchSelectionsOption
}

// FindDialogOnlyDirsFocus returns the focus index for the only-directories checkbox.
func (s FindDialogState) FindDialogOnlyDirsFocus() int {
	return 2
}

// FindDialogSelectionsFocus returns the focus index for search-only-selections, or -1 when hidden.
func (s FindDialogState) FindDialogSelectionsFocus() int {
	if s.FindDialogHasSelectionsCheckbox() {
		return 3
	}
	return -1
}

// FindDialogOKFocus returns the focus index for the OK button.
func (s FindDialogState) FindDialogOKFocus() int {
	if s.FindDialogHasSelectionsCheckbox() {
		return 4
	}
	return 3
}

// FindDialogCancelFocus returns the focus index for the Cancel button.
func (s FindDialogState) FindDialogCancelFocus() int {
	if s.FindDialogHasSelectionsCheckbox() {
		return 5
	}
	return 4
}

// FindDialogMaxFocus returns the highest focus index (Cancel).
func (s FindDialogState) FindDialogMaxFocus() int {
	return s.FindDialogCancelFocus()
}

// HistoryDialogState is a fuzzy picker over one panel’s navigation history paths.
type HistoryDialogState struct {
	Open         bool
	PanelID      int      // LeftPanel or RightPanel
	Paths        []string // snapshot when dialog opened
	CurrentIndex int      // snapshot HistoryIndex when dialog opened
	DisplayLines []string // per-row UI text ("* path" / "  path"); len == len(Paths)
	Query        string
	QueryCursor  int              // rune offset of caret within Query (0..len(runes))
	QueryScroll  int              // first visible rune offset for horizontal scrolling
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
	// FuzzyExtra is action id and keywords for the rank-only corpus (after Title), space-separated.
	FuzzyExtra string
}

// HelpViewState holds state for the centered help dialog with fuzzy search.
type HelpViewState struct {
	Open        bool
	Query       string
	QueryCursor int // rune offset of caret within Query (0..len(runes))
	QueryScroll int // first visible rune offset for horizontal scrolling
	Entries     []HelpEntry
	Ranked      []int            // indices into Entries (rank order)
	MatchRanges [][]search.Range // len == len(Entries); rune ranges on the painted padded row
	Selected    int              // index into Ranked
	ListScroll  int              // first visible row index into Ranked
	Focus       int              // 0=list+fiter, 1=Close button
}
