package app

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// panelDrag is a drag over a file list: the left button selects the rows it covers, the right
// button unselects them. A press on a row arms it; it becomes active when the pointer reaches
// another screen row, and only then does it set the panel's pending range.
type panelDrag struct {
	armed, active bool
	button        tcell.ButtonMask
	panel         int
	lastY         int    // screen row of the last report, clamped to the list
	anchor        string // path of the pressed entry
}

// mouseState is the click bookkeeping behind handleMouse.
type mouseState struct {
	// enabled mirrors whether mouse reporting is currently on at the screen.
	enabled bool
	// buttons is the button mask of the previous event; a release report carries the buttons still
	// held, so a press is only the bits that were not set before.
	buttons tcell.ButtonMask
	// lastTarget/lastRow/lastAt identify the previous left press for double-click detection.
	lastTarget string
	lastRow    int
	lastAt     time.Time
	// pointer/x/y are the last reported pointer cell, for the dialog button hover (pointer is
	// false until a report arrives and while reporting is off).
	pointer bool
	x, y    int
	// keyed hides every hover highlight from a key press until the next mouse event.
	keyed bool
	// drag is the file list drag in progress.
	drag panelDrag
}

// hovers reports whether pointer hover highlights show: the pointer has reported, no key was
// pressed since and no file list drag is active. Every hover goes through it.
func (m *mouseState) hovers() bool { return m.pointer && !m.keyed && !m.drag.active }

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

// mouseEventMask is the reporting mode while mouse support is on: buttons plus all motion, for
// the title path hover highlight.
const mouseEventMask = tcell.MouseButtonEvents | tcell.MouseMotionEvents

// applyMouseConfig switches terminal mouse reporting to match [ui].mouse.
func (a *App) applyMouseConfig() {
	switch {
	case a.config.UI.Mouse && !a.mouse.enabled:
		a.screen.EnableMouse(mouseEventMask)
		a.mouse.enabled = true
	case !a.config.UI.Mouse && a.mouse.enabled:
		a.cancelPanelDrag()
		a.screen.DisableMouse()
		a.mouse.enabled = false
		a.mouse.buttons = 0
		a.mouse.pointer = false
		a.clearPanelTitleHover()
	}
}

// releaseMouse turns mouse reporting off before the TTY is handed to another program and
// returns the func that turns it back on. Both are no-ops while mouse support is off.
func (a *App) releaseMouse() (restore func()) {
	if !a.mouse.enabled {
		return func() {}
	}
	a.cancelPanelDrag()
	a.screen.DisableMouse()
	a.mouse.buttons = 0
	a.mouse.pointer = false
	a.clearPanelTitleHover()
	return func() {
		if a.config.UI.Mouse {
			a.screen.EnableMouse(mouseEventMask)
		} else {
			a.mouse.enabled = false
		}
	}
}

// wheelDelta is the signed row step of a wheel report: rows down, -rows up, 0 for no wheel.
func wheelDelta(buttons tcell.ButtonMask, rows int) int {
	switch {
	case buttons&tcell.WheelUp != 0:
		return -rows
	case buttons&tcell.WheelDown != 0:
		return rows
	}
	return 0
}

// mousePanelAt returns the browser panel whose column holds (x, y) and its file list frame. ok is
// false outside both columns and on the inactive column while quick view or file preview paints
// a preview there.
func (a *App) mousePanelAt(layout ui.Layout, x, y int) (panelID int, file ui.Rect, ok bool) {
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		col := layout.Primary
		if id == ui.SecondaryPanel {
			col = layout.Secondary
		}
		if !col.Contains(x, y) {
			continue
		}
		file, ok := a.panelFileFrame(layout, id)
		return id, file, ok
	}
	return 0, ui.Rect{}, false
}

