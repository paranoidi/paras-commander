// Package dialog owns the file-operation and navigation-dialog family: the generic
// FileDialogField-based open/submit/key-handling machinery; the rename, mkdir, delete,
// duplicate, chmod, chown, symlink, and hardlink dialogs built on it; the mass-rename dialog;
// the archive-extract dialog; the flatten dialog; the bookmarks path picker and add-bookmark
// dialog; the copy/move transfer dialog (including the multi-location preview list and
// self-copy-rename flow); and the path-picker (history/bookmarks fuzzy picker) shared by the
// transfer, flatten, and file-dialog path fields. Dialog STATE (dialog.FileDialogState,
// dialog.TransferDialogState, dialog.PathPickerState, dialog.FlattenDialogState, etc.) lives in
// the shared ui.Model as usual; this package holds the orchestration (opening dialogs,
// dispatching keys, executing the underlying file operation).
//
// The settings/config-edit/message-theme/debounce-calibrate dialogs, the SFTP connect/password
// dialogs, the history dialog, and the quit/stash confirmations remain in internal/app (see
// AGENTS.md's App package layout section for why); this package reaches the few app-side pieces
// it still needs — SFTP password execution, opening the generic message dialog, the quick-filter
// check, and launching the external editor for mass-rename's External mode — through Host.
package dialog

