package app

import (
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
	// lastPanel/lastRow/lastAt identify the previous left press for double-click detection.
	lastPanel, lastRow int
	lastAt             time.Time
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

// handleMouse handles one mouse event. Only the browser view reacts; everything else (other
// views, dialogs, menus, the quick filter) swallows mouse input.
func (a *App) handleMouse(ev *tcell.EventMouse) (quit, rendered bool) {
	buttons := ev.Buttons()
	pressed := buttons &^ a.mouse.buttons
	a.mouse.buttons = buttons &^ (tcell.WheelUp | tcell.WheelDown | tcell.WheelLeft | tcell.WheelRight)
	if a.model.ViewMode != ui.ViewBrowser || a.inputMode() != InputModeNormal || a.terminalPanelHasKeyFocus() {
		return false, false
	}
	x, y := ev.Position()
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
		double := a.mouse.lastPanel == id && a.mouse.lastRow == row &&
			time.Since(a.mouse.lastAt) <= config.MouseDoubleClickMS*time.Millisecond
		a.mouse.lastPanel, a.mouse.lastRow, a.mouse.lastAt = id, row, time.Now()
		if !double {
			return false, true
		}
		a.mouse.lastAt = time.Time{} // a third press starts a new pair
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