// panelFileFrame returns the file list frame panelID draws in layout; ok is false when its column
// is off screen or shows the preview instead.
func (a *App) panelFileFrame(layout ui.Layout, panelID int) (ui.Rect, bool) {
	col, p := layout.Primary, &a.model.Primary
	if panelID == ui.SecondaryPanel {
		col, p = layout.Secondary, &a.model.Secondary
	}
	if col.Width <= 0 {
		return ui.Rect{}, false
	}
	if panelID != a.model.ActivePanel && (a.model.QuickViewDisplayActive() || a.model.FilePreviewDraw.Open) {
		return ui.Rect{}, false
	}
	return ui.FileListFrame(col, p, panelID, a.model.ActivePanel, a.selectedStripSplitParams(panelID, 0)), true
}

// mouseHit locates a cell in the browser: the panel under it and, when it lies on an entry row,
// the index of that entry (row < 0 otherwise). ok is false outside any clickable file panel.
func (a *App) mouseHit(layout ui.Layout, x, y int) (panelID, row int, ok bool) {
	id, file, ok := a.mousePanelAt(layout, x, y)
	if !ok {
		return 0, -1, false
	}
	p := a.panelByID(id)
	if p.CarouselMode {
		// ponytail: carousel columns have their own row layout; clicks only focus the panel.
		return id, -1, true
	}
	top := ui.PanelListTop(file)
	if (ui.Rect{X: file.X, Y: top, Width: file.Width, Height: ui.PanelListRows(file)}).Contains(x, y) {
		if idx := p.ScrollOffset + y - top; idx < p.VisibleEntryCount() {
			return id, idx, true
		}
	}
	return id, -1, true
}

// panelTitleHit returns the panel and the title path segment under (x, y) (ui.PanelTitleSegmentAt).
func (a *App) panelTitleHit(layout ui.Layout, x, y int) (panelID int, seg ui.PanelTitleSegment, ok bool) {
	id, file, ok := a.mousePanelAt(layout, x, y)
	if !ok {
		return 0, ui.PanelTitleSegment{}, false
	}
	seg, ok = ui.PanelTitleSegmentAt(file, &a.model, id, a.styles, x, y)
	return id, seg, ok
}

// updatePanelTitleHover tracks the pointer for the title path highlight and reports whether the
// highlighted segment changed (the caller then renders). Motion that keeps the same segment, or
// none, under the pointer changes nothing visible and does not render.
func (a *App) updatePanelTitleHover(layout ui.Layout, x, y int) bool {
	prev := a.model.PanelTitleHover
	var next ui.PanelTitleHover
	var nextSeg ui.PanelTitleSegment
	if a.inputMode() == InputModeNormal && a.model.ViewMode == ui.ViewBrowser {
		if id, seg, ok := a.panelTitleHit(layout, x, y); ok {
			next, nextSeg = ui.PanelTitleHover{Active: true, Panel: id, X: x}, seg
		}
	}
	a.model.PanelTitleHover = next
	if !prev.Active || !next.Active {
		return prev.Active != next.Active
	}
	return prev.Panel != next.Panel || !nextSeg.Contains(prev.X)
}

// mouseGoesToDialog reports whether presses below the chrome belong to the top-most dlg dialog
// (handleDialogMouse): every mode but the browser and the Jobs, Commands, Messages, Compare and
// Duplicates lists.
func (a *App) mouseGoesToDialog() bool {
	switch a.inputMode() {
	case InputModeJobsView, InputModeCommandsView, InputModeMessagesView:
		return a.model.JobsResultFilterDialog.Open
	case InputModeCompareView:
		return a.model.CompareFilterDialog.Open || a.model.CompareMergeDialog.Open
	case InputModeNormal, InputModeDedupView, InputModeUniqueView:
		return false
	}
	return true
}

