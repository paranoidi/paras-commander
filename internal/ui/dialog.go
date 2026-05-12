package ui

import (
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
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

// JobEntry is a renderable job summary for the jobs view.
type JobEntry struct {
	ID          string
	Type        string
	Status      string
	Sources     []string
	Destination string
	CurrentPath string
	DoneFiles   int
	TotalFiles  int
	DoneBytes   int64
	TotalBytes  int64
	Error       string
	StartedAt   time.Time
	FinishedAt  time.Time
	// ETABytesPerSec is smoothed throughput from recent progress samples (bytes/s).
	ETABytesPerSec float64
	// ETAFilesPerSec is smoothed completion rate from recent progress samples (files/s).
	ETAFilesPerSec float64
	// DisplaySpeedBPS is slower-smoothed B/s for the Queue Speed column.
	DisplaySpeedBPS float64
	// ThroughputSamples is a snapshot of recent instantaneous B/s with timestamps (details chart).
	ThroughputSamples []jobs.ThroughputSample
	// PendingBlocker is set when the job waits on a conflict or disk-space prompt.
	PendingBlocker *jobs.BlockerDetails
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

// dialogButtonLabelRunePadding is extra width beyond utf8.RuneCountInString(label) for drawDialogButton (spaces and brackets).
const dialogButtonLabelRunePadding = 6

func dialogButtonWidth(label string) int {
	return utf8.RuneCountInString(label) + dialogButtonLabelRunePadding
}

// drawDialogButton renders a single button with its shortcut letter highlighted.
// shortcut is the letter inside label to highlight (e.g. 'O' for "OK").
// Output shape: space, "[", space, label, space, "]", space so theme backgrounds cover the chrome.
// Returns the rendered width in rune columns.
func drawDialogButton(screen tcell.Screen, x, y int, label string, shortcut rune, focused bool, styles theme.Theme) int {
	var baseStyle tcell.Style
	if focused {
		baseStyle = styles.DialogButtonActive
	} else {
		baseStyle = styles.DialogButtonNormal
	}
	out := x
	primitive.Text(screen, out, y, 1, " ", baseStyle)
	out++
	primitive.Text(screen, out, y, 1, "[", baseStyle)
	out++
	primitive.Text(screen, out, y, 1, " ", baseStyle)
	out++

	highlighted := false
	for _, r := range label {
		style := baseStyle
		if !highlighted && (r == shortcut || r == unicode.ToUpper(shortcut) || r == unicode.ToLower(shortcut)) {
			style = accentGlyphStyle(baseStyle, styles.DialogAccent)
			highlighted = true
		}
		screen.SetContent(out, y, r, nil, style)
		out++
	}

	primitive.Text(screen, out, y, 1, " ", baseStyle)
	out++
	primitive.Text(screen, out, y, 1, "]", baseStyle)
	out++
	primitive.Text(screen, out, y, 1, " ", baseStyle)
	out++

	return out - x
}
