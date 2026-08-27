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

// PanelNavigationHost navigates a panel to a directory and optionally selects an entry by name.
type PanelNavigationHost interface {
	NavigatePanelToPath(panelID int, path string, selectName string) error
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
	// ToggleLeaderMenu opens (or, if already open, closes) the `:` leader menu scoped to
	// the current auxiliary view's own actions.
	ToggleLeaderMenu()
	// DispatchLeaderLetter fires the auxiliary view's own leader-menu action bound to event's
	// rune directly, without opening the `:` menu, when vi-motion mode is on. Returns false
	// (no-op) when vi-motion mode is off, event isn't a plain letter, or no action in this
	// view's leader menu is bound to it.
	DispatchLeaderLetter(event *tcell.EventKey) bool
}
