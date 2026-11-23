package app

import (
	"github.com/paranoidi/paras-commander/internal/apphandler/meta"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// panelTab is a tab that is not shown: the whole panel.State of the slot, the meta columns it
// showed and, when a navigation was still loading as the tab was left, its target (resume), which
// the tab re-enters on return. The shown tab's state lives in panelInstance.state, never here.
type panelTab struct {
	state   panel.State
	columns meta.TabColumns
	resume  pathloc.Path
}

// noOtherTabMessage is the toast of closing a panel's only tab.
const noOtherTabMessage = "No other tab"

// tabSlotOfAction returns the slot (1-based) a panel.tab-N action selects.
func tabSlotOfAction(actionID string) (int, bool) {
	for i, id := range keymap.PanelTabActions {
		if id == actionID {
			return i + 1, true
		}
	}
	return 0, false
}

// selectTab switches panelID to slot: a used slot is shown, an empty one is opened as a copy of
// the current tab, the current slot is a no-op. Tabs exist in the browser only.
func (a *App) selectTab(panelID, slot int) {
	inst := a.panelInst(panelID)
	if inst == nil || !ui.IsBrowserPanel(panelID) || a.model.ViewMode != ui.ViewBrowser ||
		slot < 1 || slot > config.TabSlots || slot == inst.activeTab {
		return
	}
	next := inst.tabs[slot-1]
	inst.tabs[slot-1] = nil
	a.swapTab(panelID, slot, next, true)
}

// closeTab closes panelID's active tab and shows the nearest lower used slot, else the next
// higher one. Slots are not renumbered.
func (a *App) closeTab(panelID int) {
	inst := a.panelInst(panelID)
	if inst == nil || !ui.IsBrowserPanel(panelID) || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	slot := 0
	for i := inst.activeTab - 2; i >= 0 && slot == 0; i-- {
		if inst.tabs[i] != nil {
			slot = i + 1
		}
	}
	for i := inst.activeTab; i < config.TabSlots && slot == 0; i++ {
		if inst.tabs[i] != nil {
			slot = i + 1
		}
	}
	if slot == 0 {
		a.setTransientMessage(noOtherTabMessage, ui.MessageUrgencyInfo)
		return
	}
	next := inst.tabs[slot-1]
	inst.tabs[slot-1] = nil
	a.swapTab(panelID, slot, next, false)
}

// swapTab is the one entry of every tab change (select, close, mouse): it leaves the live tab,
// keeping it in its slot when keep is set, and shows next in slot; a nil next is a new tab, a copy
// of the tab being left.
//
// Panel IDs 0 and 1 are assumed everywhere to be &model.Primary / &model.Secondary, and the
// closures wired on panel.State resolve the panel by ID, so the swap replaces the State value in
// place instead of re-pointing anything.
func (a *App) swapTab(panelID, slot int, next *panelTab, keep bool) {
	inst := a.panelInst(panelID)
	live := inst.state
	prevSlot := inst.activeTab

	// Leave. The pending load is rolled back while the state is still the live one: its Rollback
	// closure captures the State pointer, which only means this tab until the value is replaced.
	resume := a.leaveTab(panelID)
	live.Park()
	left := &panelTab{state: *live, columns: a.metaCtrl.TabColumns(panelID), resume: resume}
	if next == nil {
		next = &panelTab{state: left.state.Duplicate(), columns: left.columns}
	}
	a.supersedeTabPanel(panelID, inst)

	// Enter. Settings that belong to the panel, not to the tab, follow the live state (hidden-file
	// visibility is applied by the load below).
	entered := next.state
	entered.ScrollMode, entered.ScrollEdgeMargin = left.state.ScrollMode, left.state.ScrollEdgeMargin
	entered.DiskUsageIdleSortEligible = left.state.DiskUsageIdleSortEligible
	entered.Strip.Tree = left.state.Strip.Tree
	entered.SelectionStashPaths = left.state.SelectionStashPaths
	entered.SelectionStashStripCursorPath = left.state.SelectionStashStripCursorPath
	entered.BumpGenerationsPast(&left.state)
	if keep {
		inst.tabs[prevSlot-1] = left
	}
	*live = entered
	inst.activeTab = slot
	a.syncTabBar(panelID)
	a.metaCtrl.RestoreTabColumns(panelID, next.columns)
	a.ensurePanelsVisible() // the tab row may have appeared or gone

	// Old rows stay on screen until the load lands (the listing is stale by however long the tab
	// was hidden anyway); it also restarts the tree loads the switch cancelled.
	rows := a.panelViewportRows(panelID)
	var err error
	shown := left.state.ShowHidden // hidden-file visibility is global, not per tab
	switch {
	case !next.resume.IsZero():
		live.ShowHidden = shown
		err = live.NavigateToPath(next.resume, "", rows)
	case live.ShowHidden != shown:
		// A visibility change reloads like the toggle does, so files it reveals are not marked new.
		err = live.SetShowHidden(shown, rows)
	default:
		err = live.Refresh(rows)
	}
	if err != nil {
		a.setErrorMessage("List failed", err)
	}
	live.RescheduleGitStatusIfNeeded()
}

// leaveTab undoes what only the shown tab can hold: the quick filters, a drag, the hover and
// navigation holds, and a navigation still loading (rolled back; the returned resume is its
// target, zero when the load was no navigation).
func (a *App) leaveTab(panelID int) (resume pathloc.Path) {
	inst := a.panelInst(panelID)
	pan := inst.state
	if pan.Filter.Active || pan.Filter.Editing {
		pan.CancelFilter(a.panelViewportRows(panelID))
	}
	if pan.Strip.Filter.Active || pan.Strip.Filter.Editing {
		pan.CancelStripFilter(a.selectedStripViewportRows(panelID))
	}
	if a.model.ActivePanel == panelID {
		a.model.ActiveSubFocus = ui.SubFocusFileList
	}
	a.cancelPanelDrag()
	a.clearPanelTitleHover()
	a.clearPanelSyncFollowNavCoalesce()
	a.previewCtrl.ClearCarouselPreviewNavCoalesce()
	a.previewCtrl.CloseCarouselFilePreview()
	if req := inst.pendingReq; req != nil && pan.ListingPending {
		if req.Rollback != nil {
			req.Rollback()
		}
		if req.HistoryVisit != 0 {
			pan.RevertHistoryVisit(req.HistoryPath, req.HistoryVisit)
		}
		if !req.Loc.Equal(pan.Path) {
			resume = req.Loc
		}
	}
	inst.pendingReq = nil
	return resume
}

// supersedeTabPanel drops every result still on its way for the tab being left, so none lands in
// the tab entered: listings and git status (generations), carousel side columns, drift passes,
// idle re-sorts, the selection-size scan and the working indicator. Results guarded by the listing
// epoch or the selection generation are dropped by swapTab raising both, tree-child results by the cancelled
// context Park leaves behind.
func (a *App) supersedeTabPanel(panelID int, inst *panelInstance) {
	inst.supersede()
	inst.carouselParentSnapshotGen.Add(1)
	inst.carouselChildSnapshotGen.Add(1)
	a.clearCarouselPaintDefer(panelID)
	a.idleSortInvalidate(a.diskIdleSortSpec(), panelID)
	a.idleSortInvalidate(a.metaIdleSortSpec(), panelID)
	inst.selectionSizeScanFP, inst.selectionSizeScanGen, inst.selectionSizeScanPath = "", 0, ""
	inst.selectionSizeScanDebounce.Invalidate()
	a.invalidateDirLoadingIndicator(panelID)
}

// syncTabBar publishes panelID's tab slots to the model: the active slot and the directory of
// every other used one (the target it will re-enter while a navigation was pending). Only this
// writes ui.Model.Tabs.
func (a *App) syncTabBar(panelID int) {
	inst := a.panelInst(panelID)
	bar := ui.TabBar{Active: inst.activeTab}
	for i, t := range inst.tabs {
		if t == nil {
			continue
		}
		bar.Paths[i] = t.state.Path
		if !t.resume.IsZero() {
			bar.Paths[i] = t.resume
		}
	}
	a.model.Tabs[panelID] = bar
}
