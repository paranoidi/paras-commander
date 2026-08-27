package commands

import (
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// AppendEntry appends entry to CommandsList and returns its index. Used both internally
// (run-for-each batches) and by run_executable.go/user_menu.go in internal/app for
// file-execute and user-menu command rows.
func (h *Handler) AppendEntry(entry ui.CommandRunEntry) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	idx := len(h.model.CommandsList)
	h.model.CommandsList = append(h.model.CommandsList, entry)
	return idx
}

// PatchEntry mutates the CommandsList row at idx under lock. No-op when idx is out of range.
func (h *Handler) PatchEntry(idx int, fn func(*ui.CommandRunEntry)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if idx < 0 || idx >= len(h.model.CommandsList) {
		return
	}
	fn(&h.model.CommandsList[idx])
}

func (h *Handler) markCommandsCanceled(fromIdx, count int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	list := h.model.CommandsList
	for j := 0; j < count && fromIdx+j < len(list); j++ {
		e := &list[fromIdx+j]
		if e.Phase == ui.CommandRunDone {
			continue
		}
		e.Phase = ui.CommandRunDone
		e.ExitCode = -1
		if e.ErrorMsg == "" {
			e.ErrorMsg = "Canceled"
		}
	}
}

// Len returns the number of CommandsList rows.
func (h *Handler) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.model.CommandsList)
}

// ToggleView opens the Commands view, or closes it if already active.
func (h *Handler) ToggleView() {
	if h.model.ViewMode == ui.ViewCommands {
		h.CloseView()
		return
	}
	h.OpenView()
}

// OpenView switches to the Commands view.
func (h *Handler) OpenView() {
	h.model.ViewMode = ui.ViewCommands
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.model.MenuDefinitions = menu.CommandsDefinitions(h.keys, h.keysCommands)
	h.model.Menu.ActiveMenu = menu.DefaultIndexCommands()
	if h.model.CommandsView.FocusPane < 0 || h.model.CommandsView.FocusPane > 2 {
		h.model.CommandsView.FocusPane = 0
	}
	h.EnsureSelectionVisible()
}

// OpenViewAt switches to the Commands view with row idx selected, focused, and scrolled into
// view (used when a new command row starts running and should come into focus).
func (h *Handler) OpenViewAt(idx int) {
	h.OpenView()
	h.model.CommandsView.Selected = idx
	h.model.CommandsView.FocusPane = 0
	h.model.CommandsView.ListScroll = 0
	h.model.CommandsView.StdoutScroll = 0
	h.model.CommandsView.StderrScroll = 0
	h.EnsureSelectionVisible()
}

// CloseView switches back to the browser view.
func (h *Handler) CloseView() {
	h.model.ViewMode = ui.ViewBrowser
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.model.MenuDefinitions = h.host.BrowserMenuDefinitions()
	h.model.Menu.ActiveMenu = menu.DefaultIndex()
	h.model.CommandsView = ui.CommandsViewState{}
}

// EnsureSelectionVisible scrolls the Commands-view list so the selected row is on-screen.
func (h *Handler) EnsureSelectionVisible() {
	n := h.Len()
	width, height := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(width, height)
	if layout.TooSmall {
		h.model.CommandsView.EnsureSelectionVisible(n, 0)
		return
	}
	visible := ui.PanelListRows(layout.Primary)
	h.model.CommandsView.EnsureSelectionVisible(n, visible)
}

// TryDispatch handles Commands-view action IDs. Returns true when actionID was consumed.
func (h *Handler) TryDispatch(actionID string) bool {
	switch actionID {
	case keymap.ActionCommandsOpen:
		h.ToggleView()
		return true
	case keymap.ActionCommandsClose:
		if h.model.ViewMode == ui.ViewCommands {
			h.CloseView()
		}
		return true
	case keymap.ActionCommandsTerminate:
		if h.model.ViewMode == ui.ViewCommands {
			h.terminateSelectedCommand()
		}
		return true
	case keymap.ActionCommandsKill:
		if h.model.ViewMode == ui.ViewCommands {
			h.killSelectedCommand()
		}
		return true
	default:
		return false
	}
}

