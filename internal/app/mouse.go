package app

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// mouseState is the click bookkeeping behind handleMouse.
type mouseState struct {
	// enabled mirrors whether mouse reporting is currently on at the screen.
	enabled bool
	// buttons is the button mask of the previous event; tcell repeats the held buttons on every
	// drag report, so a press is only the bits that were not set before.
	buttons tcell.ButtonMask
	// lastTarget/lastRow/lastAt identify the previous left press for double-click detection.
	lastTarget string
	lastRow    int
	lastAt     time.Time
}

// doubleClick records a left press on row of target and reports whether it completes a double
// click: the previous press hit the same row within config.MouseDoubleClickMS. A third press
// starts a new pair.
func (m *mouseState) doubleClick(target string, row int) bool {
	double := m.lastTarget == target && m.lastRow == row &&
		time.Since(m.lastAt) <= config.MouseDoubleClickMS*time.Millisecond
	m.lastTarget, m.lastRow, m.lastAt = target, row, time.Now()
	if double {
		m.lastAt = time.Time{}
	}
	return double
}

// applyMouseConfig switches terminal mouse reporting to match [ui].mouse.
func (a *App) applyMouseConfig() {
	switch {
	case a.config.UI.Mouse && !a.mouse.enabled:
		a.screen.EnableMouse(tcell.MouseButtonEvents | tcell.MouseDragEvents)
		a.mouse.enabled = true
	case !a.config.UI.Mouse && a.mouse.enabled:
		a.screen.DisableMouse()
		a.mouse.enabled = false
		a.mouse.buttons = 0
	}
}

// releaseMouse turns mouse reporting off before the TTY is handed to another program and
// returns the func that turns it back on. Both are no-ops while mouse support is off.
func (a *App) releaseMouse() (restore func()) {
	if !a.mouse.enabled {
		return func() {}
	}
	a.screen.DisableMouse()
	a.mouse.buttons = 0
	return func() {
		if a.config.UI.Mouse {
			a.screen.EnableMouse(tcell.MouseButtonEvents | tcell.MouseDragEvents)
		} else {
			a.mouse.enabled = false
		}
	}
}

// mouseHit locates a cell in the browser: the panel under it and, when it lies on an entry row,
// the index of that entry (row < 0 otherwise). ok is false outside any clickable file panel.
func (a *App) mouseHit(x, y int) (panelID, row int, ok bool) {
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return 0, -1, false
	}
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		col, p := layout.Primary, &a.model.Primary
		if id == ui.SecondaryPanel {
			col, p = layout.Secondary, &a.model.Secondary
		}
		if x < col.X || x >= col.X+col.Width || y < col.Y || y >= col.Y+col.Height {
			continue
		}
		// The inactive column shows a preview while quick view or file preview is open.
		if id != a.model.ActivePanel && (a.model.QuickViewDisplayActive() || a.model.FilePreviewDraw.Open) {
			return 0, -1, false
		}
		if p.CarouselMode {
			// ponytail: carousel columns have their own row layout; clicks only focus the panel.
			return id, -1, true
		}
		file := ui.FileListFrame(col, p, id, a.model.ActivePanel, a.model.ThemeDialog.Open, a.selectedStripSplitParams(id, 0))
		top := ui.PanelListTop(file)
		if x >= file.X && x < file.X+file.Width && y >= top && y < top+ui.PanelListRows(file) {
			if idx := p.ScrollOffset + y - top; idx < p.VisibleEntryCount() {
				return id, idx, true
			}
		}
		return id, -1, true
	}
	return 0, -1, false
}

// focusPanelByMouse makes id the active panel with the file list focused.
func (a *App) focusPanelByMouse(id int) {
	if a.model.ActivePanel != id {
		a.switchPanel()
	}
	a.model.ActiveSubFocus = ui.SubFocusFileList
}

