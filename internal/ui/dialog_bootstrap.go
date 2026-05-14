package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/lineedit"
)

// Dialog types and helpers are implemented in package dialog; these aliases and
// function bindings keep a single stable import path (ui) for application code.

type (
	ThemeChoice               = dialog.ThemeChoice
	MessageDialogState        = dialog.MessageDialogState
	ThemeDialogState          = dialog.ThemeDialogState
	ConfigDialogState         = dialog.ConfigDialogState
	SortDialogState           = dialog.SortDialogState
	ListingFormatDialogState  = dialog.ListingFormatDialogState
	GroupSelectState          = dialog.GroupSelectState
	PathPickerPurpose         = dialog.PathPickerPurpose
	PathPickerItem            = dialog.PathPickerItem
	PathPickerState           = dialog.PathPickerState
	HistoryDialogState        = dialog.HistoryDialogState
	MetaEntry                 = dialog.MetaEntry
	MetaDialogState           = dialog.MetaDialogState
	UserMenuDialogState       = dialog.UserMenuDialogState
	HelpEntry                 = dialog.HelpEntry
	HelpViewState             = dialog.HelpViewState
	FileDialogType            = dialog.FileDialogType
	FileDialogField           = dialog.FileDialogField
	MkdirAction               = dialog.MkdirAction
	RenamePhase               = dialog.RenamePhase
	RenameSlugifySep          = dialog.RenameSlugifySep
	FileDialogState           = dialog.FileDialogState
	MassRenameModeUI          = dialog.MassRenameModeUI
	MassRenameSource          = dialog.MassRenameSource
	PrimaryModal              = dialog.PrimaryModal
	TransferKind              = dialog.TransferKind
	TransferDialogPhase       = dialog.TransferDialogPhase
	TransferDialogState       = dialog.TransferDialogState
	ConflictDialogState       = dialog.ConflictDialogState
	QuitConfirmState          = dialog.QuitConfirmState
	DialogTrailingButtonsForm = dialog.DialogTrailingButtonsForm
	DialogLinearForm          = dialog.DialogLinearForm
	TransferDialogLinearForm  = dialog.TransferDialogLinearForm
	DialogButtonSpec          = dialog.DialogButtonSpec
)

const (
	PathPickerPurposeNavigate                 = dialog.PathPickerPurposeNavigate
	PathPickerPurposeApplyTransferDestination = dialog.PathPickerPurposeApplyTransferDestination
	PathPickerPurposeApplyFileDialogField     = dialog.PathPickerPurposeApplyFileDialogField

	FileDialogNone        = dialog.FileDialogNone
	FileDialogRename      = dialog.FileDialogRename
	FileDialogMkdir       = dialog.FileDialogMkdir
	FileDialogDelete      = dialog.FileDialogDelete
	FileDialogChmod       = dialog.FileDialogChmod
	FileDialogChown       = dialog.FileDialogChown
	FileDialogSymlink     = dialog.FileDialogSymlink
	FileDialogHardlink    = dialog.FileDialogHardlink
	FileDialogAddBookmark = dialog.FileDialogAddBookmark
	FileDialogRunForEach  = dialog.FileDialogRunForEach
	FileDialogMassRename  = dialog.FileDialogMassRename

	MassRenameModeUISimple = dialog.MassRenameModeUISimple
	MassRenameModeUIRegex  = dialog.MassRenameModeUIRegex

	MkdirActionCreate           = dialog.MkdirActionCreate
	MkdirActionCreateCopySelect = dialog.MkdirActionCreateCopySelect
	MkdirActionCreateMoveSelect = dialog.MkdirActionCreateMoveSelect

	RenamePhaseMain     = dialog.RenamePhaseMain
	RenamePhaseSanitize = dialog.RenamePhaseSanitize
	RenamePhaseSlugify  = dialog.RenamePhaseSlugify

	RenameSlugifyDot        = dialog.RenameSlugifyDot
	RenameSlugifyUnderscore = dialog.RenameSlugifyUnderscore

	PrimaryModalNone     = dialog.PrimaryModalNone
	PrimaryModalTheme    = dialog.PrimaryModalTheme
	PrimaryModalTransfer = dialog.PrimaryModalTransfer
	PrimaryModalConflict = dialog.PrimaryModalConflict
	PrimaryModalQuit     = dialog.PrimaryModalQuit

	TransferKindCopy = dialog.TransferKindCopy
	TransferKindMove = dialog.TransferKindMove

	TransferPhaseDestination    = dialog.TransferPhaseDestination
	TransferPhaseSelfCopyRename = dialog.TransferPhaseSelfCopyRename

	TransferDestSubFocusText   = dialog.TransferDestSubFocusText
	TransferDestSubFocusPicker = dialog.TransferDestSubFocusPicker
)