// dialogHovers are the top-most dlg dialog's button and list row under the last pointer position.
// render sets Model.DialogButtonHover and Model.DialogListHover from them, so the highlights follow
// the dialog as it changes.
func (a *App) dialogHovers() (dlg.ButtonHover, dlg.Rect) {
	if !a.mouse.hovers() || !a.mouseGoesToDialog() {
		return dlg.ButtonHover{}, dlg.Rect{}
	}
	d, ok := a.activeDialog(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !ok {
		return dlg.ButtonHover{}, dlg.Rect{}
	}
	spec, fit := d.spec()
	if !fit {
		return dlg.ButtonHover{}, dlg.Rect{}
	}
	w, h := a.screen.Size()
	row, _ := dlg.HoveredListRow(spec, w, h, *d.focus, a.mouse.x, a.mouse.y)
	return dlg.HoveredButton(spec, *d.focus, w, h, a.mouse.x, a.mouse.y), row
}

// menuBarHover is the menu title under the last pointer position, where a click would open it.
func (a *App) menuBarHover() ui.MenuHover {
	if !a.mouse.hovers() || !a.menuBarAcceptsMouse() {
		return ui.MenuHover{}
	}
	w, h := a.screen.Size()
	index, ok := ui.MenuBarMenuAt(a.layoutForTerminalSize(w, h), &a.model, a.mouse.x, a.mouse.y)
	return ui.MenuHover{Active: ok, Index: index}
}

// selectPulldownItemUnderPointer moves the open pulldown's selection onto the item under (x, y)
// (not the frame or a separator) and reports whether it moved.
func (a *App) selectPulldownItemUnderPointer(layout ui.Layout, x, y int) bool {
	if a.inputMode() != InputModeMenu {
		return false
	}
	item, _ := ui.PulldownItemAt(layout, &a.model, x, y)
	if item < 0 || item == a.model.Menu.SelectedItem {
		return false
	}
	a.model.Menu.SelectedItem = item
	return true
}

// footerHover is the footer entry under the last pointer position, where a click would press it.
func (a *App) footerHover() ui.FooterHover {
	if !a.mouse.hovers() {
		return ui.FooterHover{}
	}
	w, h := a.screen.Size()
	return ui.FooterHoverAt(a.layoutForTerminalSize(w, h), &a.model, a.mouse.x, a.mouse.y)
}

// auxListHit is the row hit-test of the open Jobs, Commands or Messages list (ok false in
// other modes).
func (a *App) auxListHit() (hit func(ui.Layout, *ui.Model, int, int) (int, bool), ok bool) {
	switch a.inputMode() {
	case InputModeJobsView:
		return ui.JobsListEntryAt, true
	case InputModeCommandsView:
		return ui.CommandsListEntryAt, true
	case InputModeMessagesView:
		return ui.MessagesListEntryAt, true
	case InputModeCompareView:
		return ui.CompareListEntryAt, true
	}
	return nil, false
}

// auxListHover is the row of the open Jobs, Commands, Messages, Compare or Duplicates list under the last pointer
// position: a job or source row, a command entry, a whole message. It only highlights: the
// selection stays put, so crossing the list on the way to the Details, Conflict or output panes
// does not switch what they show. A key press hides it until the pointer reports again.
func (a *App) auxListHover() ui.ListRowHover {
	hit, ok := a.auxListHit()
	dedup := a.inputMode() == InputModeDedupView
	unique := a.inputMode() == InputModeUniqueView
	if (!ok && !dedup && !unique) || !a.mouse.hovers() || a.mouseGoesToDialog() {
		return ui.ListRowHover{}
	}
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if unique {
		row, ok := ui.UniqueListEntryAt(layout, &a.model, a.mouse.x, a.mouse.y)
		return ui.ListRowHover{Active: ok, Row: row}
	}
	if dedup {
		row, copies, ok := ui.DedupListEntryAt(layout, &a.model, a.mouse.x, a.mouse.y)
		return ui.ListRowHover{Active: ok, Row: row, Copies: copies}
	}
	row, ok := hit(layout, &a.model, a.mouse.x, a.mouse.y)
	return ui.ListRowHover{Active: ok, Row: row}
}

// statusCommandsHover reports whether the last pointer position is on the status-command area,
// where a click would open the Status commands dialog.
func (a *App) statusCommandsHover() bool {
	if !a.mouse.hovers() || !a.menuBarAcceptsMouse() {
		return false
	}
	w, h := a.screen.Size()
	return ui.StatusCommandsAt(a.layoutForTerminalSize(w, h), &a.model, a.mouse.x, a.mouse.y)
}

// panelRowHover is the browser panel entry under the last pointer position (mouseHit), in
// normal mode only, hidden after a key press until the pointer reports again.
func (a *App) panelRowHover() ui.PanelRowHover {
	if !a.mouse.hovers() {
		return ui.PanelRowHover{}
	}
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return ui.PanelRowHover{}
	}
	if a.inputMode() == InputModeDedupView {
		row, ok := ui.DedupBrowseEntryAt(layout, &a.model, a.mouse.x, a.mouse.y)
		return ui.PanelRowHover{Active: ok, Panel: ui.DedupBrowsePanel, Entry: row}
	}
	if a.inputMode() == InputModeUniqueView {
		row, ok := ui.UniqueDestEntryAt(layout, &a.model, a.mouse.x, a.mouse.y)
		return ui.PanelRowHover{Active: ok, Panel: a.inactivePanelID(), Entry: row}
	}
	if a.inputMode() != InputModeNormal || a.model.ViewMode != ui.ViewBrowser {
		return ui.PanelRowHover{}
	}
	id, row, ok := a.mouseHit(layout, a.mouse.x, a.mouse.y)
	if !ok || row < 0 {
		return ui.PanelRowHover{}
	}
	return ui.PanelRowHover{Active: true, Panel: id, Entry: row}
}

