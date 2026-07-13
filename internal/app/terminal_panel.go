package app

import (
	"errors"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/subshell"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// terminalWakePayload wakes the event loop after a coalesced PTY output burst.
type terminalWakePayload struct{}

// terminalPanelDrawer adapts subshell.PanelFeed to ui.TerminalDrawer.
type terminalPanelDrawer struct {
	feed  *subshell.PanelFeed
	style tcell.Style
}

func (d *terminalPanelDrawer) DrawTo(setCell func(x, y int, r rune, style tcell.Style)) (int, int, bool) {
	return d.feed.Draw(d.style, setCell)
}

// terminalLayoutRows returns the rows the layout reserves for the terminal panel.
// Full-screen views (jobs, compare, …) reclaim the strip; the feed keeps running.
func (a *App) terminalLayoutRows() int {
	if a.model.TerminalPanel.Visible && a.model.ViewMode == ui.ViewBrowser {
		return a.model.TerminalPanel.Rows
	}
	return 0
}

// terminalPanelContentDims returns the panel's content grid from the current layout.
// ok is false when the layout omits the strip.
func (a *App) terminalPanelContentDims() (cols, rows int, ok bool) {
	w, h := a.screen.Size()
	lay := a.layoutForTerminalSize(w, h)
	if lay.Terminal.Height <= 0 || lay.Terminal.Width <= 0 {
		return 0, 0, false
	}
	return lay.Terminal.Width, lay.Terminal.Height, true
}

// toggleTerminalPanelVisible shows/hides the panel without changing focus:
// hidden → shown (unfocused); visible (focused or not) → hidden. Hiding while
// focused syncs the active panel to the shell cwd first, same as unfocusing.
func (a *App) toggleTerminalPanelVisible() {
	if a.model.ModalDialogOpen() || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	tp := &a.model.TerminalPanel
	if !tp.Visible {
		a.openTerminalPanel(false)
	} else {
		if tp.Focused {
			a.syncPanelFromSubshellCwd()
		}
		a.closeTerminalPanel()
	}
	a.render()
}

// toggleTerminalPanelFocus toggles keyboard focus into/out of the panel:
// hidden → open+focused; visible+unfocused → focused; focused → unfocused
// (syncing the active panel to the shell cwd, mirroring drop-to-shell return).
func (a *App) toggleTerminalPanelFocus() {
	if a.model.ModalDialogOpen() || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	tp := &a.model.TerminalPanel
	switch {
	case !tp.Visible:
		a.openTerminalPanel(true)
	case tp.Focused:
		tp.Focused = false
		a.syncPanelFromSubshellCwd()
	default:
		tp.Focused = true
	}
	a.render()
}

func (a *App) openTerminalPanel(focus bool) {
	if !a.config.Shell.Persistent || strings.TrimSpace(a.config.Shell.Command) != "" {
		a.setTransientMessage("Terminal panel requires the persistent shell", ui.MessageUrgencyWarn)
		return
	}
	if a.activePanel().Path.IsRemote() {
		a.setTransientMessage("Terminal panel is not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	panelDir := a.localActivePanelDir()
	fresh, ok := a.ensureSubshell(panelDir)
	if !ok {
		return
	}

	tp := &a.model.TerminalPanel
	if tp.Rows < config.MinShellTerminalPanelHeight {
		tp.Rows = a.config.Shell.TerminalPanelHeight
	}
	tp.Visible = true
	tp.Focused = focus
	cols, rows, dimsOK := a.terminalPanelContentDims()
	if !dimsOK {
		tp.Visible, tp.Focused = false, false
		a.setTransientMessage("Terminal panel: not enough screen space", ui.MessageUrgencyWarn)
		return
	}

	if !fresh && panelDir != "" {
		if errors.Is(a.subshellChdirIfNeeded(panelDir), subshell.ErrBusy) {
			a.setTransientMessage("Shell is busy — panel directory was not sent to the shell", ui.MessageUrgencyWarn)
		}
	}
	if a.terminalFeed != nil {
		// Feed already alive from a previous hide — reuse it so the emulator
		// state (shell prompt, command output) is instantly visible.
		a.terminalFeed.Resize(cols, rows)
		tp.Drawer = &terminalPanelDrawer{feed: a.terminalFeed, style: a.styles.TerminalTextStyle()}
		return
	}
	feed, err := a.subshell.StartPanelFeed(cols, rows, a.postTerminalWake)
	if err != nil {
		tp.Visible, tp.Focused = false, false
		a.setErrorMessage("Terminal", err)
		return
	}
	a.terminalFeed = feed
	tp.Drawer = &terminalPanelDrawer{feed: feed, style: a.styles.TerminalTextStyle()}
}

// closeTerminalPanel hides the panel but keeps the feed alive so the emulator
// state persists across hide/show cycles. The shell session stays alive.
// closeSubshell calls this first, then closes the subshell (which kills the feed).
func (a *App) closeTerminalPanel() {
	tp := &a.model.TerminalPanel
	tp.Visible, tp.Focused = false, false
	tp.Drawer = nil
}

// syncPanelFromSubshellCwd mirrors [shell].sync_cwd_on_return when focus leaves the shell.
func (a *App) syncPanelFromSubshellCwd() {
	if !a.config.Shell.SyncCwdOnReturn || a.subshell == nil || a.activePanel().Path.IsRemote() {
		return
	}
	if cwd, err := a.subshell.Cwd(); err == nil {
		a.syncActivePanelToDir(cwd)
	}
}

// postTerminalWake runs on the PTY reader goroutine; coalesced via terminalWakePending.
func (a *App) postTerminalWake() {
	if a.terminalWakePending.CompareAndSwap(false, true) {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(terminalWakePayload{}))
	}
}

func (a *App) handleTerminalWake() {
	a.terminalWakePending.Store(false)
	feed := a.terminalFeed
	if feed == nil {
		return
	}
	if feed.Exited() {
		a.closeSubshell()
		a.setTransientMessage("Terminal: shell exited", ui.MessageUrgencyInfo)
		return
	}
	if !a.model.TerminalPanel.Visible {
		// Feed is alive (emulator accumulates output) but the panel is hidden;
		// skip the render until the user re-opens it.
		return
	}
	// ponytail: full render per coalesced output burst; switch to a terminal-rect-only
	// partial paint if profiling ever shows this hot.
	a.render()
}

func (a *App) growTerminalPanel()   { a.resizeTerminalPanel(1) }
func (a *App) shrinkTerminalPanel() { a.resizeTerminalPanel(-1) }

func (a *App) resizeTerminalPanel(delta int) {
	tp := &a.model.TerminalPanel
	if !tp.Visible {
		return
	}
	requested := tp.Rows + delta
	if requested < config.MinShellTerminalPanelHeight {
		return
	}
	previous := tp.Rows
	tp.Rows = requested
	cols, rows, ok := a.terminalPanelContentDims()
	if !ok || rows != requested {
		// Layout clamped or refused: keep the model honest about what is on screen.
		tp.Rows = previous
		cols, rows, ok = a.terminalPanelContentDims()
		if !ok {
			return
		}
	}
	if a.terminalFeed != nil {
		a.terminalFeed.Resize(cols, rows)
	}
	a.render()
}

// resizeTerminalFeedToLayout re-syncs the PTY/emulator grid after a screen resize.
// When the layout omits the strip (screen too small) the panel stays Visible and
// returns automatically once the screen grows; focus falls back to the files.
func (a *App) resizeTerminalFeedToLayout() {
	if a.terminalFeed == nil || !a.model.TerminalPanel.Visible {
		return
	}
	if cols, rows, ok := a.terminalPanelContentDims(); ok {
		a.terminalFeed.Resize(cols, rows)
	} else {
		a.model.TerminalPanel.Focused = false
	}
}

// terminalPanelHasKeyFocus reports whether keystrokes belong to the embedded shell.
// Any modal surface (dialogs, menu, quick filter) wins over the panel.
func (a *App) terminalPanelHasKeyFocus() bool {
	tp := a.model.TerminalPanel
	if !tp.Visible || !tp.Focused || a.model.ViewMode != ui.ViewBrowser || a.terminalFeed == nil {
		return false
	}
	if a.model.ModalDialogOpen() || a.model.Menu.Open || a.inQuickFilterUI() {
		return false
	}
	_, _, ok := a.terminalPanelContentDims()
	return ok
}

// handleTerminalPanelKey routes a key while the panel is focused: the [terminal]
// overlay chords first, everything else encoded to PTY bytes (F10 reaches htop).
func (a *App) handleTerminalPanelKey(event *tcell.EventKey) (rendered bool) {
	if id, ok := a.keysTerminal.Lookup(event); ok {
		switch id {
		case keymap.ActionTerminalTogglePanel:
			a.toggleTerminalPanelVisible()
			return true
		case keymap.ActionTerminalFocus:
			a.toggleTerminalPanelFocus()
			return true
		case keymap.ActionTerminalGrow:
			a.growTerminalPanel()
			return true
		case keymap.ActionTerminalShrink:
			a.shrinkTerminalPanel()
			return true
		case keymap.ActionAppDropToShell:
			a.dropToShell()
			return true
		}
	}
	if b := subshell.EncodeKey(event, a.terminalFeed.AppCursor()); len(b) > 0 {
		if _, err := a.subshell.WritePTY(b); err != nil {
			a.setErrorMessage("Terminal", err)
			a.render()
			return true
		}
	}
	return false
}

// hwCursorState mirrors what ShowCursor/HideCursor asked of the terminal; tcell exposes
// no getter, so the app tracks it for the render hash cache (see emitScreenAfterFullRender).
type hwCursorState struct {
	x, y    int
	visible bool
}

// syncTerminalPanelCursor owns the hardware cursor: shown at the emulator cursor while
// the panel is focused, hidden otherwise (nothing else in the app shows it).
func (a *App) syncTerminalPanelCursor() {
	if a.terminalPanelHasKeyFocus() {
		w, h := a.screen.Size()
		lay := a.layoutForTerminalSize(w, h)
		cx, cy, visible := a.terminalFeed.Cursor()
		if visible && lay.Terminal.Height > 0 && cx >= 0 && cy >= 0 &&
			cx < lay.Terminal.Width && cy < lay.Terminal.Height {
			a.screen.ShowCursor(lay.Terminal.X+cx, lay.Terminal.Y+cy)
			a.pendingCursor = hwCursorState{x: lay.Terminal.X + cx, y: lay.Terminal.Y + cy, visible: true}
			return
		}
	}
	a.screen.HideCursor()
	a.pendingCursor = hwCursorState{}
}
