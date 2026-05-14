package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) switchPanel() {
	if a.model.ActivePanel == ui.LeftPanel {
		a.model.ActivePanel = ui.RightPanel
	} else {
		a.model.ActivePanel = ui.LeftPanel
	}
	a.model.ActiveSubFocus = ui.SubFocusFileList
}

func (a *App) reloadActive(successMessage string) {
	if a.model.ActivePanel == ui.LeftPanel {
		if err := a.model.Left.Refresh(a.activeViewportRows()); err != nil {
			a.setErrorMessage("Refresh failed", err)
			return
		}
		a.setTransientMessage(successMessage, ui.MessageUrgencyInfo)
		return
	}
	if err := a.model.Right.Refresh(a.activeViewportRows()); err != nil {
		a.setErrorMessage("Refresh failed", err)
		return
	}
	a.setTransientMessage(successMessage, ui.MessageUrgencyInfo)
}

func (a *App) activePanel() *panel.State {
	if a.model.ActivePanel == ui.LeftPanel {
		return &a.model.Left
	}
	return &a.model.Right
}

func (a *App) panelByID(panelID int) *panel.State {
	if panelID == ui.LeftPanel {
		return &a.model.Left
	}
	return &a.model.Right
}

func (a *App) activeViewportRows() int {
	return a.panelViewportRows(a.model.ActivePanel)
}

func (a *App) panelViewportRows(panelID int) int {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	col := layout.Left
	p := &a.model.Left
	if panelID == ui.RightPanel {
		col = layout.Right
		p = &a.model.Right
	}
	stripN := ui.SelectionsStripLayoutItemCount(p, panelID, a.model.ActivePanel, a.model.ThemeDialog.Open)
	fileCol, _ := ui.SplitPanelColumn(col, stripN, a.model.SelectionsPanelMaxRows, 3)
	return ui.PanelListRows(fileCol)
}

func (a *App) selectionsStripViewportRows(panelID int) int {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	col := layout.Left
	p := &a.model.Left
	if panelID == ui.RightPanel {
		col = layout.Right
		p = &a.model.Right
	}
	stripN := ui.SelectionsStripLayoutItemCount(p, panelID, a.model.ActivePanel, a.model.ThemeDialog.Open)
	_, stripCol := ui.SplitPanelColumn(col, stripN, a.model.SelectionsPanelMaxRows, 3)
	return ui.SelectionsStripListRows(stripCol)
}

func (a *App) toggleSelectionsStripFocus() {
	if a.model.ActiveSubFocus == ui.SubFocusSelectionsStrip {
		a.model.ActiveSubFocus = ui.SubFocusFileList
		return
	}
	if a.activePanel().SelectionsStripCount() > 0 {
		a.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
		a.activePanel().EnsureSelectionsStripCursorVisible(a.selectionsStripViewportRows(a.model.ActivePanel))
	}
}

// navigateFromSelectionsStrip opens the directory for the highlighted strip path in the active panel
// (the directory itself if the path is a directory, otherwise its parent) and focuses the file list.
func (a *App) navigateFromSelectionsStrip() {
	p := a.activePanel()
	selPath, ok := p.SelectedPathAtStripIndex(p.SelectionsStripCursor)
	if !ok {
		return
	}
	abs := filepath.Clean(selPath)
	info, err := os.Stat(abs)
	if err != nil {
		a.setErrorMessage("Cannot open path", err)
		return
	}
	var dirToLoad string
	var selectName string
	if info.IsDir() {
		dirToLoad = abs
	} else {
		dirToLoad = filepath.Clean(filepath.Dir(abs))
		selectName = filepath.Base(abs)
	}
	vr := a.activeViewportRows()
	if err := p.NavigateTo(dirToLoad, selectName, vr); err != nil {
		a.setErrorMessage("Open failed", err)
		return
	}
	p.EnsureCursorVisible(vr)
	a.model.ActiveSubFocus = ui.SubFocusFileList
}

// navigatePanelToDirectory loads dirPath into panelID's listing and navigation history.
func (a *App) navigatePanelToDirectory(panelID int, dirPath, selectedName string) error {
	p := a.panelByID(panelID)
	vr := a.panelViewportRows(panelID)
	return p.NavigateTo(filepath.Clean(dirPath), selectedName, vr)
}

// toggleSyncFollow flips latched panel sync on the active panel with mutual exclusion:
// if the active panel already drives sync it is disabled; otherwise the other panel's sync
// (if any) is cleared first and the active panel becomes the new driver. Enabling fires
// one immediate sync hop so the follower lands on the highlighted folder right away.
func (a *App) toggleSyncFollow() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	active := a.model.ActivePanel
	if a.model.SyncFollowEnabled && a.model.SyncFollowPanel == active {
		a.model.SyncFollowEnabled = false
		a.clearPanelSyncFollowNavCoalesce()
		a.setTransientMessage("Sync disabled", ui.MessageUrgencyInfo)
		return
	}
	a.clearPanelSyncFollowNavCoalesce()
	a.model.SyncFollowEnabled = true
	a.model.SyncFollowPanel = active
	arrow := "→"
	driver := "Left"
	follower := "Right"
	if active == ui.RightPanel {
		arrow = "←"
		driver = "Right"
		follower = "Left"
	}
	a.setTransientMessage(fmt.Sprintf("Sync: %s %s %s", driver, arrow, follower), ui.MessageUrgencyInfo)
	a.syncFollowFromActive()
}