// refreshPointerHovers sets the highlights that follow the last pointer position (dialog buttons
// and list rows,
// menu titles, the status commands, footer entries, file list rows, Jobs, Commands and Messages rows) and reports
// whether one changed. render calls it, so they track the screen as it changes under a still
// pointer; motion calls it and renders on a change.
func (a *App) refreshPointerHovers() bool {
	button, dialogRow := a.dialogHovers()
	menuTitle, footer := a.menuBarHover(), a.footerHover()
	status, row, listRow := a.statusCommandsHover(), a.panelRowHover(), a.auxListHover()
	prev := a.model.DialogButtonHover
	changed := button.Active != prev.Active || button.ID != prev.ID || button.Rect != prev.Rect ||
		menuTitle != a.model.MenuBarHover || footer != a.model.FooterHover ||
		status != a.model.StatusCommandsHover || row != a.model.PanelRowHover ||
		listRow != a.model.AuxListHover || dialogRow != a.model.DialogListHover
	a.model.DialogButtonHover, a.model.DialogListHover = button, dialogRow
	a.model.MenuBarHover, a.model.FooterHover = menuTitle, footer
	a.model.StatusCommandsHover, a.model.PanelRowHover, a.model.AuxListHover = status, row, listRow
	return changed
}

// clearPanelTitleHover drops the title path highlight; it reports whether one was shown.
func (a *App) clearPanelTitleHover() bool {
	was := a.model.PanelTitleHover.Active
	a.model.PanelTitleHover = ui.PanelTitleHover{}
	return was
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

// armPanelDrag prepares a drag from entry row of panel id, the row a press of button at screen row
// y just landed on.
func (a *App) armPanelDrag(id, row, y int, button tcell.ButtonMask) {
	if e, _, ok := a.panelByID(id).VisibleEntry(row); ok {
		a.mouse.drag = panelDrag{armed: true, button: button, panel: id, lastY: y, anchor: e.Path}
	}
}

// cancelPanelDrag drops the drag without selecting anything and reports whether an active drag
// ended, which the caller must paint.
func (a *App) cancelPanelDrag() bool {
	d := a.mouse.drag
	a.mouse.drag = panelDrag{}
	if d.active {
		a.panelByID(d.panel).ClearPendingRange()
	}
	return d.active
}

// panelDragRow maps pointer row y to the entry a drag targets, and returns y clamped to the list.
// A list that fits the panel never scrolls, so the entry is the one under the pointer. A longer
// list can scroll on every cursor move (edge and center scroll modes), so the same screen row
// keeps meaning another entry: there the cursor follows the pointer's row changes, and past the
// top or bottom of the list each report steps one row.
func panelDragRow(file ui.Rect, p *panel.State, y, lastY int) (row, listY int) {
	top, rows, count := ui.PanelListTop(file), ui.PanelListRows(file), p.VisibleEntryCount()
	listY = max(top, min(y, top+rows-1))
	switch {
	case count <= rows:
		row = p.ScrollOffset + listY - top
	case y < top:
		row = p.Cursor - 1
	case y >= top+rows:
		row = p.Cursor + 1
	default:
		row = p.Cursor + listY - lastY
	}
	return max(0, min(row, count-1)), listY
}

// movePanelDrag activates the drag if it is not yet and moves the cursor to entry target, reporting
// whether anything visible changed.
func (a *App) movePanelDrag(p *panel.State, target int) bool {
	d := &a.mouse.drag
	changed := !d.active
	if !d.active {
		d.active = true
		p.RangeAnchor, p.RangeUnselect = d.anchor, d.button == tcell.Button2
		a.clearPanelTitleHover()
	}
	if target != p.Cursor {
		a.setCursorByMouse(target)
		changed = true
	}
	return changed
}

// handlePanelDrag runs ahead of everything else in handleMouse while a drag is armed, so the
// footer, menus and other panel ignore the pointer. The drag stays with its panel: only y counts.
// A release on an active left drag commits through panel.select-range, the keyboard's path, which
// keeps the cursor where it is; an active right drag unselects the range. A right button released
// without activating was a right click and toggles the row. handled is false when the event
// belongs to the normal handling: a left button released without activating, just a click.
func (a *App) handlePanelDrag(layout ui.Layout, y int, buttons tcell.ButtonMask) (handled, rendered bool) {
	d := &a.mouse.drag
	if !d.armed {
		return false, false
	}
	p := a.panelByID(d.panel)
	file, ok := a.panelFileFrame(layout, d.panel)
	if !ok || layout.TooSmall || a.inputMode() != InputModeNormal || a.model.ViewMode != ui.ViewBrowser ||
		a.model.ActivePanel != d.panel || p.VisibleEntryCount() == 0 {
		if !a.cancelPanelDrag() {
			return false, false
		}
		a.render()
		return true, true
	}
	target, listY := panelDragRow(file, p, y, d.lastY)
	step := wheelDelta(buttons, config.MouseWheelRows)
	// A wheel report carries no button bits even with the left button held, so it is no release.
	release := step == 0 && buttons&d.button == 0
	if step != 0 {
		target = max(0, min(p.Cursor+step, p.VisibleEntryCount()-1))
	}
	moved := y != d.lastY || step != 0 // a click scrolling the list must not count as a drag
	if release {
		moved = y == listY && moved // a release past the edge is not one more scroll step
	}
	changed := false
	if moved {
		changed = a.movePanelDrag(p, target)
	}
	d.lastY = listY
	switch {
	case release && !d.active:
		right := d.button == tcell.Button2
		a.mouse.drag = panelDrag{}
		if !right {
			return false, false
		}
		a.tryDispatchSelectionActions(keymap.ActionPanelSelectToggle)
	case release:
		switch span := p.PendingRange(); {
		case !span.Active:
			p.ClearPendingRange()
		case span.Unselect:
			a.announceRange(primitive.RangeUnselected, p.UnselectPendingRange())
		default:
			a.tryDispatchSelectionActions(keymap.ActionPanelSelectRange)
		}
		a.mouse.drag = panelDrag{}
	case !changed:
		return true, false
	}
	a.render()
	return true, true
}

// handleMouse handles one mouse event. An armed file list drag takes it first
// (handlePanelDrag). The footer, the menu bar and its pulldown take clicks in
// every mode that shows them; the browser panels react in normal mode, the Jobs, Commands and
// Messages lists in their views, and the top-most dlg dialog in every other mode
// (handleDialogMouse). Everything else (other panes and views, the quick filter) swallows it.
func (a *App) handleMouse(ev *tcell.EventMouse) (quit, rendered bool) {
	buttons := ev.Buttons()
	pressed := buttons &^ a.mouse.buttons
	a.mouse.buttons = buttons &^ (tcell.WheelUp | tcell.WheelDown | tcell.WheelLeft | tcell.WheelRight)
	a.mouse.pointer, a.mouse.keyed = true, false
	a.mouse.x, a.mouse.y = ev.Position()
	if a.terminalPanelHasKeyFocus() {
		return false, false
	}
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	x, y := ev.Position()
	if handled, rendered := a.handlePanelDrag(layout, y, buttons); handled {
		return false, rendered
	}
	if a.inputMode() == InputModeFilePreviewView {
		left := pressed&tcell.Button1 != 0
		double := left && a.mouse.doubleClick("preview", y)
		if text, handled := a.previewCtrl.HandleFullscreenSelectionMouse(x, y, left, double, buttons&tcell.Button1 != 0); handled {
			a.copyPreviewSelection(text)
			a.render()
			return false, true
		}
	}
	// Motion and releases carry no new press: only the hover highlights react to them.
	if pressed == 0 && wheelDelta(buttons, 1) == 0 {
		if layout.TooSmall {
			return false, false
		}
		// Each one updates its own state, so all of them run before deciding to render.
		moved := a.selectPulldownItemUnderPointer(layout, x, y)
		hovers := a.refreshPointerHovers()
		if title := a.updatePanelTitleHover(layout, x, y); moved || hovers || title {
			a.render()
			return false, true
		}
		return false, false
	}
	if layout.TooSmall {
		return false, false
	}
	if handled, quit, rendered := a.handleChromeMouse(layout, x, y, buttons, pressed); handled {
		return quit, rendered
	}
	if a.mouseGoesToDialog() {
		return a.handleDialogMouse(x, y, buttons, pressed)
	}
	switch a.inputMode() {
	case InputModeJobsView, InputModeCommandsView, InputModeMessagesView:
		return false, a.handleAuxListMouse(layout, x, y, buttons, pressed)
	case InputModeCompareView:
		return false, a.handleAuxListMouse(layout, x, y, buttons, pressed)
	case InputModeDedupView:
		return false, a.handleDedupMouse(layout, x, y, buttons, pressed)
	case InputModeUniqueView:
		return false, a.handleUniqueMouse(layout, x, y, buttons, pressed)
	case InputModeNormal:
		if a.model.ViewMode != ui.ViewBrowser {
			return false, false
		}
	}
	if a.handlePanelHeadingRightClick(layout, x, y, pressed) {
		return false, true
	}
	if id, seg, ok := a.panelTitleHit(layout, x, y); ok && pressed&tcell.Button1 != 0 {
		a.focusPanelByMouse(id)
		if seg.Depth == 0 {
			// The panel directory itself: reload it.
			return a.finishResolvedKeyboardAction(keymap.ActionPanelRefresh)
		}
		if err := a.navigatePanelToAncestor(id, seg.Depth); err != nil {
			a.setErrorMessage("Navigate failed", err)
		}
		a.updatePanelTitleHover(layout, x, y)
		a.render()
		return false, true
	}
	if handled, rendered := a.handlePanelHeaderMouse(layout, x, y, pressed); handled {
		return false, rendered
	}
	id, row, ok := a.mouseHit(layout, x, y)
	if !ok {
		return false, a.wheelInactivePreview(layout, x, y, buttons)
	}
	switch step := wheelDelta(buttons, config.MouseWheelRows); {
	case step != 0:
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
			a.armPanelDrag(id, row, y, tcell.Button1)
			return false, true
		}
		return a.finishResolvedKeyboardAction(keymap.ActionNavOpen)
	case pressed&tcell.Button2 != 0: // tcell: Button2 is the right button
		a.focusPanelByMouse(id)
		if row < 0 {
			return false, true
		}
		a.setCursorByMouse(row)
		a.armPanelDrag(id, row, y, tcell.Button2) // the release toggles the row unless it drags
		return false, true
	}
	return false, false
}

