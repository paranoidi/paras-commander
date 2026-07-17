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
	PrimaryModalDedupEmptyDirs
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
	FlattenIntoDest      bool // last content row when MultiLocation(); copies as dest/<basename>
	FocusField           int  // content indices then OK, Add paused, Cancel; see TransferDialogLinearForm
	SelfCopyDestDir      string
	SelfCopyOrigBasename string
	SelfCopyNewName      FileDialogField
	// DestPathInvalid is true after a debounced check when the destination looks like a path and os.Lstat fails.
	DestPathInvalid bool
	// DestPathCheckPending is true until debounced validation runs after Destination.Value changed.
	DestPathCheckPending bool

	// CommonRoot is the canonical common-root path of a selection spanning multiple
	// directories (set when the transfer is issued away from that root); empty otherwise.
	// Non-empty CommonRoot switches the destination phase into the multi-location layout
	// (source/root header, preview list, flatten checkbox). See MultiLocation.
	CommonRoot string
	// Entries previews the selections relative to CommonRoot (non-recursive, root-relative
	// or basename labels depending on FlattenIntoDest).
	Entries       []DeleteListEntry
	EntriesScroll int
}

// MultiLocation reports whether the destination phase should show the multi-location
// layout (common-root header, preview list, Flatten into destination checkbox) —
// i.e. the transfer was issued on a selection spanning multiple directories away
// from their common root.
func (st TransferDialogState) MultiLocation() bool {
	return st.CommonRoot != "" && st.Phase == TransferPhaseDestination
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

// TransferDialogEffectiveNumContent returns the focusable content count for the current
// dialog screen: self-copy rename = 1; destination phase = destination row plus, for
// copy, the two preserve checkboxes; plus one more (Flatten into destination, always
// last) when MultiLocation().
func TransferDialogEffectiveNumContent(st TransferDialogState) int {
	if st.Phase == TransferPhaseSelfCopyRename {
		return 1
	}
	n := 1 // destination
	if st.Kind == TransferKindCopy {
		n = 3 // destination + two preserve checkboxes
	}
	if st.MultiLocation() {
		n++
	}
	return n
}
