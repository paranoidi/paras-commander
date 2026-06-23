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
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func scrollModeFromConfig(scrollMode string) panel.ScrollMode {
	m, err := panel.ParseScrollMode(scrollMode)
	if err != nil {
		return panel.ScrollModeEdge
	}
	return panel.EffectiveScrollMode(m)
}

func (a *App) syncScrollFromConfig() {
	mode := scrollModeFromConfig(a.config.UI.ScrollMode)
	margin := a.config.UI.ScrollEdgeMargin
	a.model.Primary.ScrollMode = mode
	a.model.Secondary.ScrollMode = mode
	a.model.Primary.ScrollEdgeMargin = margin
	a.model.Secondary.ScrollEdgeMargin = margin
}

func (a *App) switchPanel() {
	leavingDisplay := a.model.QuickViewDisplayActive()
	a.switchPanelSwap()
	if leavingDisplay {
		a.pauseQuickViewDisplay()
	}
	if a.model.QuickViewDisplayActive() {
		a.resumeQuickViewDisplay()
	}
}

func (a *App) switchPanelSwap() {
	if a.model.HideInactivePanel {
		a.model.HideInactivePanel = false
		if a.model.ActivePanel == ui.PrimaryPanel {
			a.model.ActivePanel = ui.SecondaryPanel
		} else {
			a.model.ActivePanel = ui.PrimaryPanel
		}
		a.model.ActiveSubFocus = ui.SubFocusFileList
		return
	}
	if a.model.ActivePanel == ui.PrimaryPanel {
		a.model.ActivePanel = ui.SecondaryPanel
	} else {
		a.model.ActivePanel = ui.PrimaryPanel
	}
	a.model.ActiveSubFocus = ui.SubFocusFileList
}

// toggleHideInactivePanel hides or shows the inactive twin panel (browser only).
func (a *App) toggleHideInactivePanel() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if a.model.HideInactivePanel {
		a.model.HideInactivePanel = false
		a.setTransientMessage("Inactive panel shown", ui.MessageUrgencyInfo)
		return
	}
	hadSync := a.model.SyncFollowEnabled
	hadQuickView := a.model.QuickViewEnabled
	if hadSync {
		a.model.SyncFollowEnabled = false
		a.clearPanelSyncFollowNavCoalesce()
	}
	if hadQuickView {
		a.model.QuickViewEnabled = false
		a.model.QuickViewPanel = -1
		a.clearQuickViewDebounce()
		a.closeFilePreview()
		a.clearQuickViewDirOverlay()
		a.quickViewLastFingerprint = ""
	}
	a.model.HideInactivePanel = true
	switch {
	case hadSync && hadQuickView:
		a.setTransientMessage("Inactive panel hidden — sync and quick view disabled", ui.MessageUrgencyWarn)
	case hadSync:
		a.setTransientMessage("Inactive panel hidden — sync disabled", ui.MessageUrgencyWarn)
	case hadQuickView:
		a.setTransientMessage("Inactive panel hidden — quick view disabled", ui.MessageUrgencyWarn)
	default:
		a.setTransientMessage("Inactive panel hidden", ui.MessageUrgencyInfo)
	}
}

func (a *App) reloadActive(successMessage string) {
	if a.model.ActivePanel == ui.PrimaryPanel {
		if err := a.model.Primary.Refresh(a.activeViewportRows()); err != nil {
			a.setErrorMessage("Refresh failed", err)
			return
		}
		a.requestVolumeSpaceRefreshAsync(ui.SecondaryPanel)
		a.syncOpenPathInputsAfterFSChange()
		a.setTransientMessage(successMessage, ui.MessageUrgencyInfo)
		return
	}
	if err := a.model.Secondary.Refresh(a.activeViewportRows()); err != nil {
		a.setErrorMessage("Refresh failed", err)
		return
	}
	a.requestVolumeSpaceRefreshAsync(ui.PrimaryPanel)
	a.syncOpenPathInputsAfterFSChange()
	a.setTransientMessage(successMessage, ui.MessageUrgencyInfo)
}