// handlePanelHeadingRightClick opens the Columns dialog for the panel on a right press anywhere on
// its top border or column header row, and reports whether it did.
func (a *App) handlePanelHeadingRightClick(layout ui.Layout, x, y int, pressed tcell.ButtonMask) bool {
	if pressed&tcell.Button2 == 0 { // tcell: Button2 is the right button
		return false
	}
	id, file, ok := a.mousePanelAt(layout, x, y)
	if !ok {
		return false
	}
	if _, _, onHeader := ui.PanelHeaderColumnAt(file, &a.model, id, x, y); y != file.Y && !onHeader {
		return false
	}
	a.focusPanelByMouse(id)
	a.metaCtrl.OpenDialog(id)
	a.render()
	return true
}

// handlePanelHeaderMouse handles a left press on a file list's column header row: a sortable
// column title sorts by it (reversing when already sorted by it), anywhere else opens the sort
// dialog for that panel.
func (a *App) handlePanelHeaderMouse(layout ui.Layout, x, y int, pressed tcell.ButtonMask) (handled, rendered bool) {
	if pressed&tcell.Button1 == 0 {
		return false, false
	}
	id, file, ok := a.mousePanelAt(layout, x, y)
	if !ok {
		return false, false
	}
	col, onTitle, onHeader := ui.PanelHeaderColumnAt(file, &a.model, id, x, y)
	if !onHeader {
		return false, false
	}
	a.focusPanelByMouse(id)
	if onTitle {
		a.applyPanelSort(id, a.panelByID(id).HeaderColumnSort(col.Mode, col.MetaColumn))
	} else {
		a.openSortDialogForPanel(id)
	}
	a.render()
	return true, true
}

