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
	mode := scrollModeFromConfig(a.config.UI.Scroll.Mode)
	margin := a.config.UI.Scroll.EdgeMargin
	a.model.Primary.ScrollMode = mode
	a.model.Secondary.ScrollMode = mode
	a.model.Primary.ScrollEdgeMargin = margin
	a.model.Secondary.ScrollEdgeMargin = margin
}

func (a *App) switchPanel() {
	leavingDisplay := a.model.QuickViewDisplayActive()
	a.switchPanelSwap()
	if leavingDisplay {
		a.previewCtrl.PauseQuickViewDisplay()
	}
	if a.model.QuickViewDisplayActive() {
		a.previewCtrl.ResumeQuickViewDisplay()
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
		a.previewCtrl.DisableQuickViewDisplay()
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
		a.dialogCtrl.SyncOpenPathInputsAfterFSChange()
		a.setTransientMessage(successMessage, ui.MessageUrgencyInfo)
		return
	}
	if err := a.model.Secondary.Refresh(a.activeViewportRows()); err != nil {
		a.setErrorMessage("Refresh failed", err)
		return
	}
	a.requestVolumeSpaceRefreshAsync(ui.PrimaryPanel)
	a.dialogCtrl.SyncOpenPathInputsAfterFSChange()
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

// navigateToSelectionsRoot moves the active panel to the deepest common ancestor of its
// selected paths — the directory copy/move is permitted from when selections span
// multiple parent directories.
func (a *App) navigateToSelectionsRoot() {
	root, _, ok := a.activePanel().SelectionsCommonRoot()
	if !ok {
		a.setTransientMessage("No selections", ui.MessageUrgencyInfo)
		return
	}
	if a.activePanel().Path.Equal(root) {
		a.setTransientMessage("Already at selections root", ui.MessageUrgencyInfo)
		return
	}
	if err := a.navigatePanelToDirectory(a.model.ActivePanel, root.String(), ""); err != nil {
		a.setErrorMessage("Navigate failed", err)
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
		a.previewCtrl.DisableQuickViewDisplay()
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
	a.dialogCtrl.ReconcileDeleteDialogScans()
	a.reconcileFindDialogSelectionSizeScans()
	a.handlePanelDirChanged(ui.PrimaryPanel)
	a.handlePanelDirChanged(ui.SecondaryPanel)
	a.metaCtrl.HandlePanelDirChanged(ui.PrimaryPanel)
	a.metaCtrl.HandlePanelDirChanged(ui.SecondaryPanel)
	a.metaCtrl.ReconcileForPanel(ui.PrimaryPanel)
	a.metaCtrl.ReconcileForPanel(ui.SecondaryPanel)
	// Panel sync reads the driver's highlight after idle-sort / meta hooks may adjust cursors.
	if !a.syncFollowNavSkipReconcile.Load() {
		a.syncFollowFromActive()
	}
	a.previewCtrl.ReconcileQuickViewPreview()
	a.previewCtrl.ReconcileCarouselFilePreview()
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
		} else if a.previewCtrl.FilePreviewOpen() {
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
	case keymap.ActionFileView, keymap.ActionFileEdit:
		// Same handlers as the file list (OpenFilePreviewFullscreen / EditActiveFile),
		// targeting the highlighted strip row via ActiveSubFocus resolution.
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

// tryDispatchNavigation handles list-navigation and directory-navigation actions.
// handled reports whether actionID was consumed; quit reports whether the app should exit
// (only ActionNavOpen can request this, via handleNavOpen/handleChooserOpen).
func (a *App) tryDispatchNavigation(actionID string) (handled, quit bool) {
	viewportRows := a.activeViewportRows()
	activePanel := a.activePanel()
	switch actionID {
	case keymap.ActionNavUp:
		a.doListNav(func() { activePanel.Move(-1, viewportRows) })
	case keymap.ActionNavDown:
		a.doListNav(func() { activePanel.Move(1, viewportRows) })
	case keymap.ActionNavPageUp:
		a.doListNav(func() { activePanel.Page(-1, viewportRows) })
	case keymap.ActionNavPageDown:
		a.doListNav(func() { activePanel.Page(1, viewportRows) })
	case keymap.ActionNavTop:
		a.doListNav(func() { activePanel.Top(viewportRows) })
	case keymap.ActionNavBottom:
		a.doListNav(func() { activePanel.Bottom(viewportRows) })
	case keymap.ActionNavOpen:
		return true, a.handleNavOpen(activePanel, viewportRows)
	case keymap.ActionPanelOpenDirInOther:
		if a.model.ViewMode != ui.ViewBrowser {
			break
		}
		entry, ok := activePanel.CurrentEntry()
		if !ok || entry.Type != localfs.EntryDirectory {
			break
		}
		if err := a.navigatePanelToDirectory(a.inactivePanelID(), entry.Path, ""); err != nil {
			a.setErrorMessage("Open in other panel failed", err)
			break
		}
		if a.disableSyncFollowIfEnabled() {
			a.setTransientMessage("Open in other panel — sync disabled", ui.MessageUrgencyWarn)
		}
		activePanel.CycleFilterMatch(1, viewportRows)
	case keymap.ActionPanelOpenActivePathInOther:
		if a.model.ViewMode != ui.ViewBrowser {
			break
		}
		if err := a.navigatePanelToDirectory(a.inactivePanelID(), activePanel.PathString(), ""); err != nil {
			a.setErrorMessage("Open current path in other panel failed", err)
			break
		}
		if a.disableSyncFollowIfEnabled() {
			a.setTransientMessage("Open in other panel — sync disabled", ui.MessageUrgencyWarn)
		}
	case keymap.ActionNavParent:
		if err := activePanel.Parent(viewportRows); err != nil {
			a.setErrorMessage("Parent failed", err)
		}
	case keymap.ActionNavHome:
		if a.model.UserHomeDir == "" {
			break
		}
		if err := a.navigatePanelToDirectory(a.model.ActivePanel, a.model.UserHomeDir, ""); err != nil {
			a.setErrorMessage("Navigate to home failed", err)
		}
	case keymap.ActionNavForward:
		if _, err := activePanel.HistoryForward(viewportRows); err != nil {
			a.setErrorMessage("Forward history failed", err)
		}
	case keymap.ActionNavBackward:
		if _, err := activePanel.HistoryBackward(viewportRows); err != nil {
			a.setErrorMessage("Backward history failed", err)
		}
	default:
		return false, false
	}
	return true, false
}

// tryDispatchSelectionActions handles panel selection actions (toggle/group/invert/clear/stash).
func (a *App) tryDispatchSelectionActions(actionID string) bool {
	viewportRows := a.activeViewportRows()
	activePanel := a.activePanel()
	switch actionID {
	case keymap.ActionPanelSelectToggle:
		_, conflicts := activePanel.ToggleSelectionAndAdvance(viewportRows)
		if conflicts {
			a.setTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		}
	case keymap.ActionPanelSelectGroup:
		a.openGroupSelect("select", "panel")
	case keymap.ActionPanelUnselectGroup:
		a.openGroupSelect("unselect", "panel")
	case keymap.ActionPanelInvertSelection:
		activePanel.InvertSelection()
		a.setTransientMessage("Selection inverted", ui.MessageUrgencyInfo)
	case keymap.ActionPanelClearSelection:
		activePanel.ClearSelection()
		a.setTransientMessage("Selection cleared", ui.MessageUrgencyInfo)
	case keymap.ActionPanelStashToggle:
		a.togglePanelSelectionStash()
	default:
		return false
	}
	return true
}

// tryDispatchPanelLayout handles panel sort/listing-format/carousel/zoom/split/hidden-file actions.
func (a *App) tryDispatchPanelLayout(actionID string) bool {
	viewportRows := a.activeViewportRows()
	activePanel := a.activePanel()
	switch actionID {
	case keymap.ActionPanelToggleHideInactive:
		a.toggleHideInactivePanel()
	case keymap.ActionPanelSortDialog:
		a.openSortDialog()
	case keymap.ActionPanelListingFormatDialog:
		if activePanel.CarouselMode {
			a.setTransientMessage("Listing format is not available in carousel view", ui.MessageUrgencyInfo)
			break
		}
		a.openListingFormatDialog()
	case keymap.ActionPanelCycleSort:
		activePanel.CycleSort(viewportRows)
		a.setTransientMessage(fmt.Sprintf("Sort: %s", activePanel.Sort.Mode.String()), ui.MessageUrgencyInfo)
	case keymap.ActionPanelCycleListingFormat:
		if activePanel.CarouselMode {
			a.setTransientMessage("Listing format is not available in carousel view", ui.MessageUrgencyInfo)
			break
		}
		activePanel.CycleListingFormat()
		a.setTransientMessage(fmt.Sprintf("Listing: %s", activePanel.ListFormat.String()), ui.MessageUrgencyInfo)
	case keymap.ActionPanelToggleCarousel:
		activePanel.CarouselMode = !activePanel.CarouselMode
		if activePanel.CarouselMode {
			activePanel.SetListLayout(panel.ListLayoutFlat, viewportRows)
		} else {
			a.previewCtrl.ClearCarouselPreviewNavCoalesce()
			a.previewCtrl.CloseCarouselFilePreview()
		}
		onOff := "off"
		if activePanel.CarouselMode {
			onOff = "on"
		}
		a.setTransientMessage(fmt.Sprintf("Carousel view: %s", onOff), ui.MessageUrgencyInfo)
	case keymap.ActionPanelToggleTree:
		if a.toggleTreeForPanel(activePanel, viewportRows) {
			a.setTransientMessage("Tree view is not available in carousel view", ui.MessageUrgencyInfo)
		}
	case keymap.ActionPanelTreeExpand:
		if a.expandTreeForPanel(activePanel, viewportRows) {
			a.setTransientMessage("Tree view is not available in carousel view", ui.MessageUrgencyInfo)
		}
	case keymap.ActionPanelTreeCollapse:
		a.collapseTreeForPanel(activePanel, viewportRows)
	case keymap.ActionPanelTreeCollapseAll:
		a.collapseAllTreeForPanel(activePanel, viewportRows)
	case keymap.ActionPanelTreeExpandAllShallow:
		if a.expandAllTreeShallowForPanel(activePanel, viewportRows) {
			a.setTransientMessage("Tree view is not available in carousel view", ui.MessageUrgencyInfo)
		}
	case keymap.ActionPanelTreePrevSiblingDir:
		activePanel.JumpTreeSiblingDir(-1, viewportRows)
	case keymap.ActionPanelTreeNextSiblingDir:
		activePanel.JumpTreeSiblingDir(1, viewportRows)
	case keymap.ActionPanelToggleZoomActivePanel:
		a.toggleZoomActivePanelGuarded()
	case keymap.ActionPanelToggleSplitOrientation:
		a.togglePaneSplitOrientation()
	case keymap.ActionPanelReverseSort:
		activePanel.ToggleSortReverse(viewportRows)
		direction := "ascending"
		if activePanel.Sort.Reverse {
			direction = "descending"
		}
		a.setTransientMessage(fmt.Sprintf("Sort %s (%s)", direction, activePanel.Sort.Mode.String()), ui.MessageUrgencyInfo)
	case keymap.ActionPanelToggleHidden:
		if err := a.toggleHiddenGlobal(); err != nil {
			a.setErrorMessage("Toggle hidden failed", err)
		}
	default:
		return false
	}
	return true
}

// toggleTreeForPanel enables tree layout on target (seeding it from the current flat listing)
// if not already active, then toggles expand/collapse on the directory under target's cursor —
// so pressing Space on a not-yet-tree panel both switches to tree view and opens that row in
// one step. Returns true when blocked by CarouselMode (single entry point for both the direct
// keybinding dispatch and the scoped Left/Right menu item; see activateScopedPanelMenu).
func (a *App) toggleTreeForPanel(target *panel.State, viewportRows int) (blocked bool) {
	if target.ListLayout != panel.ListLayoutTree {
		if !target.SetListLayout(panel.ListLayoutTree, viewportRows) {
			return true
		}
	}
	if err := target.ToggleTreeExpand(viewportRows); err != nil {
		a.setErrorMessage("Toggle tree failed", err)
	}
	return false
}

// expandTreeForPanel enables tree layout on target if not already active, then expands the
// directory under the cursor. Mirrors toggleTreeForPanel's auto-enable step; see its doc comment.
func (a *App) expandTreeForPanel(target *panel.State, viewportRows int) (blocked bool) {
	if target.ListLayout != panel.ListLayoutTree {
		if !target.SetListLayout(panel.ListLayoutTree, viewportRows) {
			return true
		}
	}
	if err := target.ExpandTreeCursorRow(viewportRows); err != nil {
		a.setErrorMessage("Expand failed", err)
	}
	return false
}

// collapseTreeForPanel collapses the directory under the cursor. Unlike expandTreeForPanel, it
// never auto-enables tree mode: there is nothing to collapse if tree mode was never on.
func (a *App) collapseTreeForPanel(target *panel.State, viewportRows int) {
	if err := target.CollapseTreeCursorRow(viewportRows); err != nil {
		a.setErrorMessage("Collapse failed", err)
	}
}

// collapseAllTreeForPanel clears all expand state on target. Never auto-enables tree mode.
func (a *App) collapseAllTreeForPanel(target *panel.State, viewportRows int) {
	target.CollapseAllTree(viewportRows)
}

// expandAllTreeShallowForPanel enables tree layout on target if not already active, then expands
// directories by one level: under the cursor when it sits on an already-expanded directory,
// otherwise every depth-0 directory.
func (a *App) expandAllTreeShallowForPanel(target *panel.State, viewportRows int) (blocked bool) {
	if target.ListLayout != panel.ListLayoutTree {
		if !target.SetListLayout(panel.ListLayoutTree, viewportRows) {
			return true
		}
	}
	if err := target.ExpandAllTreeShallow(viewportRows); err != nil {
		a.setErrorMessage("Expand all failed", err)
	}
	return false
}

// toggleZoomActivePanelGuarded toggles active-panel zoom unless quick view/file preview is
// active, carousel mode is on, or the terminal size is below the configured zoom threshold.
func (a *App) toggleZoomActivePanelGuarded() {
	if a.previewCtrl.FilePreviewOpen() || a.model.QuickViewDisplayActive() {
		a.setTransientMessage("Zoom disabled while quick view or file view is active", ui.MessageUrgencyInfo)
		return
	}
	activePanel := a.activePanel()
	if activePanel.CarouselMode {
		a.setTransientMessage("Panel zoom is always on in carousel view", ui.MessageUrgencyInfo)
		return
	}
	tw, th := a.screen.Size()
	orientation := a.effectivePaneSplitOrientation()
	if orientation == ui.SplitVertical {
		if a.zoomActivePanelSuppressedByTerminalHeight(th) {
			a.setTransientMessage(fmt.Sprintf(
				"Panel zoom unavailable (terminal height ≥ %d)",
				a.config.UI.Zoom.DisabledAboveHeight,
			), ui.MessageUrgencyInfo)
			return
		}
	} else if a.zoomActivePanelSuppressedByTerminalWidth(tw) {
		a.setTransientMessage(fmt.Sprintf(
			"Panel zoom unavailable (terminal width ≥ %d)",
			a.config.UI.Zoom.DisabledAboveWidth,
		), ui.MessageUrgencyInfo)
		return
	}
	a.toggleRuntimeZoomActivePanel()
}
