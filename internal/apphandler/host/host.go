package host

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// LayoutHost supplies terminal layout for modal and full-screen views.
type LayoutHost interface {
	LayoutForTerminalSize(w, h int) ui.Layout
}

// MessageHost supplies transient and modal error messaging.
type MessageHost interface {
	SetTransientMessage(text string, urgency ui.MessageUrgency)
	SetErrorMessage(title string, err error)
}

// PanelHost supplies twin-panel accessors used by the jobs handler.
type PanelHost interface {
	ActivePanel() *panel.State
	ActivePanelSources() []string
	InactivePanel() *panel.State
	PrimaryPanel() *panel.State
	SecondaryPanel() *panel.State
}

// ShellHost supplies quit, menu, and global action dispatch from auxiliary views.
type ShellHost interface {
	HandleQuit() bool
	HandleQuitImmediate() bool
	OpenMenu()
	OpenMenuByShortcut(shortcut rune) bool
	Dispatch(actionID string)
	TryDispatchAuxiliaryScreens(actionID string) bool
	ActionFromKeyEvent(ev *tcell.EventKey) string
}
