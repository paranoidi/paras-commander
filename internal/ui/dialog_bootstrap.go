package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/lineedit"
)

// Dialog types and helpers are implemented in package dialog; these aliases and
// function bindings keep a single stable import path (ui) for application code.

const PreferredFormDialogWidth = dialog.PreferredFormDialogWidth

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
	FindEntry                 = dialog.FindEntry
	FindDialogState           = dialog.FindDialogState
	MetaEntry                 = dialog.MetaEntry
	MetaDialogState           = dialog.MetaDialogState
	UserMenuDialogState       = dialog.UserMenuDialogState
	HelpEntry                 = dialog.HelpEntry
	HelpViewState             = dialog.HelpViewState
	ScrollingQuery            = dialog.ScrollingQuery
	FileDialogType            = dialog.FileDialogType
	FileDialogField           = dialog.FileDialogField
	MkdirAction               = dialog.MkdirAction
	MkdirActionRadioSpec      = dialog.MkdirActionRadioSpec
	RenamePhase               = dialog.RenamePhase
	RenameSlugifySep          = dialog.RenameSlugifySep
	FileDialogState           = dialog.FileDialogState
	MassRenameModeUI          = dialog.MassRenameModeUI
	MassRenameSource          = dialog.MassRenameSource
	DeleteListEntry           = dialog.DeleteListEntry
	PrimaryModal              = dialog.PrimaryModal
	TransferKind              = dialog.TransferKind
	TransferDialogPhase       = dialog.TransferDialogPhase
	TransferDialogState       = dialog.TransferDialogState
	FlattenDialogState        = dialog.FlattenDialogState
	FlattenDialogLinearForm   = dialog.FlattenDialogLinearForm
	ConflictDialogState       = dialog.ConflictDialogState
	HostKeyDialogState        = dialog.HostKeyDialogState
	SFTPConnectDialogState    = dialog.SFTPConnectDialogState
	QuitConfirmState          = dialog.QuitConfirmState
	StashRestoreDialogState   = dialog.StashRestoreDialogState
	DialogTrailingButtonsForm = dialog.DialogTrailingButtonsForm
	DialogLinearForm          = dialog.DialogLinearForm
	TransferDialogLinearForm  = dialog.TransferDialogLinearForm
	DialogButtonSpec          = dialog.DialogButtonSpec
)

const (
	PathPickerPurposeNavigate                 = dialog.PathPickerPurposeNavigate
	PathPickerPurposeApplyTransferDestination = dialog.PathPickerPurposeApplyTransferDestination
	PathPickerPurposeApplyFlattenDestination  = dialog.PathPickerPurposeApplyFlattenDestination
	PathPickerPurposeApplyFileDialogField     = dialog.PathPickerPurposeApplyFileDialogField

	FileDialogNone         = dialog.FileDialogNone
	FileDialogRename       = dialog.FileDialogRename
	FileDialogCopyHere     = dialog.FileDialogCopyHere
	FileDialogMkdir        = dialog.FileDialogMkdir
	FileDialogDelete       = dialog.FileDialogDelete
	FileDialogChmod        = dialog.FileDialogChmod
	FileDialogChown        = dialog.FileDialogChown
	FileDialogSymlink      = dialog.FileDialogSymlink
	FileDialogHardlink     = dialog.FileDialogHardlink
	FileDialogAddBookmark  = dialog.FileDialogAddBookmark
	FileDialogRunForEach   = dialog.FileDialogRunForEach
	FileDialogMassRename   = dialog.FileDialogMassRename
	FileDialogExtract      = dialog.FileDialogExtract
	FileDialogSFTPConnect  = dialog.FileDialogSFTPConnect
	FileDialogSFTPPassword = dialog.FileDialogSFTPPassword

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
	PrimaryModalFlatten  = dialog.PrimaryModalFlatten
	PrimaryModalConflict = dialog.PrimaryModalConflict
	PrimaryModalQuit     = dialog.PrimaryModalQuit

	TransferKindCopy = dialog.TransferKindCopy
	TransferKindMove = dialog.TransferKindMove

	TransferPhaseDestination    = dialog.TransferPhaseDestination
	TransferPhaseSelfCopyRename = dialog.TransferPhaseSelfCopyRename

	TransferDestSubFocusText   = dialog.TransferDestSubFocusText
	TransferDestSubFocusPicker = dialog.TransferDestSubFocusPicker
	FlattenDestSubFocusText    = dialog.FlattenDestSubFocusText
	FlattenDestSubFocusPicker  = dialog.FlattenDestSubFocusPicker
)