func (h *Handler) selectedRunningCommandRow() (int, bool) {
	idx := h.model.CommandsView.Selected
	h.mu.RLock()
	defer h.mu.RUnlock()
	if idx < 0 || idx >= len(h.model.CommandsList) {
		return 0, false
	}
	if h.model.CommandsList[idx].Phase != ui.CommandRunRunning {
		return 0, false
	}
	return idx, true
}

func (h *Handler) terminateSelectedCommand() {
	idx, ok := h.selectedRunningCommandRow()
	if !ok {
		h.host.SetTransientMessage("Could not terminate command", ui.MessageUrgencyWarn)
		return
	}
	if h.closeSelectedPTYRow(idx) {
		return
	}
	if !h.signalCommandRow(idx, syscall.SIGTERM) {
		h.host.SetTransientMessage("Could not terminate command", ui.MessageUrgencyWarn)
	}
}

func (h *Handler) killSelectedCommand() {
	idx, ok := h.selectedRunningCommandRow()
	if !ok {
		h.host.SetTransientMessage("Could not kill command", ui.MessageUrgencyWarn)
		return
	}
	if h.closeSelectedPTYRow(idx) {
		return
	}
	if !h.signalCommandRow(idx, syscall.SIGKILL) {
		h.host.SetTransientMessage("Could not kill command", ui.MessageUrgencyWarn)
	}
}

// closeSelectedPTYRow closes the live PTY session for row idx, when it is the one currently
// running interactively. Subshell.Close sends SIGHUP then SIGKILL after a short grace, same
// terminate-vs-kill collapsing the embedded shell panel already relies on. Returns false when
// idx isn't a live PTY row, so callers fall back to the ordinary os.Process signal path.
func (h *Handler) closeSelectedPTYRow(idx int) bool {
	sess := h.currentEntryPTY()
	if sess == nil || sess.idx != idx {
		return false
	}
	_ = sess.sub.Close()
	return true
}

// MoveSelection moves the Commands-list cursor by delta rows (clamped), scrolling into view
// and resetting stream scroll when the row actually changes. Shared by raw arrow-key handling
// and help-dialog activation.
func (h *Handler) MoveSelection(delta int) {
	n := h.Len()
	sel := max(0, h.model.CommandsView.Selected+delta)
	if n > 0 && sel > n-1 {
		sel = n - 1
	}
	h.setSelection(sel)
}

// SelectEdge moves the Commands-list cursor to the first (toEnd=false) or last (toEnd=true) row.
func (h *Handler) SelectEdge(toEnd bool) {
	n := h.Len()
	sel := 0
	if toEnd && n > 0 {
		sel = n - 1
	}
	h.setSelection(sel)
}

func (h *Handler) setSelection(sel int) {
	beforeSel := h.model.CommandsView.Selected
	h.model.CommandsView.Selected = sel
	if beforeSel != sel {
		h.model.CommandsView.StdoutScroll = 0
		h.model.CommandsView.StderrScroll = 0
	}
	h.EnsureSelectionVisible()
}

