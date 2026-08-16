package commands

import (
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// Host supplies cross-cutting app services the commands handler cannot import from internal/app.
type Host interface {
	host.LayoutHost
	host.MessageHost
	host.ShellHost

	ActivePanel() *panel.State
	InactivePanel() *panel.State

	// Styles returns the current theme snapshot, read fresh on every use (the settings/theme
	// dialogs mutate it in place after construction) — needed for the run-for-each PTY
	// session's terminal text style in the bottom terminal panel.
	Styles() theme.Theme

	// BrowserMenuDefinitions returns the browser view's menu bar definitions, restored when
	// the Commands view closes.
	BrowserMenuDefinitions() []menu.Definition

	// SetTransientMessageBanner sets both the Messages log entry (log) and the status banner
	// (banner) text at the given urgency.
	SetTransientMessageBanner(log, banner string, urgency ui.MessageUrgency)
	ClearTransientMessage()

	// CloseFileDialog closes the active file dialog (used after Run for each is submitted).
	CloseFileDialog()
	// FocusedFileDialogField returns the currently focused field of the active file dialog,
	// or nil when none is focused (e.g. a rename sub-phase).
	FocusedFileDialogField() *dialog.FileDialogField

	// RefreshAfterUserMenuCommand refreshes the active browser panel after a background
	// user-menu/run-for-each command finishes (no-op outside the browser view).
	RefreshAfterUserMenuCommand()
}