// reconcileAfterEvent runs after each input event in Run(). It re-establishes derived
// UI invariants. Each invariant must be idempotent (a no-op when state is already
// consistent). New invariants belong here, not sprinkled at call sites: any code path
// that mutates panel state automatically triggers them via the Run-loop chokepoint.
func (a *App) reconcileAfterEvent() {
	a.handlePanelDirChanged(ui.LeftPanel)
	a.handlePanelDirChanged(ui.RightPanel)
	a.handleMetaPanelDirChanged(ui.LeftPanel)
	a.handleMetaPanelDirChanged(ui.RightPanel)
	// Panel sync reads the driver's highlight after idle-sort / meta hooks may adjust cursors.
	if !a.syncFollowNavSkipReconcile.Load() {
		a.syncFollowFromActive()
	}
}

// syncFollowTargetPath returns the absolute directory path the follower should mirror when
// the driver panel is active. When keyboard focus is on the selections strip, the strip row
// wins so sync matches what the user is steering; otherwise the file-list cursor row is used.
func (a *App) syncFollowTargetPath(driver *panel.State) (string, bool) {
	if a.model.ActiveSubFocus == ui.SubFocusSelectionsStrip && driver.SelectionsStripCount() > 0 {
		p, ok := driver.SelectedPathAtStripIndex(driver.SelectionsStripCursor)
		if !ok {
			return "", false
		}
		p = filepath.Clean(p)
		if p == "" || p == "." {
			return "", false
		}
		fi, err := os.Stat(p)
		if err != nil {
			return "", false
		}
		if fi.IsDir() {
			return p, true
		}
		// Strip row is a file: mirror its parent directory (common "work here" intent).
		parent := filepath.Clean(filepath.Dir(p))
		if parent == "" || parent == p {
			return "", false
		}
		if fi2, err2 := os.Stat(parent); err2 != nil || !fi2.IsDir() {
			return "", false
		}
		return parent, true
	}
	entry, ok := driver.CurrentEntry()
	if !ok || entry.Type != localfs.EntryDirectory {
		return "", false
	}
	return filepath.Clean(entry.Path), true
}

// syncFollowFromActive mirrors the active panel's highlighted directory into the inactive panel
// when the active panel drives latched sync. Uses panel.State.Load (no NavigateTo) so the
// follower's directory history is left intact. Non-directory cursor entries are silent no-ops.
// Idempotent: when the follower already sits at the target path it returns without work.
func (a *App) syncFollowFromActive() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if !a.model.SyncFollowEnabled || a.model.SyncFollowPanel != a.model.ActivePanel {
		return
	}
	driver := a.panelByID(a.model.SyncFollowPanel)
	targetPath, ok := a.syncFollowTargetPath(driver)
	if !ok {
		return
	}
	followerID := a.inactivePanelID()
	follower := a.panelByID(followerID)
	if filepath.Clean(follower.Path) == targetPath {
		return
	}
	if err := follower.Load(targetPath); err != nil {
		return
	}
	follower.EnsureCursorVisible(a.panelViewportRows(followerID))
}

func panelSyncFollowListNavAction(actionID string) bool {
	switch actionID {
	case keymap.ActionNavUp, keymap.ActionNavDown,
		keymap.ActionNavPageUp, keymap.ActionNavPageDown,
		keymap.ActionNavTop, keymap.ActionNavBottom:
		return true
	default:
		return false
	}
}

// panelSyncFollowNavCoalesceContext is true when latched sync should mirror the file-list cursor
// (not the selections strip) from the active driver panel.
func (a *App) panelSyncFollowNavCoalesceContext() bool {
	return a.model.ViewMode == ui.ViewBrowser &&
		a.model.SyncFollowEnabled &&
		a.model.SyncFollowPanel == a.model.ActivePanel &&
		a.model.ActiveSubFocus == ui.SubFocusFileList &&
		!a.inQuickFilterUI()
}

// panelSyncFollowHeldListNav is true when this key event will move the file-list cursor via
// the normal browser dispatch path (used to extend debounced sync vs clearing it).
func (a *App) panelSyncFollowHeldListNav(resolvedAction string, event *tcell.EventKey) bool {
	if !panelSyncFollowListNavAction(resolvedAction) {
		return false
	}
	if a.inputMode() != InputModeNormal {
		return false
	}
	if a.model.ViewMode != ui.ViewBrowser || a.model.ActiveSubFocus != ui.SubFocusFileList {
		return false
	}
	if a.inQuickFilterUI() {
		return false
	}
	if a.shouldStartFilter(event) {
		return false
	}
	return true
}

