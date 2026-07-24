package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Host supplies cross-cutting app services the dialog handler cannot import from internal/app.
type Host interface {
	host.LayoutHost
	host.MessageHost
	host.PanelNavigationHost

	ActivePanel() *panel.State
	InactivePanel() *panel.State
	InactivePanelID() int
	PanelByID(panelID int) *panel.State
	ActiveViewportRows() int
	PanelViewportRows(panelID int) int

	// Config and Styles return the App's live config/theme (not a snapshot): both are mutable
	// at runtime from the settings and theme dialogs, so the handler must re-read them on every
	// use rather than caching a copy from construction time.
	Config() config.Config
	Styles() theme.Theme

	// Path picker / path-completion glue: still owned by internal/app (reserved for a later
	// path-picker extraction), reached through Host until then.
	OpenPathPickerForFileField(fieldIndex int)
	SyncPathFieldCompletion(f *dialog.FileDialogField, textWidth int)
	TransferDestinationTextWidth() int
	TryPathPickerHostShortcut(ev *tcell.EventKey) bool

	// Extract, flatten, copy/move activation, and bookmark/SFTP dialogs: still owned by
	// internal/app (reserved for a later extraction step), reached through Host until then.
	OpenExtractDialog(p *panel.State)
	OpenFlattenDialog()
	ActivateCopyAction()
	ActivateMoveAction()
	ExecuteExtract()
	ExecuteAddBookmark()
	ExecuteSFTPPassword()

	// Mass-rename dialog: opening and the key-handling machinery live in this package, but the
	// dialog's own preview/execution logic is still owned by internal/app (reserved for a later
	// extraction step).
	OpenMassRenameDialog(p *panel.State)
	ApplyMassRenameModeFromFocus()
	MassRenameSyncFieldLabels()
	RecomputeMassRenamePreview()
	MassRenameClampFocusAfterModeChange(prev dialog.MassRenameModeUI)
	LaunchMassRenameExternalEditor()
	ExecuteMassRename()

	// OpenDedupEmptyDirsConfirm opens the dedup-view "N directories left empty" confirmation
	// (internal/app dedup_view.go) when Delete is pressed while the dedup view's own delete
	// dialog is overlaid on it.
	OpenDedupEmptyDirsConfirm()
}
