package jobs

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Host supplies cross-cutting app services the jobs handler cannot import from internal/app.
type Host interface {
	LayoutForTerminalSize(w, h int) ui.Layout
	SetTransientMessage(text string, urgency ui.MessageUrgency)
	SetUnsupportedMessage(msg string)
	RefreshBothPanels()
	RequestBothPanelsVolumeSpaceRefreshAsync()
	ActivePanel() *panel.State
	ActivePanelSources() []string
	InactivePanel() *panel.State
	LeftPanel() *panel.State
	RightPanel() *panel.State
	OpenTransferDialogSelfCopyRename(kind ui.TransferKind, absDest, source string)
	HandleQuit() bool
	HandleQuitImmediate() bool
	OpenMenu()
	OpenMenuByShortcut(shortcut rune) bool
	Dispatch(actionID string)
	TryDispatchAuxiliaryScreens(actionID string) bool
	ActionFromKeyEvent(ev *tcell.EventKey) string
	SetJobFailedTransientMessage(err error, fallback string)
	DevMode() bool
}
