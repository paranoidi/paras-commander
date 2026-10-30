package dialog

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

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
	PrimaryModalUndo
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

// TransferDialogState holds the copy/move destination dialog (shared chrome and navigation).
type TransferDialogState struct {
	Open                bool
	Kind                TransferKind
	Phase               TransferDialogPhase
	Destination         FileDialogField
	PreservePermissions bool // copy only
	PreserveTimestamps  bool // copy only
	DereferenceSymlinks bool // copy only
	SourceIsRemote      bool // copy only; true when the source panel is SFTP
	FlattenIntoDest     bool // last content row when MultiLocation(); copies as dest/<basename>
	// Focus is the focused widget (TransferDest, TransferNewName, a checkbox ID, dlg.OK,
	// TransferAdvanced or dlg.Cancel); empty is the first widget (the destination or new name input).
	Focus dlg.ID
	// Advanced switches the dialog to the Conflict rules screen (Rules) with the Add paused and
	// Remove empty directories checkboxes; AdvFocus is its focused widget.
	Advanced             bool
	AdvFocus             dlg.ID
	Rules                ConflictDialogState // seeded by StartAdvanced; only its Rules and Blocker are used
	StartPaused          bool
	RemoveEmptyDirs      bool // move only
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

// StartAdvanced switches to the Conflict rules screen. The rules are seeded the first time only,
// so leaving and re-entering keeps the user's choices.
func (st *TransferDialogState) StartAdvanced() {
	if st.Rules.Blocker.Conflict == nil {
		st.Rules.StartAdvanced()
		st.Rules.Blocker = jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{Move: st.Kind == TransferKindMove}}
	}
	st.Advanced = true
	st.AdvFocus = ConflictRuleID(0, 0)
}

// RulesChanged reports whether the user picked rules other than the untouched default.
func (st TransferDialogState) RulesChanged() bool {
	return st.Rules.Blocker.Conflict != nil && st.Rules.Rules != (jobs.ConflictRules{Differ: jobs.RuleFallback})
}

// MultiLocation reports whether the destination phase should show the multi-location
// layout (common-root header, preview list, Flatten into destination checkbox) —
// i.e. the transfer was issued on a selection spanning multiple directories away
// from their common root.
func (st TransferDialogState) MultiLocation() bool {
	return st.CommonRoot != "" && st.Phase == TransferPhaseDestination
}

// DereferenceUnsupported reports whether "Dereference symlinks" should be disabled:
// a non-local scheme (SFTP, fileindex) has no inode-equivalent identity to dereference against,
// on either side of the copy.
func (st TransferDialogState) DereferenceUnsupported() bool {
	return st.SourceIsRemote || pathloc.HasURIPrefix(strings.TrimSpace(st.Destination.Value))
}

// ConflictDialogState holds the quick job-blocker answer dialog (Ctrl+Q).
type ConflictDialogState struct {
	Open    bool
	JobID   string
	Blocker jobs.BlockerDetails
	Focus   dlg.ID // a button of the File exists / Disk space screen ("" = the first)

	// Advanced switches the dialog to "Conflict rules": the time and size radio rows, the Verify
	// checkbox with its Same / Differ rows, the apply-to-all checkbox, and OK/Cancel (Rules).
	Advanced bool
	AdvFocus dlg.ID // a ConflictRuleID radio, ConflictVerify, ConflictApplyAll, dlg.OK or dlg.Cancel
	Rules    jobs.ConflictRules
	AdvAll   bool // apply the rules to all remaining conflicts
	// OpenedFromPanel: the advanced dialog came from the jobs-view panel, so Cancel closes
	// instead of returning to the main dialog.
	OpenedFromPanel bool
}

// QuitConfirmState holds the quit confirmation dialog.
type QuitConfirmState struct {
	Open  bool
	Focus dlg.ID // QuitStay (the default) or QuitQuit
	// WarnLine1 / WarnLine2 override copy when non-empty (e.g. active commands vs jobs only).
	WarnLine1 string
	WarnLine2 string
}

// FocusOnDest reports whether the destination input has focus (destination phase).
func (st TransferDialogState) FocusOnDest() bool {
	return st.Phase == TransferPhaseDestination && (st.Focus == TransferDest || st.Focus == "")
}

// FocusOnInput reports whether the phase's text input (destination, or the new name of the
// self-copy phase) has focus.
func (st TransferDialogState) FocusOnInput() bool {
	if st.Phase == TransferPhaseSelfCopyRename {
		return st.Focus == TransferNewName || st.Focus == ""
	}
	return st.FocusOnDest()
}
