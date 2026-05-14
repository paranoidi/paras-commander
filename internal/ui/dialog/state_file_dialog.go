package dialog

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

// RenamePhase selects the rename dialog screen (main name field vs tool sub-dialogs).
type RenamePhase int

const (
	RenamePhaseMain RenamePhase = iota
	RenamePhaseSanitize
	RenamePhaseSlugify
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

	// RenamePhase and the following fields apply when DialogType == FileDialogRename.
	RenamePhase               RenamePhase
	RenameSanitizeDots        bool
	RenameSanitizeUnderscores bool
	RenameSlugifySep          RenameSlugifySep
}