// HandleViewKey handles key events while the Commands view is active. The bool return mirrors
// input.go's quit convention for view key handlers.
func (h *Handler) HandleViewKey(event *tcell.EventKey) bool {
	h.clampFocusPane()
	switch event.Key() {
	case tcell.KeyEsc:
		h.CloseView()
		return false
	case tcell.KeyTab:
		n := h.focusPaneCount()
		h.model.CommandsView.FocusPane = (h.model.CommandsView.FocusPane + 1) % n
		return false
	case tcell.KeyBacktab:
		n := h.focusPaneCount()
		h.model.CommandsView.FocusPane = (h.model.CommandsView.FocusPane + n - 1) % n
		return false
	}

	if h.model.ViMotionMode {
		event = keymap.RemapViMotionKey(event)
	}

	nextAction := h.host.ActionFromKeyEvent(event)
	if nextAction == keymap.ActionAppQuit {
		return h.host.HandleQuit()
	}
	if nextAction == keymap.ActionAppQuitImmediate {
		return h.host.HandleQuitImmediate()
	}
	if nextAction == keymap.ActionAppOpenMenu {
		h.host.OpenMenu()
		return false
	}
	if nextAction == keymap.ActionAppLeaderMenu {
		h.host.ToggleLeaderMenu()
		return false
	}
	// Note: 'k' is Kill's leader letter (ActionCommandsKill), but with vi-motion mode on, the
	// remap above already turned a bare 'k' into KeyUp before this point, so it never reaches
	// DispatchLeaderLetter as a rune — Kill stays reachable via the `:` menu, F9 menu, and its
	// S-F8 chord.
	if h.host.DispatchLeaderLetter(event) {
		return false
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if h.host.OpenMenuByShortcut(event.Rune()) {
			return false
		}
	}
	if nextAction != "" && h.TryDispatch(nextAction) {
		return false
	}
	if nextAction != "" && h.host.TryDispatchAuxiliaryScreens(nextAction) {
		return false
	}
	if nextAction == keymap.ActionPanelExternalBrowser {
		h.host.Dispatch(nextAction)
		return false
	}

	fp := h.model.CommandsView.FocusPane
	switch fp {
	case 0:
		switch event.Key() {
		case tcell.KeyUp:
			h.MoveSelection(-1)
		case tcell.KeyDown:
			h.MoveSelection(1)
		case tcell.KeyPgUp:
			h.MoveSelection(-5)
		case tcell.KeyPgDn:
			h.MoveSelection(5)
		case tcell.KeyHome:
			h.SelectEdge(false)
		case tcell.KeyEnd:
			h.SelectEdge(true)
		}
	case 1:
		h.scrollPane(&h.model.CommandsView.StdoutScroll, h.maxStdoutScroll(), event.Key())
	case 2:
		h.scrollPane(&h.model.CommandsView.StderrScroll, h.maxStderrScroll(), event.Key())
	}
	return false
}

// scrollPane applies an Up/Down/PgUp/PgDn key to a scroll offset clamped to [0, maxScroll],
// collapsing the identical stdout/stderr pane scroll switches in HandleViewKey.
func (h *Handler) scrollPane(scroll *int, maxScroll int, key tcell.Key) {
	switch key {
	case tcell.KeyUp:
		if *scroll > 0 {
			*scroll--
		}
	case tcell.KeyDown:
		if *scroll < maxScroll {
			*scroll++
		}
	case tcell.KeyPgUp:
		*scroll = max(0, *scroll-5)
	case tcell.KeyPgDn:
		*scroll = min(maxScroll, *scroll+5)
	}
}

func (h *Handler) clampFocusPane() {
	maxP := h.focusPaneCount() - 1
	if h.model.CommandsView.FocusPane > maxP {
		h.model.CommandsView.FocusPane = maxP
	}
}

func (h *Handler) focusPaneCount() int {
	width, height := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 2
	}
	_, stderrRect := ui.SplitJobsSecondaryColumnFlexTop(layout.Secondary, 8)
	if stderrRect.Height == 0 {
		return 2
	}
	return 3
}

func (h *Handler) selectedCommandEntry() ui.CommandRunEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.model.CommandsView.Selected >= 0 && h.model.CommandsView.Selected < len(h.model.CommandsList) {
		return h.model.CommandsList[h.model.CommandsView.Selected]
	}
	return ui.CommandRunEntry{}
}

func (h *Handler) maxStdoutScroll() int {
	sel := h.selectedCommandEntry()
	width, height := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	stdoutRect, _, stdoutLines, _ := ui.CommandsStreamPanels(layout.Secondary, sel)
	contentH := ui.JobsPanelContentRows(stdoutRect)
	return max(0, len(stdoutLines)-contentH)
}

func (h *Handler) maxStderrScroll() int {
	sel := h.selectedCommandEntry()
	width, height := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	_, stderrRect, _, stderrLines := ui.CommandsStreamPanels(layout.Secondary, sel)
	contentH := ui.JobsPanelContentRows(stderrRect)
	return max(0, len(stderrLines)-contentH)
}