// wheelInactivePreview scrolls the preview in the inactive column on a wheel report over it, without
// taking focus, and reports whether it rendered.
func (a *App) wheelInactivePreview(layout ui.Layout, x, y int, buttons tcell.ButtonMask) (rendered bool) {
	step := wheelDelta(buttons, config.MouseWheelRows)
	if step == 0 {
		return false
	}
	id, col := ui.SecondaryPanel, layout.Secondary
	if a.model.ActivePanel == ui.SecondaryPanel {
		id, col = ui.PrimaryPanel, layout.Primary
	}
	if !col.Contains(x, y) {
		return false
	}
	if _, listed := a.panelFileFrame(layout, id); listed || !a.previewCtrl.ScrollInactiveColumnPreview(step) {
		return false
	}
	a.render()
	return true
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
func (a *App) handleChromeMouse(layout ui.Layout, x, y int, buttons, pressed tcell.ButtonMask) (handled, quit, rendered bool) {
	left := pressed&tcell.Button1 != 0
	if fk, shift, ok := ui.FooterKeyAt(layout, &a.model, x, y); ok {
		if !left {
			return true, false, false
		}
		event := fk.Event
		if shift {
			event = fk.ShiftEvent
		}
		ev, ok := event()
		if !ok {
			return true, false, false
		}
		// Paint even when the key path did not, so every mouse event that acts renders (the Run
		// loop skips the post-event pass for mouse events that did not render).
		quit, rendered := a.handleKey(ev)
		if quit {
			return true, true, false
		}
		if !rendered {
			a.render()
		}
		return true, false, true
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
	if ui.StatusCommandsAt(layout, &a.model, x, y) && a.menuBarAcceptsMouse() {
		if !left {
			return true, false, false
		}
		if a.model.Menu.Open {
			a.closeMenu()
		}
		a.openStatusCommandsDialog()
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
	switch delta := wheelDelta(buttons, 1); {
	case delta != 0:
		if !inside {
			return true, false, false
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
	case InputModeNormal, InputModeMenu, InputModeFilter, InputModeJobsView, InputModeCommandsView, InputModeMessagesView, InputModeCompareView, InputModeDedupView, InputModeUniqueView:
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
		a.closeLeaderMenu()
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
// focuses the list and selects the entry under the pointer (in Messages it also copies the
// message), the wheel moves the selection.
func (a *App) handleAuxListMouse(layout ui.Layout, x, y int, buttons, pressed tcell.ButtonMask) (rendered bool) {
	hit, ok := a.auxListHit()
	if !ok {
		return false
	}
	var selectEntry, move func(int)
	switch a.inputMode() {
	case InputModeJobsView:
		selectEntry, move = a.jobsCtrl.FocusListRow, a.jobsCtrl.MoveSelection
	case InputModeCommandsView:
		selectEntry, move = a.commandsCtrl.FocusListRow, a.commandsCtrl.MoveSelection
	case InputModeCompareView:
		selectEntry = func(idx int) { a.moveCompareSelection(idx - a.model.CompareView.Selected) }
		move = a.moveCompareSelection
	default:
		selectEntry, move = a.copyMessage, a.moveMessagesSelection
	}
	idx, ok := hit(layout, &a.model, x, y)
	if !ok {
		return false
	}
	switch step := wheelDelta(buttons, config.MouseWheelRows); {
	case step != 0:
		move(step)
	case pressed&tcell.Button1 != 0:
		selectEntry(idx)
	default:
		return false
	}
	a.render()
	return true
}

// handleDedupMouse handles the Duplicates view's two tree panes: a left click focuses the pane
// under the pointer and selects the row, the wheel moves the focused pane's selection.
func (a *App) handleDedupMouse(layout ui.Layout, x, y int, buttons, pressed tcell.ButtonMask) (rendered bool) {
	row, copies, ok := ui.DedupListEntryAt(layout, &a.model, x, y)
	if !ok {
		return a.handleDedupBrowseMouse(layout, x, y, buttons, pressed)
	}
	switch step := wheelDelta(buttons, config.MouseWheelRows); {
	case step != 0:
		a.dedupCtrl.MoveSelection(step)
	case pressed&tcell.Button1 != 0:
		a.dedupCtrl.FocusPaneRow(copies, row)
	default:
		return false
	}
	a.dedupCtrl.EnsureSelectionVisible(a.dedupVisibleRows())
	a.render()
	return true
}

// handleDedupBrowseMouse handles the Duplicates browse panel: a left click focuses it and moves
// its cursor to the entry under the pointer, the wheel moves the cursor.
func (a *App) handleDedupBrowseMouse(layout ui.Layout, x, y int, buttons, pressed tcell.ButtonMask) (rendered bool) {
	row, ok := ui.DedupBrowseEntryAt(layout, &a.model, x, y)
	if !ok {
		return false
	}
	p, rows := &a.model.DedupPanel, a.dedupBrowseRows(layout)
	switch step := wheelDelta(buttons, config.MouseWheelRows); {
	case step != 0:
		a.doListNav(func() { p.Move(step, rows) })
	case pressed&tcell.Button1 != 0:
		a.dedupCtrl.FocusBrowsePanel()
		a.doListNav(func() { p.Move(row-p.Cursor, rows) })
	default:
		return false
	}
	a.render()
	return true
}