func (a *App) activePanel() *panel.State {
	if a.model.ActivePanel == ui.PrimaryPanel {
		return &a.model.Primary
	}
	return &a.model.Secondary
}

func (a *App) panelByID(panelID int) *panel.State {
	if panelID == ui.PrimaryPanel {
		return &a.model.Primary
	}
	return &a.model.Secondary
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
	col := layout.Primary
	p := &a.model.Primary
	if panelID == ui.SecondaryPanel {
		col = layout.Secondary
		p = &a.model.Secondary
	}
	return ui.FileListViewportRows(col, p, panelID, a.model.ActivePanel, a.model.ThemeDialog.Open, a.model.SelectionsPanelMaxRows)
}

func (a *App) wireFileListViewportRows() {
	a.model.Primary.FileListViewportRows = func() int { return a.panelViewportRows(ui.PrimaryPanel) }
	a.model.Secondary.FileListViewportRows = func() int { return a.panelViewportRows(ui.SecondaryPanel) }
}

func (a *App) selectionsStripViewportRows(panelID int) int {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	col := layout.Primary
	p := &a.model.Primary
	if panelID == ui.SecondaryPanel {
		col = layout.Secondary
		p = &a.model.Secondary
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
	p.EnsureCursorInViewport(vr)
	a.model.ActiveSubFocus = ui.SubFocusFileList
}

// navigatePanelToDirectory loads dirPath into panelID's listing and navigation history.
func (a *App) navigatePanelToDirectory(panelID int, dirPath, selectedName string) error {
	p := a.panelByID(panelID)
	vr := a.panelViewportRows(panelID)
	loc, err := pathloc.Parse(dirPath)
	if err != nil {
		return err
	}
	return p.NavigateToPath(loc, selectedName, vr)
}

// disableSyncFollowIfEnabled turns off latched panel sync; returns whether it was on.
func (a *App) disableSyncFollowIfEnabled() bool {
	if !a.model.SyncFollowEnabled {
		return false
	}
	a.model.SyncFollowEnabled = false
	a.clearPanelSyncFollowNavCoalesce()
	return true
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
	displacedQuickView := a.model.QuickViewEnabled
	if displacedQuickView {
		a.model.QuickViewEnabled = false
		a.model.QuickViewPanel = -1
		a.clearQuickViewDebounce()
		a.closeFilePreview()
		a.clearQuickViewDirOverlay()
		a.quickViewLastFingerprint = ""
	}
	a.clearPanelSyncFollowNavCoalesce()
	a.model.SyncFollowEnabled = true
	a.model.SyncFollowPanel = active
	arrow, driver, follower := ui.SyncFollowToastParts(active, a.effectivePaneSplitOrientation())
	if displacedQuickView {
		a.setTransientMessage(fmt.Sprintf("Sync: %s %s %s — quick view disabled", driver, arrow, follower), ui.MessageUrgencyWarn)
	} else {
		a.setTransientMessage(fmt.Sprintf("Sync: %s %s %s", driver, arrow, follower), ui.MessageUrgencyInfo)
	}
	a.syncFollowFromActive()
}

// reconcileAfterEvent runs after each input event in Run(). It re-establishes derived
// UI invariants. Each invariant must be idempotent (a no-op when state is already
// consistent). New invariants belong here, not sprinkled at call sites: any code path
// that mutates panel state automatically triggers them via the Run-loop chokepoint.
func (a *App) reconcileAfterEvent() {
	a.reconcileSelectionSizeScans(ui.PrimaryPanel)
	a.reconcileSelectionSizeScans(ui.SecondaryPanel)
	a.reconcileDeleteDialogScans()
	a.reconcileFindDialogSelectionSizeScans()
	a.handlePanelDirChanged(ui.PrimaryPanel)
	a.handlePanelDirChanged(ui.SecondaryPanel)
	a.handleMetaPanelDirChanged(ui.PrimaryPanel)
	a.handleMetaPanelDirChanged(ui.SecondaryPanel)
	a.reconcileMetaForContentChanges(ui.PrimaryPanel)
	a.reconcileMetaForContentChanges(ui.SecondaryPanel)
	// Panel sync reads the driver's highlight after idle-sort / meta hooks may adjust cursors.
	if !a.syncFollowNavSkipReconcile.Load() {
		a.syncFollowFromActive()
	}
	a.reconcileQuickViewPreview()
	a.reconcileCarouselFilePreview()
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
		p = panel.CleanPathString(p)
		if p == "" {
			return "", false
		}
		if a.pathVolumeContendsWithActiveJob(p) {
			parent := panel.CleanPathString(filepath.Dir(p))
			if parent != "" && parent != p {
				return parent, true
			}
			return p, true
		}
		fi, err := os.Stat(p)
		if err != nil {
			return "", false
		}
		if fi.IsDir() {
			return p, true
		}
		// Strip row is a file: mirror its parent directory (common "work here" intent).
		parent := panel.CleanPathString(filepath.Dir(p))
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
	return panel.CleanPathString(entry.Path), true
}

// syncFollowFromActive mirrors the active panel's highlighted directory into the inactive panel
// when the active panel drives latched sync. Uses panel.State.Load (no NavigateTo) so the
// follower's directory history is left intact. Non-directory cursor entries are silent no-ops.
// Idempotent: when the follower already sits at the target path it returns without work.
func (a *App) syncFollowFromActive() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	a.commandsMu.RLock()
	previewOpen := a.model.FilePreview.Open || a.model.QuickViewDisplayActive()
	a.commandsMu.RUnlock()
	if previewOpen {
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
	if filepath.Clean(follower.PathString()) == targetPath {
		return
	}
	if a.pathVolumeContendsWithActiveJob(targetPath) {
		return
	}
	if err := follower.Load(targetPath); err != nil {
		return
	}
	follower.EnsureCursorInViewport(a.panelViewportRows(followerID))
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
	a.syncFollowNav.Clear()
	a.syncFollowNavGen.Add(1)
	a.syncFollowNavSkipReconcile.Store(false)
}

func (a *App) armPanelSyncFollowNavCoalesceAfterListNav() {
	if a.config.UI.KeyRepeatDebounceMS <= 0 {
		return
	}
	if !a.panelSyncFollowNavCoalesceContext() {
		return
	}
	gen := a.syncFollowNavGen.Add(1)
	delay := time.Duration(a.config.UI.KeyRepeatDebounceMS) * time.Millisecond
	a.syncFollowNavSkipReconcile.Store(true)
	a.syncFollowNav.Reset(delay, func() {
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
		if a.model.QuickViewEnabled {
			a.switchPanel()
		} else if a.filePreviewOpen() {
			a.model.ActiveSubFocus = ui.SubFocusInactivePreview
		} else {
			a.switchPanel()
		}
	case keymap.ActionPanelClearSelection:
		p.ClearSelection()
		a.model.ActiveSubFocus = ui.SubFocusFileList
		a.setTransientMessage("Selection cleared", ui.MessageUrgencyInfo)
	case keymap.ActionNavOpen:
		a.navigateFromSelectionsStrip()
	case keymap.ActionPanelToggleSync:
		return false
	default:
		if actionID != "" {
			return true
		}
		return false
	}
	return true
}

func (a *App) ensurePanelsVisible() {
	if a.model.ViewMode == ui.ViewMessages {
		a.ensureMessagesViewSelectionVisible()
		return
	}
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		a.model.Primary.EnsureCursorInViewport(0)
		a.model.Secondary.EnsureCursorInViewport(0)
		return
	}
	a.model.Primary.EnsureCursorInViewport(a.panelViewportRows(ui.PrimaryPanel))
	a.model.Secondary.EnsureCursorInViewport(a.panelViewportRows(ui.SecondaryPanel))
}

func panelLabel(panelID int) string {
	if panelID == ui.PrimaryPanel {
		return "Primary panel"
	}
	return "Secondary panel"
}

func (a *App) inactivePanel() *panel.State {
	if a.model.ActivePanel == ui.PrimaryPanel {
		return &a.model.Secondary
	}
	return &a.model.Primary
}

func (a *App) inactivePanelID() int {
	if a.model.ActivePanel == ui.PrimaryPanel {
		return ui.SecondaryPanel
	}
	return ui.PrimaryPanel
}
