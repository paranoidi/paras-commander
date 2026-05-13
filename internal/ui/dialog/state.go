package dialog

import (
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
)

// ThemeChoice describes a theme option rendered in the selection dialog.
type ThemeChoice struct {
	Name  string
	Label string
}

// MessageDialogState is a generic modal with a title, body text, and OK or OK/Cancel buttons.
type MessageDialogState struct {
	Open        bool
	Title       string
	Message     string
	TwoButtons  bool
	ButtonFocus int // 0=OK, 1=Cancel when TwoButtons
}

// ThemeDialogState is the renderable state for the theme selection modal.
type ThemeDialogState struct {
	Open        bool
	Selected    int
	Focus       int // 0=list, 1=OK button, 2=Cancel button
	CurrentName string
	Choices     []ThemeChoice
}

// ConfigDialogState is the Options → Configuration modal (runtime UI toggles persisted to config.toml).
type ConfigDialogState struct {
	Open          bool
	ShowFileIcons bool
	Focus         int // 0=checkbox, 1=OK, 2=Cancel
}

// SortDialogState is the renderable state for the sort configuration modal.
type SortDialogState struct {
	Open                  bool
	SortMode              panel.SortMode
	SortReverse           bool
	DirectoriesFirst      bool
	DiskUsageIdleSizeSort bool
	Focus                 int // 0-3=radios, 4=disk idle sort, 5=reverse, 6=dirs first, 7=OK, 8=Cancel
	PanelID               int // LeftPanel or RightPanel
}

// GroupSelectState is the renderable state for the group selection input modal.
type GroupSelectState struct {
	Open             bool
	Text             string
	Mode             string // "select" or "unselect"
	FilesOnly        bool
	CaseSensitive    bool
	UseShellPatterns bool
	Focus            int // 0=pattern input, 1=Files only, 2=Case sensitive, 3=Using shell patterns, 4=OK, 5=Cancel
}

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

// FileDialogType identifies which file operation dialog is active.
type FileDialogType int

const (
	FileDialogNone FileDialogType = iota
	FileDialogRename
	FileDialogMkdir
	FileDialogDelete
	FileDialogChmod
	FileDialogChown
	FileDialogSymlink
	FileDialogHardlink
	FileDialogAddBookmark
	FileDialogRunForEach
)

// FileDialogField is a single input field in a file operation dialog.
type FileDialogField struct {
	Label   string
	Value   string
	Prefill string
	Cursor  int
	// PrefillPending is true while Value still shows the suggested default (Prefill).
	// The first printable character clears and replaces; Backspace/arrow/home/end/delete
	// commits the suggestion so the user edits it in place.
	PrefillPending bool
	// PathPicker enables a trailing glyph and path-picker sub-focus on the input row.
	PathPicker bool
	// PickerFocused is true when the trailing path-picker glyph has sub-focus (file dialogs).
	PickerFocused bool
}

// MkdirAction identifies the post-mkdir action chosen via radio buttons in the mkdir dialog.
// Only meaningful when DialogType == FileDialogMkdir and MkdirShowActions == true.
type MkdirAction int

const (
	MkdirActionCreate           MkdirAction = iota // just create the directory
	MkdirActionCreateCopySelect                    // create and queue copy of current selection into it
	MkdirActionCreateMoveSelect                    // create and queue move of current selection into it
)

// FileDialogState holds state for any file operation dialog.
type FileDialogState struct {
	Open         bool
	DialogType   FileDialogType
	Fields       []FileDialogField
	FocusedField int
	Message      string
	// RunForEachPaths / RunForEachDir apply when DialogType == FileDialogRunForEach (targets resolved at dialog open).
	RunForEachPaths []string
	RunForEachDir   string
	// MkdirShowActions enables the extra "Create / Create and copy selected / Create and move selected" radio
	// rows below the directory-name input. Set by openMkdirDialog when the active panel has selections.
	MkdirShowActions bool
	// MkdirAction is the currently selected mkdir post-action (only meaningful when MkdirShowActions is true).
	MkdirAction MkdirAction
}

// PrimaryModal identifies which exclusive modal occupies the primary dialog layer.
// Overlay modals (Sort, GroupSelect, FileDialog) may draw on top; see Render.
type PrimaryModal int

const (
	PrimaryModalNone PrimaryModal = iota
	PrimaryModalTheme
	PrimaryModalTransfer
	PrimaryModalConflict
	PrimaryModalQuit
)

// TransferKind selects copy vs move in the shared destination dialog.
type TransferKind int

const (
	TransferKindCopy TransferKind = iota
	TransferKindMove
)

// TransferDialogPhase selects the copy/move dialog screen.
type TransferDialogPhase uint8

const (
	// TransferPhaseDestination is the normal destination path (+ copy options).
	TransferPhaseDestination TransferDialogPhase = iota
	// TransferPhaseSelfCopyRename prompts for an alternate basename when the item would copy/move onto itself.
	TransferPhaseSelfCopyRename
)

// Destination sub-focus for path input row (text vs trailing path-picker glyph).
const (
	TransferDestSubFocusText = iota
	TransferDestSubFocusPicker
)

// TransferDialogState holds the copy/move destination dialog (shared chrome and navigation).
type TransferDialogState struct {
	Open                 bool
	Kind                 TransferKind
	Phase                TransferDialogPhase
	Destination          FileDialogField
	DestSubFocus         int  // TransferDestSubFocus* when Phase==TransferPhaseDestination and FocusField==0
	PreservePermissions  bool // copy only
	PreserveTimestamps   bool // copy only
	FocusField           int  // content indices then OK, Add paused, Cancel; see TransferDialogLinearForm
	SelfCopyDestDir      string
	SelfCopyOrigBasename string
	SelfCopyNewName      FileDialogField
	// DestPathInvalid is true after a debounced check when the destination looks like a path and os.Lstat fails.
	DestPathInvalid bool
	// DestPathCheckPending is true until debounced validation runs after Destination.Value changed.
	DestPathCheckPending bool
}

// ConflictDialogState holds the conflict resolution dialog.
type ConflictDialogState struct {
	Open        bool
	JobID       string
	Source      string
	Destination string
	Focus       int // 0=overwrite, 1=skip, 2=overwrite-all, 3=skip-all, 4=cancel
}

// QuitConfirmState holds the quit confirmation dialog.
type QuitConfirmState struct {
	Open  bool
	Focus int // 0=stay, 1=quit-anyway
	// WarnLine1 / WarnLine2 override copy when non-empty (e.g. active commands vs jobs only).
	WarnLine1 string
	WarnLine2 string
}

// TransferDialogNumContent returns the number of focusable content rows before OK/Cancel.
func TransferDialogNumContent(kind TransferKind) int {
	if kind == TransferKindCopy {
		return 3 // destination + two checkboxes
	}
	return 1 // destination only
}

// TransferDialogEffectiveNumContent returns the focusable content count for the current dialog screen.
func TransferDialogEffectiveNumContent(st TransferDialogState) int {
	if st.Phase == TransferPhaseSelfCopyRename {
		return 1
	}
	return TransferDialogNumContent(st.Kind)
}
