package dialog

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// PathPickerPurpose selects what happens when the user confirms a path in the picker.
type PathPickerPurpose int

const (
	// PathPickerPurposeNavigate jumps the active panel to the selected directory (bookmarks menu).
	PathPickerPurposeNavigate PathPickerPurpose = iota
	// PathPickerPurposeApplyTransferDestination writes the path into the copy/move destination field.
	PathPickerPurposeApplyTransferDestination
	// PathPickerPurposeApplyFlattenDestination writes the path into the flatten destination field.
	PathPickerPurposeApplyFlattenDestination
	// PathPickerPurposeApplyFileDialogField writes the path into FileDialog.Fields[FileFieldIndex].Value.
	PathPickerPurposeApplyFileDialogField
)

// PathPickerItem is one fuzzy-listed row with source, optional name, and filesystem path.
type PathPickerItem struct {
	ID     string // letter shortcut for jump-to-row (aa, ab, …)
	Source string // "history", "fzf-marks", or "gnome"
	Name   string
	Path   string
	// PathMissing is true when the path does not exist on disk (or remote VFS).
	PathMissing bool
}

// SearchLine returns the fuzzy-filter key for this item. The source label is a category column,
// not searchable text.
func (i PathPickerItem) SearchLine() string {
	parts := make([]string, 0, 2)
	if i.Name != "" {
		parts = append(parts, i.Name)
	}
	if i.Path != "" {
		parts = append(parts, i.Path)
	}
	return strings.Join(parts, " ")
}

// Empty reports whether the item carries no list-row content.
func (i PathPickerItem) Empty() bool {
	return i.Source == "" && i.Name == "" && i.Path == ""
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
	// Focus is PathPickerFilter ("" = the filter, which also steers the list), dlg.OK or dlg.Cancel.
	Focus dlg.ID
	// QueryPathInvalid is true after a debounced check when the filter looks like a path and os.Lstat fails.
	QueryPathInvalid bool
	// QueryPathCheckPending is true until debounced validation runs after Query changed.
	QueryPathCheckPending bool
	// Completion holds the filesystem-completion dropdown state for Query.
	Completion PathCompletion
}

// FilterFocused reports whether the query input (and with it the list) holds focus.
func (s PathPickerState) FilterFocused() bool { return s.Focus == "" || s.Focus == PathPickerFilter }

// FindEntry is one indexed path in the recursive find dialog.
type FindEntry struct {
	RelLine string
	IsDir   bool
	Type    localfs.EntryType
	Size    int64 // file byte size from index walk; 0 for directories
}

// AbsPath reconstructs the absolute path of the entry from the dialog's root path.
func (e FindEntry) AbsPath(rootPath string) string {
	return filepath.Join(rootPath, filepath.FromSlash(e.RelLine))
}

// FindDialogState is a fuzzy picker over recursively indexed paths under a panel root. Its
// widgets are described by FindDialogView; Focus names the focused one (the Find* IDs, OK or
// Cancel; empty = the query, which also drives the result list).
type FindDialogState struct {
	Open                bool
	PanelID             int
	RootPath            string
	IncludeHidden       bool
	StayOnCurrentVolume bool
	// MaxDepth is the committed index depth limit (path segments below RootPath); 0 = unlimited.
	MaxDepth int
	// DepthInput is the depth field edit buffer: "" or one digit 1-9.
	DepthInput string
	// OnlyDirectories hides non-directory entries from the ranked result list (instant filter).
	OnlyDirectories bool
	// OnlyFiles hides directory entries from the ranked result list (instant filter).
	OnlyFiles bool
	// JumpMode turns Find into "Jump to directory": directories only, breadth-first,
	// no marking, no Only dirs/files radios, no selection scope.
	JumpMode           bool
	ListingDevice      uint64
	ListingDeviceValid bool
	// ShowSearchSelectionsOption is true when the panel had selected directories at open.
	ShowSearchSelectionsOption bool
	// SearchOnlySelections limits indexing to SelectionDirRoots when true.
	SearchOnlySelections bool
	// SelectionDirRoots is a pruned snapshot of selected directory paths at dialog open.
	SelectionDirRoots []string
	// Entries mirrors the coordinator index; UI thread updates only via scan.Event (docs/agents/navigation.md).
	Entries []FindEntry
	// PathMeta resolves isDir and file size by absolute path for marked-selection helpers.
	PathMeta    func(absPath string) (isDir bool, size int64, ok bool)
	Query       string
	QueryCursor int // rune offset of caret within Query (0..len(runes))
	QueryScroll int // first visible rune offset for horizontal scrolling
	Ranked      []int
	MatchRanges map[int][]search.Range // sparse: only entries with actual match ranges
	// RankDisplayLines holds RelLine text parallel to Ranked while indexing before Entries catch up.
	RankDisplayLines []string
	Selected         int
	ListScroll       int
	Focus            dlg.ID
	Indexing         bool
	IndexedCount     int
	// Index BFS level progress: depth, directories read and total at that depth.
	Depth, LevelDone, LevelTotal int
	IndexDone                    bool
	IndexErr                     string
	// RankPending is true while an async background rank is in progress or scheduled.
	RankPending bool
	// FullRanked holds uncapped query-filtered entry indices for bulk select-all / group select.
	FullRanked           []int
	FullRankedGen        int
	FullRankedEntriesLen int
	FullRankedOnlyDirs   bool
	FullRankedOnlyFiles  bool
	// MarkedPaths holds paths toggled selected in the dialog (applied to the panel on OK).
	MarkedPaths map[string]bool
	// markedSelGen bumps on MarkedPaths mutations; drives selection-size derived cache.
	markedSelGen   uint64
	markedSelCache findMarkedSelCache
	// pathIndex maps each entry's cleaned absolute path to its index in Entries, giving
	// PathMeta O(1) lookups instead of a linear scan of Entries. Kept in sync by
	// RebuildPathIndex/ExtendPathIndex as Entries is replaced/appended.
	pathIndex map[string]int
}