var (
	AltDialogOK                         = dialog.AltDialogOK
	AltDialogCancel                     = dialog.AltDialogCancel
	MkdirActionRadioSpecs               = dialog.MkdirActionRadioSpecs
	MkdirActionForAltShortcut           = dialog.MkdirActionForAltShortcut
	MetaEntryIndexForAltShortcut        = dialog.MetaEntryIndexForAltShortcut
	UserMenuEntryIndexForAltShortcut    = dialog.UserMenuEntryIndexForAltShortcut
	ListOKCancelNavFocusKey             = dialog.ListOKCancelNavFocusKey
	ListClampedSelectionDelta           = dialog.ListClampedSelectionDelta
	EnsurePathPickerListScroll          = dialog.EnsurePathPickerListScroll
	EnsureHistoryListScroll             = dialog.EnsureHistoryListScroll
	EnsureSFTPConnectListScroll         = dialog.EnsureSFTPConnectListScroll
	EnsureFindListScroll                = dialog.EnsureFindListScroll
	CenterFindListScroll                = dialog.CenterFindListScroll
	ComputeHelpDialogListMetrics        = dialog.ComputeHelpDialogListMetrics
	FindDialogNavFocusKey               = dialog.FindDialogNavFocusKey
	ScrollContentLen                    = dialog.ScrollContentLen
	EnsureScrollInputVisible            = dialog.EnsureScrollInputVisible
	AdjustScrollForCompletion           = dialog.AdjustScrollForCompletion
	EnsurePathInputScroll               = dialog.EnsurePathInputScroll
	ShouldPreemptiveScrollRevealOnErase = dialog.ShouldPreemptiveScrollRevealOnErase
	AdjustScrollRevealOnErase           = dialog.AdjustScrollRevealOnErase
	ConfigDialogScrollModeFocus         = dialog.ConfigDialogScrollModeFocus
	ConfigDialogScrollbarFocus          = dialog.ConfigDialogScrollbarFocus
	ConfigDialogScrollModeIndex         = dialog.ConfigDialogScrollModeIndex
	ConfigDialogScrollbarIndex          = dialog.ConfigDialogScrollbarIndex
	ConfigDialogMoveScrollFocus         = dialog.ConfigDialogMoveScrollFocus
	NewDialogLinearForm                 = dialog.NewDialogLinearForm
	NewDialogTrailingButtonsForm        = dialog.NewDialogTrailingButtonsForm
	NewTransferDialogLinearForm         = dialog.NewTransferDialogLinearForm
	TransferDialogNumContent            = dialog.TransferDialogNumContent
	TransferDialogEffectiveNumContent   = dialog.TransferDialogEffectiveNumContent
	TransferDialogMoveFocus             = dialog.TransferDialogMoveFocus
	FlattenDialogMoveFocus              = dialog.FlattenDialogMoveFocus
	NewFlattenDialogLinearForm          = dialog.NewFlattenDialogLinearForm
	DialogPairLeftRight                 = dialog.DialogPairLeftRight
	IsWordRune                          = lineedit.IsWordRune
	BackwardWordIndex                   = lineedit.BackwardWordIndex
	ForwardWordIndex                    = lineedit.ForwardWordIndex
	KillWordBackward                    = lineedit.KillWordBackward
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

// FileDialogFocusForm returns trailing-button focus layout for a file dialog (delegates to dialog).
func FileDialogFocusForm(st FileDialogState) dialog.DialogTrailingButtonsForm {
	return dialog.FileDialogFocusForm(st)
}

// MassRenamePreviewViewportRows returns how many preview lines fit for a terminal height.
func MassRenamePreviewViewportRows(layoutHeight int) int {
	return dialog.MassRenamePreviewViewportRows(layoutHeight)
}

// MassRenameEnsurePreviewScroll clamps MassRenamePreviewScroll to keep the viewport valid.
func MassRenameEnsurePreviewScroll(st *FileDialogState, viewportRows, totalRows int) {
	dialog.MassRenameEnsurePreviewScroll(st, viewportRows, totalRows)
}

// FileDialogMassRenameOKEnabled reports whether mass rename OK may run (no preview conflicts / invalid find).
func FileDialogMassRenameOKEnabled(st FileDialogState) bool {
	return dialog.FileDialogMassRenameOKEnabled(st)
}

// DeleteDialogListViewportRows returns how many delete-list name rows fit for a terminal height.
func DeleteDialogListViewportRows(layoutHeight int, state FileDialogState) int {
	return dialog.DeleteDialogListViewportRows(layoutHeight, state)
}

// DeleteEnsureListScroll clamps DeleteListScroll to keep the viewport valid.
func DeleteEnsureListScroll(st *FileDialogState, viewportRows, totalRows int) {
	dialog.DeleteEnsureListScroll(st, viewportRows, totalRows)
}

// DeleteListEntryName returns the delete confirmation list label for one entry.
func DeleteListEntryName(panelPath, homeDir, entryPath, entryName string) string {
	return dialog.DeleteListEntryName(panelPath, homeDir, entryPath, entryName)
}

// MassRenameDiff returns highlight ranges for mass-rename preview columns.
func MassRenameDiff(old, new string) ([]search.Range, []search.Range) {
	return dialog.MassRenameDiff(old, new)
}

// ApplyRenameSanitize applies dot/underscore-to-space cleanups (delegates to dialog).
func ApplyRenameSanitize(s string, dotsToSpace, underscoresToSpace bool) string {
	return dialog.ApplyRenameSanitize(s, dotsToSpace, underscoresToSpace)
}

// ApplyRenameSlugify replaces ASCII spaces with the chosen separator (delegates to dialog).
func ApplyRenameSlugify(s string, sep RenameSlugifySep) string {
	return dialog.ApplyRenameSlugify(s, dialog.RenameSlugifySep(sep))
}

// FileDialogHasRenamePhase reports dialog types that use RenamePhase (rename, copy-here).
func FileDialogHasRenamePhase(t FileDialogType) bool {
	return dialog.FileDialogHasRenamePhase(t)
}