// setCursorByMouse moves the active panel's cursor to row through the keyboard nav path.
func (a *App) setCursorByMouse(row int) {
	p := a.activePanel()
	vr := a.activeViewportRows()
	a.doListNav(func() { p.Move(row-p.Cursor, vr) })
}

// handleMouse handles one mouse event. The footer, the menu bar and its pulldown take clicks in
// every mode that shows them; the browser panels react in normal mode, the Jobs, Commands and
// Messages lists in their views, and the top-most dlg dialog in every other mode
// (handleDialogMouse). Everything else (other panes and views, the quick filter) swallows it.
func (a *App) handleMouse(ev *tcell.EventMouse) (quit, rendered bool) {
	buttons := ev.Buttons()
	pressed := buttons &^ a.mouse.buttons
	a.mouse.buttons = buttons &^ (tcell.WheelUp | tcell.WheelDown | tcell.WheelLeft | tcell.WheelRight)
	if a.terminalPanelHasKeyFocus() {
		return false, false
	}
	x, y := ev.Position()
	if handled, quit, rendered := a.handleChromeMouse(x, y, buttons, pressed); handled {
		return quit, rendered
	}
	switch a.inputMode() {
	case InputModeJobsView, InputModeCommandsView, InputModeMessagesView:
		return false, a.handleAuxListMouse(x, y, buttons, pressed)
	case InputModeNormal:
		if a.model.ViewMode != ui.ViewBrowser {
			return false, false
		}
	default:
		return a.handleDialogMouse(x, y, buttons, pressed)
	}
	id, row, ok := a.mouseHit(x, y)
	if !ok {
		return false, false
	}
	switch {
	case buttons&(tcell.WheelUp|tcell.WheelDown) != 0:
		step := config.MouseWheelRows
		if buttons&tcell.WheelUp != 0 {
			step = -step
		}
		// The wheel scrolls the panel under the pointer without taking focus.
		if id != a.model.ActivePanel {
			return false, a.scrollInactivePanel(id, step)
		}
		a.setCursorByMouse(a.activePanel().Cursor + step)
		return false, true
	case pressed&tcell.Button1 != 0:
		a.focusPanelByMouse(id)
		if row < 0 {
			return false, true
		}
		a.setCursorByMouse(row)
		if !a.mouse.doubleClick(fmt.Sprintf("panel:%d", id), row) {
			return false, true
		}
		return a.finishResolvedKeyboardAction(keymap.ActionNavOpen)
	case pressed&tcell.Button2 != 0: // tcell: Button2 is the right button
		a.focusPanelByMouse(id)
		if row < 0 {
			return false, true
		}
		a.setCursorByMouse(row)
		return a.finishResolvedKeyboardAction(keymap.ActionPanelSelectToggle)
	}
	return false, false
}

// scrollInactivePanel moves the cursor of a non-focused panel by delta rows.
func (a *App) scrollInactivePanel(id, delta int) bool {
	p := a.panelByID(id)
	vr := a.panelViewportRows(id)
	p.Move(delta, vr)
	return true
}