import (
	"github.com/gdamore/tcell/v2"
	commandsctrl "github.com/paranoidi/paras-commander/internal/apphandler/commands"
	dedupctrl "github.com/paranoidi/paras-commander/internal/apphandler/dedup"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/sched"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// PathPickerValidatePayload wakes PollEvent after debounced path-picker filter validation.
type PathPickerValidatePayload struct{}

// TransferDestValidatePayload wakes PollEvent after debounced copy/move/flatten destination
// path validation.
type TransferDestValidatePayload struct{}

// Deps wires the dialog handler at app construction.
type Deps struct {
	Host   Host
	Screen tcell.Screen
	Model  *ui.Model

	// KeysRenameDialog / KeysMkdirDialog / KeysDialogInput are the [dialog.rename] /
	// [dialog.mkdir] / [dialog.input] keymap overlays. May be nil when no overlay chords are
	// configured.
	KeysRenameDialog *keymap.Map
	KeysMkdirDialog  *keymap.Map
	KeysDialogInput  *keymap.Map
	// KeysGlobal is the global keymap, consulted by the path-picker host-shortcut check
	// (bookmark.open opens the fuzzy picker from a focused path field).
	KeysGlobal *keymap.Map
	// KeysTransferDialog is the [dialog.transfer] keymap overlay, consulted by the transfer
	// dialog's active/inactive-panel destination shortcut.
	KeysTransferDialog *keymap.Map
	// KeysFlattenDialog is the [dialog.flatten] keymap overlay, consulted by the flatten
	// dialog's active/inactive-panel destination shortcut.
	KeysFlattenDialog *keymap.Map
	// KeysBookmarkDialog is the [dialog.bookmark] keymap overlay, consulted while the
	// bookmarks path picker is open (delete/open-other fzf-marks entry chords).
	KeysBookmarkDialog *keymap.Map
	// KeysMassRenameDialog is the [dialog.mass_rename] keymap overlay, consulted while the
	// main mass-rename dialog is open (save/load/delete pattern chords).
	KeysMassRenameDialog *keymap.Map

	// ConfigDir is the resolved config directory, used to locate patterns.toml when
	// Config().MassRename.File is empty (mirrors apphandler/meta.Deps.ConfigDir).
	ConfigDir string

	// Jobs enqueues the delete/transfer jobs backing rename-with-copy-select, mkdir
	// copy/move-select, delete, and duplicate.
	Jobs *jobsctrl.Handler
	// Commands runs the run-for-each dialog/batch backend (keymap.ActionFileRunForEach).
	Commands *commandsctrl.Handler
	// Preview is consulted after file-ops that can affect the open quick view / fullscreen
	// preview (refresh, or close when the previewed file itself was deleted).
	Preview *previewctrl.Handler
	// Dedup drives the "N directories left empty" confirmation shown after a dedup-view
	// delete leaves directories dangling (ExecuteDelete's dedup branch).
	Dedup *dedupctrl.Handler

	// DiskUsage / DiskUsageIgnore back the delete confirmation dialog's live size estimate;
	// both are constructed once at startup and never reassigned, so passing them as plain Deps
	// fields (rather than through Host) matches the compare handler's DiskIgnore precedent.
	DiskUsage       *diskusage.Engine
	DiskUsageIgnore diskusage.ShouldIgnoreFolder
}

// duplicateFocusPending defers SelectVisibleEntryCentered until a queued duplicate job creates
// the entry (the job runs asynchronously, so the panel can't select the new name immediately).
type duplicateFocusPending struct {
	panelID int
	listDir string
	name    string
}

// Handler owns the file-operation dialog family: opening dialogs, dispatching their keys, and
// executing the underlying file operation.
type Handler struct {
	host                 Host
	screen               tcell.Screen
	model                *ui.Model
	keysRenameDialog     *keymap.Map
	keysMkdirDialog      *keymap.Map
	keysDialogInput      *keymap.Map
	keysGlobal           *keymap.Map
	keysTransferDialog   *keymap.Map
	keysFlattenDialog    *keymap.Map
	keysBookmarkDialog   *keymap.Map
	keysMassRenameDialog *keymap.Map
	configDir            string
	jobs                 *jobsctrl.Handler
	commands             *commandsctrl.Handler
	preview              *previewctrl.Handler
	dedup                *dedupctrl.Handler
	diskUsage            *diskusage.Engine
	diskUsageIgnore      diskusage.ShouldIgnoreFolder

	// deleteDialogScanFP is the last enqueued directory set fingerprint for the delete
	// confirmation dialog.
	deleteDialogScanFP string
	// deleteDialogSelGen / deleteDialogPanelPath / deleteDialogPrunedPaths skip ResolveSource
	// while the delete dialog is open.
	deleteDialogSelGen      uint64
	deleteDialogPanelPath   string
	deleteDialogPrunedPaths []string

	// duplicateFocus defers SelectVisibleEntryCentered until a queued duplicate job creates the entry.
	duplicateFocus duplicateFocusPending

	// massRenameHistory is the in-memory, session-only (never persisted) list of recently-executed
	// mass-rename patterns, most-recent-first, capped at maxMassRenameHistory. Merged with
	// patterns.toml's saved list when the load-pattern picker opens (see massRenameLoadPickerItems).
	massRenameHistory []ops.MassRenamePattern

	// pathPickerValidate / transferDestValidate debounce the path-picker filter's and the
	// transfer/flatten destination field's "does this path exist" background check; each Arm
	// posts a PathPickerValidatePayload / TransferDestValidatePayload interrupt through Screen
	// so Run() re-renders once the check lands.
	pathPickerValidate   sched.Debouncer
	transferDestValidate sched.Debouncer
}

// New constructs a Handler.
func New(d Deps) *Handler {
	return &Handler{
		host:                 d.Host,
		screen:               d.Screen,
		model:                d.Model,
		keysRenameDialog:     d.KeysRenameDialog,
		keysMkdirDialog:      d.KeysMkdirDialog,
		keysDialogInput:      d.KeysDialogInput,
		keysGlobal:           d.KeysGlobal,
		keysTransferDialog:   d.KeysTransferDialog,
		keysFlattenDialog:    d.KeysFlattenDialog,
		keysBookmarkDialog:   d.KeysBookmarkDialog,
		keysMassRenameDialog: d.KeysMassRenameDialog,
		configDir:            d.ConfigDir,
		jobs:                 d.Jobs,
		commands:             d.Commands,
		preview:              d.Preview,
		dedup:                d.Dedup,
		diskUsage:            d.DiskUsage,
		diskUsageIgnore:      d.DiskUsageIgnore,
	}
}
