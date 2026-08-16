package dialog

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/search"
)

// FileDialogType identifies which file operation dialog is active.
type FileDialogType int

const (
	FileDialogNone FileDialogType = iota
	FileDialogRename
	FileDialogDuplicate
	FileDialogMkdir
	FileDialogDelete
	FileDialogChmod
	FileDialogChown
	FileDialogSymlink
	FileDialogHardlink
	FileDialogAddBookmark
	FileDialogRunForEach
	FileDialogMassRename
	FileDialogExtract
	FileDialogSFTPConnect
	FileDialogSFTPPassword
)

// MassRenameModeUI selects literal vs regexp transform in the mass rename dialog.
type MassRenameModeUI int

const (
	MassRenameModeUISimple MassRenameModeUI = iota
	MassRenameModeUIRegex
	MassRenameModeUIExternalEditor
	MassRenameModeUICapitalize
)

// MassRenameSource is one selected file (absolute path resolved when the dialog opens).
type MassRenameSource struct {
	Path string
	Name string // basename
}

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
	// InputInvalid paints the row with dialog.input.*.error (e.g. mass rename regexp compile error or no matches).
	InputInvalid bool
	// CompletionSuffix is ghost filesystem completion after the caret (Tab accepts).
	CompletionSuffix string
	// CompletionIsDir is true when accepting completion should append a trailing slash.
	CompletionIsDir bool
	// Scroll is the first visible rune offset for path rows with horizontal overflow.
	Scroll int
}

// MkdirAction identifies the post-mkdir action chosen via radio buttons in the mkdir dialog.
// Only meaningful when DialogType == FileDialogMkdir and MkdirShowActions == true.
type MkdirAction int

const (
	MkdirActionCreate           MkdirAction = iota // just create the directory
	MkdirActionCreateCopySelect                    // create and queue copy of current selection into it
	MkdirActionCreateMoveSelect                    // create and queue move of current selection into it
)

// RenamePhase selects the rename dialog screen (main name field vs tool sub-dialogs).
type RenamePhase int

const (
	RenamePhaseMain RenamePhase = iota
	RenamePhaseSanitize
	RenamePhaseSlugify
	RenamePhaseEncoding
)

// RenameEncodingCandidate is one legacy-encoding decode offered in the rename encoding tool.
type RenameEncodingCandidate struct {
	Label string
	UTF8  string
}

// MassRenamePhase selects the mass-rename dialog screen (main find/replace form vs the
// save-pattern prompt, load-pattern picker, and pattern-history picker sub-screens).
type MassRenamePhase int

const (
	MassRenamePhaseMain MassRenamePhase = iota
	MassRenamePhaseSavePrompt
	MassRenamePhaseLoadPicker
	MassRenamePhaseHistoryPicker
)