// clearPanelSyncFollowNavCoalesce stops pending follower sync and allows reconcile to mirror again.
func (a *App) clearPanelSyncFollowNavCoalesce() {
	a.syncFollowNavMu.Lock()
	if a.syncFollowNavTimer != nil {
		if !a.syncFollowNavTimer.Stop() {
			select {
			case <-a.syncFollowNavTimer.C:
			default:
			}
		}
		a.syncFollowNavTimer = nil
	}
	a.syncFollowNavMu.Unlock()
	a.syncFollowNavGen.Add(1)
	a.syncFollowNavSkipReconcile.Store(false)
}

func (a *App) armPanelSyncFollowNavCoalesceAfterListNav() {
	if a.config.UI.PanelSyncFollowNavDebounceMS <= 0 {
		return
	}
	if !a.panelSyncFollowNavCoalesceContext() {
		return
	}
	gen := a.syncFollowNavGen.Add(1)
	delay := time.Duration(a.config.UI.PanelSyncFollowNavDebounceMS) * time.Millisecond
	a.syncFollowNavMu.Lock()
	defer a.syncFollowNavMu.Unlock()
	if a.syncFollowNavTimer != nil {
		if !a.syncFollowNavTimer.Stop() {
			select {
			case <-a.syncFollowNavTimer.C:
			default:
			}
		}
		a.syncFollowNavTimer = nil
	}
	a.syncFollowNavSkipReconcile.Store(true)
	a.syncFollowNavTimer = time.AfterFunc(delay, func() {
		a.syncFollowNavMu.Lock()
		a.syncFollowNavTimer = nil
		a.syncFollowNavMu.Unlock()
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(syncFollowNavFlushPayload{gen: gen}))
	})
}

// applyPanelSyncFollowNavFlush runs after the nav debounce elapses; returns whether a repaint is needed.
func (a *App) applyPanelSyncFollowNavFlush(p syncFollowNavFlushPayload) bool {
	if p.gen != a.syncFollowNavGen.Load() {
		return false
	}
	a.syncFollowNavSkipReconcile.Store(false)
	a.syncFollowFromActive()
	return true
}

// tryDispatchSelectionsStrip handles actions while the selections strip has keyboard focus.
func (a *App) tryDispatchSelectionsStrip(actionID string) bool {
	if a.model.ViewMode != ui.ViewBrowser || a.model.ActiveSubFocus != ui.SubFocusSelectionsStrip {
		return false
	}
	vr := a.selectionsStripViewportRows(a.model.ActivePanel)
	p := a.activePanel()
	switch actionID {
	case keymap.ActionNavUp:
		p.MoveSelectionsStrip(-1, vr)
	case keymap.ActionNavDown:
		p.MoveSelectionsStrip(1, vr)
	case keymap.ActionNavPageUp:
		step := vr
		if step < 1 {
			step = 1
		}
		p.MoveSelectionsStrip(-step, vr)
	case keymap.ActionNavPageDown:
		step := vr
		if step < 1 {
			step = 1
		}
		p.MoveSelectionsStrip(step, vr)
	case keymap.ActionNavTop:
		p.SelectionsStripTop(vr)
	case keymap.ActionNavBottom:
		p.SelectionsStripBottom(vr)
	case keymap.ActionPanelSelectToggle:
		p.ToggleOrRemoveStripSelection()
		if p.SelectionsStripCount() == 0 {
			a.model.ActiveSubFocus = ui.SubFocusFileList
		}
	case keymap.ActionPanelFocusSelections:
		a.toggleSelectionsStripFocus()
	case keymap.ActionPanelSwitch:
		a.switchPanel()
	case keymap.ActionAppOpenMenu:
		a.openMenu()
	case keymap.ActionNavOpen:
		a.navigateFromSelectionsStrip()
	default:
		return false
	}
	return true
}

func (a *App) ensurePanelsVisible() {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		a.model.Left.EnsureCursorVisible(0)
		a.model.Right.EnsureCursorVisible(0)
		return
	}
	leftN := ui.SelectionsStripLayoutItemCount(&a.model.Left, ui.LeftPanel, a.model.ActivePanel, a.model.ThemeDialog.Open)
	rightN := ui.SelectionsStripLayoutItemCount(&a.model.Right, ui.RightPanel, a.model.ActivePanel, a.model.ThemeDialog.Open)
	leftFile, _ := ui.SplitPanelColumn(layout.Left, leftN, a.model.SelectionsPanelMaxRows, 3)
	rightFile, _ := ui.SplitPanelColumn(layout.Right, rightN, a.model.SelectionsPanelMaxRows, 3)
	a.model.Left.EnsureCursorVisible(ui.PanelListRows(leftFile))
	a.model.Right.EnsureCursorVisible(ui.PanelListRows(rightFile))
}

func panelLabel(panelID int) string {
	if panelID == ui.LeftPanel {
		return "Left panel"
	}
	return "Right panel"
}