// handleChromeMouse handles the footer, the menu bar and the open pulldown. handled is false when
// the event belongs to whatever lies below them.
func (a *App) handleChromeMouse(x, y int, buttons, pressed tcell.ButtonMask) (handled, quit, rendered bool) {
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return false, false, false
	}
	left := pressed&tcell.Button1 != 0
	if fk, ok := ui.FooterKeyAt(layout, &a.model, x, y); ok {
		if !left {
			return true, false, false
		}
		ev, ok := fk.Event()
		if !ok {
			return true, false, false
		}
		quit, rendered := a.handleKey(ev)
		return true, quit, rendered
	}
	if index, ok := ui.MenuBarMenuAt(layout, &a.model, x, y); ok && a.menuBarAcceptsMouse() {
		if !left {
			return true, false, false
		}
		if a.model.Menu.Open && a.model.Menu.PulldownOpen && a.model.Menu.ActiveMenu == index {
			a.closeMenu()
		} else {
			a.openMenuAt(index)
		}
		a.render()
		return true, false, true
	}
	switch a.inputMode() {
	case InputModeLeaderMenu:
		return a.handleLeaderMenuMouse(layout, x, y, pressed)
	case InputModeMenu:
	default:
		return false, false, false
	}
	// The menu is modal: wheel and clicks act on the pulldown, a click anywhere else closes it.
	item, inside := ui.PulldownItemAt(layout, &a.model, x, y)
	switch {
	case buttons&(tcell.WheelUp|tcell.WheelDown) != 0:
		if !inside {
			return true, false, false
		}
		delta := 1
		if buttons&tcell.WheelUp != 0 {
			delta = -1
		}
		a.moveMenuItem(delta)
	case left && inside:
		if item < 0 {
			return true, false, false
		}
		a.model.Menu.SelectedItem = item
		quit := a.activateMenuItem()
		if quit {
			return true, true, false
		}
	case pressed != 0 && !inside:
		a.closeMenu()
	default:
		return true, false, false
	}
	a.render()
	return true, false, true
}

// menuBarAcceptsMouse reports whether a click on a menu label may open that menu: in the views
// whose menu bar F9 opens, but not while a dialog or confirmation has the keyboard.
func (a *App) menuBarAcceptsMouse() bool {
	switch a.inputMode() {
	case InputModeNormal, InputModeMenu, InputModeFilter, InputModeJobsView, InputModeCommandsView, InputModeMessagesView, InputModeCompareView, InputModeDedupView:
		return true
	}
	return false
}

// handleLeaderMenuMouse handles the open leader menu strip, which is modal like the pulldown: a
// click on an entry runs it, a click elsewhere closes the menu as Esc does. The wheel is ignored.
func (a *App) handleLeaderMenuMouse(layout ui.Layout, x, y int, pressed tcell.ButtonMask) (handled, quit, rendered bool) {
	if pressed&(tcell.Button1|tcell.Button2|tcell.Button3) == 0 {
		return true, false, false
	}
	action, inside := ui.LeaderMenuActionAt(layout, a.model.LeaderMenu, x, y)
	switch {
	case !inside:
		if a.model.LeaderMenu.UserMenu {
			a.closeUserMenu()
		} else {
			a.closeLeaderMenu()
		}
	case action >= 0 && pressed&tcell.Button1 != 0:
		if a.activateLeaderMenu(action) {
			return true, true, false
		}
	default:
		return true, false, false
	}
	a.render()
	return true, false, true
}

// handleAuxListMouse handles the list pane of the Jobs, Commands and Messages views: a left click
// focuses the list and selects the entry under the pointer, the wheel moves the selection.
func (a *App) handleAuxListMouse(x, y int, buttons, pressed tcell.ButtonMask) (rendered bool) {
	var hit func(ui.Layout, *ui.Model, int, int) (int, bool)
	var selectEntry, move func(int)
	switch a.inputMode() {
	case InputModeJobsView:
		hit, selectEntry, move = ui.JobsListEntryAt, a.jobsCtrl.FocusListRow, a.jobsCtrl.MoveSelection
	case InputModeCommandsView:
		hit, selectEntry, move = ui.CommandsListEntryAt, a.commandsCtrl.FocusListRow, a.commandsCtrl.MoveSelection
	case InputModeMessagesView:
		hit, selectEntry, move = ui.MessagesListEntryAt, a.selectMessage, a.moveMessagesSelection
	default:
		return false
	}
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return false
	}
	idx, ok := hit(layout, &a.model, x, y)
	if !ok {
		return false
	}
	switch {
	case buttons&(tcell.WheelUp|tcell.WheelDown) != 0:
		step := config.MouseWheelRows
		if buttons&tcell.WheelUp != 0 {
			step = -step
		}
		move(step)
	case pressed&tcell.Button1 != 0:
		selectEntry(idx)
	default:
		return false
	}
	a.render()
	return true
}