// FileDialogState holds state for any file operation dialog.
type FileDialogState struct {
	Open         bool
	DialogType   FileDialogType
	Fields       []FileDialogField
	FocusedField int
	Message      string
	// RunForEachPaths / RunForEachDir apply when DialogType == FileDialogRunForEach (targets resolved at dialog open).
	RunForEachEntries []localfs.Entry
	RunForEachDir     string
	// RunForEachPools is the configured pool list; when non-empty, the dialog renders a pool selector.
	RunForEachPools []string
	// RunForEachPool is the selected pool name; empty means no pool (unlimited parallelism).
	RunForEachPool string
	// RunForEachCommandError is shown under the Command input when validation fails.
	RunForEachCommandError string
	// RunForEachPreview is the expanded command line for the first selected item, shown under
	// the Command input while it is valid (mutually exclusive with RunForEachCommandError).
	RunForEachPreview string
	// RunForEachInDirs runs the command with cwd set to each selected directory itself instead
	// of the panel's fixed directory with %f as an argument. Selection must be directories-only
	// when set.
	RunForEachInDirs bool
	// RunForEachPTY runs each entry attached to a live pseudo-TTY, shown interactively in the
	// Commands view, instead of capturing stdout/stderr non-interactively. Lets tools that
	// open /dev/tty for prompts (e.g. chezmoi) work under run-for-each.
	RunForEachPTY bool
	// ExtractSources apply when DialogType == FileDialogExtract (archive paths resolved at dialog open).
	ExtractSources []string
	// MkdirShowActions enables the extra "Create / Create and copy selected / Create and move selected" radio
	// rows below the directory-name input. Set by openMkdirDialog when the active panel has selections.
	MkdirShowActions bool
	// MkdirAction is the currently selected mkdir post-action (only meaningful when MkdirShowActions is true).
	MkdirAction MkdirAction
	// MkdirOpenInInactive opens the new directory in the inactive panel after a successful create.
	MkdirOpenInInactive bool

	// DuplicateSource is the directory path copied by FileDialogDuplicate.
	DuplicateSource string

	// RenamePhase and the following fields apply when DialogType uses rename-like phases
	// (FileDialogRename, FileDialogDuplicate).
	RenamePhase               RenamePhase
	RenameSanitizeDots        bool
	RenameSanitizeUnderscores bool
	RenameSlugifySep          RenameSlugifySep
	// RenameFocusAfter selects and centers the renamed entry after OK (single-file rename main dialog only).
	RenameFocusAfter bool
	// RenameEncodingCandidates / RenameEncodingSelected apply when opening rename with detectable legacy encodings.
	RenameEncodingCandidates []RenameEncodingCandidate
	RenameEncodingSelected   int

	// Mass rename (DialogType == FileDialogMassRename).
	MassRenameMode             MassRenameModeUI
	MassRenameCaseFold         bool
	MassRenameStripSpaces      bool
	MassRenameShowOnlyModified bool
	// MassRenameCapEachWord / MassRenameCapPunctSep apply when MassRenameMode ==
	// MassRenameModeUICapitalize: capitalize every word (vs. only the first letter of the
	// whole name) and treat ',', '.', '_' as word separators in addition to whitespace/'-'.
	MassRenameCapEachWord   bool
	MassRenameCapPunctSep   bool
	MassRenamePreviewScroll int
	MassRenameSources       []MassRenameSource
	// MassRenamePreviewBefore / After are paired basename preview columns (recomputed in app).
	// Rows with Before starting with "!" are full-width compute-error lines (After empty).
	MassRenamePreviewBefore         []string
	MassRenamePreviewAfter          []string
	MassRenamePreviewBeforeRemoved  [][]search.Range
	MassRenamePreviewBeforeReplaced [][]search.Range
	MassRenamePreviewAfterAdded     [][]search.Range
	// MassRenamePreviewAfterError marks after-column rows with a validation conflict (per-row).
	MassRenamePreviewAfterError []bool
	// MassRenamePatternCompileHint is a short regexp compile error shown under the Pattern field (regex mode).
	MassRenamePatternCompileHint string
	// MassRenameReplacementSyntaxHint is shown under the Replacement field when the pattern has capture groups.
	MassRenameReplacementSyntaxHint string
	// MassRenameExternalNames holds the per-file basenames returned by the external editor (ExternalEditor mode).
	// Nil means the editor has not been run yet.
	MassRenameExternalNames []string
	// MassRenamePhase selects between the main find/replace form and the save/load/history
	// pattern sub-screens (see MassRenamePhase).
	MassRenamePhase MassRenamePhase
	// MassRenameSavedFields stashes d.Fields while the save-pattern prompt owns Name/Description.
	MassRenameSavedFields []FileDialogField
	// MassRenameLoadPicker holds the fuzzy-filtered saved-patterns list while
	// MassRenamePhase == MassRenamePhaseLoadPicker.
	MassRenameLoadPicker MassRenamePatternPickerState
	// MassRenameHistoryPicker holds the fuzzy-filtered in-memory recently-used pattern list while
	// MassRenamePhase == MassRenamePhaseHistoryPicker. Same widget shape as MassRenameLoadPicker,
	// backed by a different, session-only item source (see apphandler/dialog's massRenameHistory).
	MassRenameHistoryPicker MassRenamePatternPickerState

	// Delete confirmation (DialogType == FileDialogDelete).
	DeleteSummary        string
	DeleteWarning        string
	DeleteEntries        []DeleteListEntry
	DeleteListScroll     int
	DeleteLayoutMinWidth int // cached at open; avoids scanning all entries each frame
	// DeleteDanglingDirs marks this delete confirmation as the post-move/delete
	// "remove directories left empty" prompt, routed in executeDelete before the
	// normal ViewMode branches (entries are already-resolved directory paths, not
	// a fresh panel selection).
	DeleteDanglingDirs bool
}

// DeleteListEntry is one row in the delete confirmation name list.
type DeleteListEntry struct {
	Name string
	Path string
	Type localfs.EntryType
}
