// Package dialog owns the file-operation dialog family: the generic FileDialogField-based
// open/submit/key-handling machinery, and the rename, mkdir, delete, duplicate, chmod, chown,
// symlink, and hardlink dialogs built on it. Dialog STATE (dialog.FileDialogState) lives in the
// shared ui.Model as usual; this package holds the orchestration (opening dialogs, dispatching
// keys, executing the underlying file operation) that used to live in internal/app's dialog_*.go
// files.
//
// Mass-rename execution, the extract/flatten dialogs, the copy/move transfer dialog, the
// path-picker, and bookmarks/SFTP dialogs are not part of this package yet: they remain in
// internal/app and are reached through Host (or, for run-for-each, through Deps.Commands)
// until a later extraction step folds them in here too.
package dialog

import (
	"github.com/gdamore/tcell/v2"
	commandsctrl "github.com/paranoidi/paras-commander/internal/apphandler/commands"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

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

	// Jobs enqueues the delete/transfer jobs backing rename-with-copy-select, mkdir
	// copy/move-select, delete, and duplicate.
	Jobs *jobsctrl.Handler
	// Commands runs the run-for-each dialog/batch backend (keymap.ActionFileRunForEach).
	Commands *commandsctrl.Handler
	// Preview is consulted after file-ops that can affect the open quick view / fullscreen
	// preview (refresh, or close when the previewed file itself was deleted).
	Preview *previewctrl.Handler

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
	host             Host
	screen           tcell.Screen
	model            *ui.Model
	keysRenameDialog *keymap.Map
	keysMkdirDialog  *keymap.Map
	keysDialogInput  *keymap.Map
	jobs             *jobsctrl.Handler
	commands         *commandsctrl.Handler
	preview          *previewctrl.Handler
	diskUsage        *diskusage.Engine
	diskUsageIgnore  diskusage.ShouldIgnoreFolder

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
}

// New constructs a Handler.
func New(d Deps) *Handler {
	return &Handler{
		host:             d.Host,
		screen:           d.Screen,
		model:            d.Model,
		keysRenameDialog: d.KeysRenameDialog,
		keysMkdirDialog:  d.KeysMkdirDialog,
		keysDialogInput:  d.KeysDialogInput,
		jobs:             d.Jobs,
		commands:         d.Commands,
		preview:          d.Preview,
		diskUsage:        d.DiskUsage,
		diskUsageIgnore:  d.DiskUsageIgnore,
	}
}