// FindEntryAt returns one indexed entry from the UI mirror.
func (s *FindDialogState) FindEntryAt(idx int) (FindEntry, bool) {
	if idx < 0 || idx >= len(s.Entries) {
		return FindEntry{}, false
	}
	return s.Entries[idx], true
}

// RebuildPathIndex rebuilds the absolute-path → Entries-index lookup from scratch.
// Call after replacing or clearing Entries.
func (s *FindDialogState) RebuildPathIndex() {
	s.pathIndex = make(map[string]int, len(s.Entries))
	for i, e := range s.Entries {
		s.pathIndex[filepath.Clean(e.AbsPath(s.RootPath))] = i
	}
}

// ExtendPathIndex adds Entries[from:] to the path index. Call after appending to Entries.
func (s *FindDialogState) ExtendPathIndex(from int) {
	if s.pathIndex == nil {
		s.RebuildPathIndex()
		return
	}
	for i := from; i < len(s.Entries); i++ {
		s.pathIndex[filepath.Clean(s.Entries[i].AbsPath(s.RootPath))] = i
	}
}

// PathIndexLookup returns the Entries index for a cleaned absolute path, or false if unknown.
func (s *FindDialogState) PathIndexLookup(absPath string) (int, bool) {
	i, ok := s.pathIndex[absPath]
	return i, ok
}

// QueryFocused reports whether the query input (and with it the result list it drives) has focus.
func (s FindDialogState) QueryFocused() bool { return s.Focus == "" || s.Focus == FindQuery }

// FindDialogHasSelectionsCheckbox reports whether the search-selections row is shown.
func (s FindDialogState) FindDialogHasSelectionsCheckbox() bool {
	return s.ShowSearchSelectionsOption
}

// HistoryDialogState is a fuzzy picker over one panel’s navigation history paths.
type HistoryDialogState struct {
	Open            bool
	PanelID         int      // PrimaryPanel or SecondaryPanel
	Paths           []string // current list (single panel or merged)
	CurrentPath     string   // PanelID's directory at open (CleanPathString); hidden from the list
	BothPanels      bool     // true when showing merged list
	PanelPaths      []string // snapshot of PanelID history at open (for F5 toggle back)
	Zoxide          []string // zoxide directories (loaded async), appended after the history when IncludeZoxide
	ZoxideAvailable bool     // zoxide is installed; shows the "Include zoxide" checkbox
	IncludeZoxide   bool     // "Include zoxide" checkbox; starts from [history].zoxide
	ZoxideFrom      int      // index into Paths where zoxide rows start (== len(Paths) when none)
	DisplayLines    []string // per-row UI text ("* path" / "  path"); len == len(Paths)
	PathMissing     []bool   // len == len(Paths); true when the path no longer exists on disk
	Query           string
	QueryCursor     int              // rune offset of caret within Query (0..len(runes))
	QueryScroll     int              // first visible rune offset for horizontal scrolling
	Ranked          []int            // indices into Paths / DisplayLines
	MatchRanges     [][]search.Range // len == len(Paths); highlights on DisplayLines
	Selected        int              // index into Ranked
	ListScroll      int
	// Focus is HistoryFilter ("" = the filter, which also steers the path list), HistoryZoxide
	// (when shown), dlg.OK or dlg.Cancel.
	Focus dlg.ID
}

// MetaEntry is one selectable command in the meta picker dialog.
type MetaEntry struct {
	Name        string
	Description string
	// Label replaces Name in the picker row when set (a provider's column title).
	Label string
	// Section is the heading the entry is listed under; "" is the top (meta.toml) section.
	// Entries are ordered section by section.
	Section string
	// SortOnActivation switches the panel's sort to this column when the entry is newly
	// activated from the meta picker.
	SortOnActivation bool
	// SortReverse reverses that sort. Ignored when SortOnActivation is false.
	SortReverse bool
}

// MetaDialogState is the checkbox picker for toggling meta columns on panel entries.
type MetaDialogState struct {
	Open    bool
	PanelID int
	Entries []MetaEntry
	Checked []bool // parallel to Entries
	Focus   dlg.ID // MetaEntryID(i) for a checkbox row, dlg.OK or dlg.Cancel; empty is the first row
	// Notice explains an empty Entries list (no meta.toml, an untrusted or invalid one); drawn
	// wrapped in the warning style in place of the checkbox rows.
	Notice string
}

// HelpEntry is one row in the full-screen help view.
type HelpEntry struct {
	ActionID string // keymap action id (e.g. file.copy)
	Title    string // "Copy"
	Keys     string // "F5"
	Section  string // "File"
	// FuzzyExtra is action id and keywords for the rank-only corpus (after Title), space-separated.
	FuzzyExtra string
}

// HelpViewState holds state for the centered help dialog with fuzzy search.
type HelpViewState struct {
	Open        bool
	Title       string // dialog title, e.g. "Help" or "Help — Jobs"
	Query       string
	QueryCursor int // rune offset of caret within Query (0..len(runes))
	QueryScroll int // first visible rune offset for horizontal scrolling
	Entries     []HelpEntry
	Ranked      []int            // indices into Entries (rank order)
	MatchRanges [][]search.Range // len == len(Entries); rune ranges on the painted padded row
	Selected    int              // index into Ranked
	ListScroll  int              // first visible row index into Ranked
	Focus       dlg.ID           // HelpFilter (the filter steering the list; "" too) or HelpClose
	TextEdit    bool             // true on the dialog text-input editing keys page
}