var (
	AltDialogOK                       = dialog.AltDialogOK
	AltDialogCancel                   = dialog.AltDialogCancel
	ListOKCancelNavFocusKey           = dialog.ListOKCancelNavFocusKey
	ListClampedSelectionDelta         = dialog.ListClampedSelectionDelta
	EnsurePathPickerListScroll        = dialog.EnsurePathPickerListScroll
	EnsureHistoryListScroll           = dialog.EnsureHistoryListScroll
	EnsureScrollInputVisible          = dialog.EnsureScrollInputVisible
	NewDialogLinearForm               = dialog.NewDialogLinearForm
	NewDialogTrailingButtonsForm      = dialog.NewDialogTrailingButtonsForm
	NewTransferDialogLinearForm       = dialog.NewTransferDialogLinearForm
	TransferDialogNumContent          = dialog.TransferDialogNumContent
	TransferDialogEffectiveNumContent = dialog.TransferDialogEffectiveNumContent
	TransferDialogMoveFocus           = dialog.TransferDialogMoveFocus
	DialogPairLeftRight               = dialog.DialogPairLeftRight
	IsWordRune                        = lineedit.IsWordRune
	BackwardWordIndex                 = lineedit.BackwardWordIndex
	ForwardWordIndex                  = lineedit.ForwardWordIndex
	KillWordBackward                  = lineedit.KillWordBackward
)

// AccentGlyphStyle applies menu/dialog shortcut accent styling (delegates to dialog).
func AccentGlyphStyle(base, accent tcell.Style) tcell.Style {
	return dialog.AccentGlyphStyle(base, accent)
}

// ThemeDialogListViewportRows is how many theme rows fit in the theme dialog list column (delegates to dialog).
func ThemeDialogListViewportRows(layout Layout, choiceCount int) int {
	return dialog.ThemeDialogListViewportRows(layout, choiceCount)
}

// FileDialogOKFocusIndex returns the FocusedField index of the OK button (delegates to dialog).
func FileDialogOKFocusIndex(st FileDialogState) int {
	return dialog.FileDialogOKFocusIndex(st)
}

// FileDialogCancelFocusIndex returns the FocusedField index of the Cancel button (delegates to dialog).
func FileDialogCancelFocusIndex(st FileDialogState) int {
	return dialog.FileDialogCancelFocusIndex(st)
}

// MassRenamePreviewViewportRows returns how many preview lines fit for a terminal height.
func MassRenamePreviewViewportRows(layoutHeight int) int {
	return dialog.MassRenamePreviewViewportRows(layoutHeight)
}

// MassRenameEnsurePreviewScroll clamps MassRenamePreviewScroll to keep the viewport valid.
func MassRenameEnsurePreviewScroll(st *FileDialogState, viewportRows, totalRows int) {
	dialog.MassRenameEnsurePreviewScroll(st, viewportRows, totalRows)
}

// ApplyRenameSanitize applies dot/underscore-to-space cleanups (delegates to dialog).
func ApplyRenameSanitize(s string, dotsToSpace, underscoresToSpace bool) string {
	return dialog.ApplyRenameSanitize(s, dotsToSpace, underscoresToSpace)
}

// ApplyRenameSlugify replaces ASCII spaces with the chosen separator (delegates to dialog).
func ApplyRenameSlugify(s string, sep RenameSlugifySep) string {
	return dialog.ApplyRenameSlugify(s, dialog.RenameSlugifySep(sep))
}
