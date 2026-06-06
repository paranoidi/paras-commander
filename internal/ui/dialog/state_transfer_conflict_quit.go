package dialog

import "github.com/paranoidi/paras-commander/internal/jobs"

// PrimaryModal identifies which exclusive modal occupies the primary dialog layer.
// Overlay modals (Sort, GroupSelect, FileDialog) may draw on top; see Render.
type PrimaryModal int

const (
	PrimaryModalNone PrimaryModal = iota
	PrimaryModalTheme
	PrimaryModalTransfer
	PrimaryModalFlatten
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

// ConflictDialogState holds the quick job-blocker answer dialog (Ctrl+Q).
type ConflictDialogState struct {
	Open    bool
	JobID   string
	Blocker jobs.BlockerDetails
	Focus   int // button index; see ui.JobBlockerDialogMaxFocus
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
