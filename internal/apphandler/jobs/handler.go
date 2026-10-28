package jobs

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/oplog"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func New(d Deps) *Handler {
	return &Handler{
		host:     d.Host,
		screen:   d.Screen,
		model:    d.Model,
		state:    d.State,
		config:   d.Config,
		keys:     d.Keys,
		keysJobs: d.KeysJobs,
	}
}

func (h *Handler) ToggleJobsView() {
	if h.model.ViewMode == ui.ViewJobs {
		h.closeJobsView()
		return
	}
	h.OpenJobsView()
}

// OpenJobsView switches to the jobs screen.
func (h *Handler) OpenJobsView() {
	h.model.ViewMode = ui.ViewJobs
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.model.MenuDefinitions = menu.JobsDefinitions(h.keys, h.keysJobs)
	h.model.Menu.ActiveMenu = menu.DefaultIndexJobs()
	h.model.JobsView = ui.JobsViewState{}
	if h.listStale {
		h.SyncJobsList()
	} else if len(h.model.JobsList) == 0 {
		h.SyncJobsList()
	}
	h.focusJobsViewOnFirstPendingBlocker()
}

func (h *Handler) focusJobsViewOnFirstPendingBlocker() {
	if idx := ui.FirstJobEntryWaitingDecisionIndex(h.model.JobsList); idx >= 0 {
		h.model.JobsView.Selected, h.model.JobsView.SelectedSub = idx, 0
		h.model.JobsView.FocusPane = 1
		h.model.JobsView.ConflictButtonFocus = 0
		h.model.JobsView.DetailScroll = 0
		h.model.JobsView.OpsScroll = 0
	}
	h.ensureJobsViewSelectionVisible()
}

func (h *Handler) closeJobsView() {
	h.model.ViewMode = ui.ViewBrowser
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.model.MenuDefinitions = menu.BrowserDefinitions(h.keys, h.host.DevMode())
	h.model.Menu.ActiveMenu = menu.DefaultIndex()
	h.model.JobsView = ui.JobsViewState{}
	h.model.JobsResultFilterDialog = dialog.JobsResultFilterDialogState{}
	h.stopOpsWork()
}

func (h *Handler) applyJobsRetention() {
	h.state.ApplyRetention(jobs.RetentionPolicy{
		ShowFinished: h.config.Jobs.ShowFinished,
		KeepFinished: h.config.Jobs.KeepFinished,
	})
}

// SyncJobsList refreshes the jobs list model from job state.
func (h *Handler) SyncJobsList() {
	h.applyJobsRetention()
	all := h.state.AllJobs()
	now := time.Now()
	queueETAs := jobs.ComputeQueueETAs(all, now)
	h.model.JobsList = ui.JobEntriesFromJobs(all, h.config.Jobs.ThroughputChartEnabled, queueETAs, h.model.JobsView.Expanded)
	h.pruneExpandedJobs()
	h.listStale = false
	h.listVersion++
}

func (h *Handler) SyncJobPathMarks() {
	h.model.JobPathMarks = ui.JobPathMarksFromJobs(h.state.AllJobs())
	h.pathMarksVersion++
}

func (h *Handler) applyNewFileMarksForCompletedJob(job *jobs.Job) {
	ui.ApplyNewFileMarksFromJob(h.host.PrimaryPanel(), job)
	ui.ApplyNewFileMarksFromJob(h.host.SecondaryPanel(), job)
}

// stashDanglingSourcesForCompletedJob queues job.Sources for the dangling-dirs check
// (see promptDanglingDirsIfAny) when the just-completed job was enqueued with
// PromptDanglingDirs. No filesystem I/O here — this runs on the event-batch path.
func (h *Handler) stashDanglingSourcesForCompletedJob(job *jobs.Job) {
	if job.PromptDanglingDirs {
		h.pendingDanglingSources = append(h.pendingDanglingSources, job.Sources...)
	}
}

// promptDanglingDirsIfAny checks the sources stashed by completed jobs for directories
// left empty and, if any are found, asks the host to open the delete-confirm prompt.
// Best-effort: filesystem errors are swallowed (skip the prompt) since this cleanup is
// a convenience, not part of the job itself. Must only run from ApplyRefreshes.
func (h *Handler) promptDanglingDirsIfAny() {
	if len(h.pendingDanglingSources) == 0 {
		return
	}
	sources := h.pendingDanglingSources
	h.pendingDanglingSources = nil
	dirs, err := ops.DanglingDirsAfter(context.Background(), sources)
	if err != nil || len(dirs) == 0 {
		return
	}
	paths := make([]string, len(dirs))
	for i, d := range dirs {
		paths[i] = d.String()
	}
	h.host.PromptDanglingDirDelete(paths)
}

func (h *Handler) ensureJobsViewSelectionVisible() {
	h.scrollToSelection(h.model.JobsView.JobsListRows(h.model.JobsList))
}

// scrollToSelection clamps the Queue cursor and scroll over rows, the current JobsListRows
// (callers that already built them pass them on instead of rebuilding).
func (h *Handler) scrollToSelection(rows []ui.JobsListRow) {
	width, height := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(width, height)
	visible := 0
	if !layout.TooSmall {
		visible = ui.PanelListRows(h.jobsViewRects(layout).List)
	}
	h.model.JobsView.EnsureSelectionVisible(rows, visible)
}

// pruneExpandedJobs forgets expanded jobs that left the list or no longer have sub-rows, so
// the expanded set stays bounded by the list.
func (h *Handler) pruneExpandedJobs() {
	exp := h.model.JobsView.Expanded
	if len(exp) == 0 {
		return
	}
	keep := make(map[string]bool, len(exp))
	for _, e := range h.model.JobsList {
		if h.model.JobsView.JobExpanded(e) {
			keep[e.ID] = true
		}
	}
	h.model.JobsView.Expanded = keep
}

// setJobExpanded expands or collapses the selected job's source sub-rows (list pane only).
// Collapsing from a sub-row moves the cursor to its job row.
func (h *Handler) setJobExpanded(expand bool) {
	st := &h.model.JobsView
	if st.FocusPane != 0 {
		return
	}
	sel := h.selectedJobEntry()
	if !ui.JobEntryHasSources(sel) || st.Expanded[sel.ID] == expand {
		return
	}
	if expand {
		if st.Expanded == nil {
			st.Expanded = map[string]bool{}
		}
		st.Expanded[sel.ID] = true
		h.SyncJobsList() // collapsed entries carry no per-source data: fetch it now
	} else {
		delete(st.Expanded, sel.ID)
		h.setSelection(st.JobsListRows(h.model.JobsList), ui.JobsListRow{Job: st.Selected, Source: oplog.NoSource})
		return
	}
	h.ensureJobsViewSelectionVisible()
}

// TryDispatch handles jobs.* actions. It returns true if actionID is a jobs-domain
// action (consumed here, including deliberate no-ops outside the jobs view).
func (h *Handler) TryDispatch(actionID string) bool {
	switch actionID {
	case keymap.ActionJobsAnswerBlocker:
		// The raw-key path answers blockers pre-dispatch (input.go); this covers
		// menu/help activation. No-op when nothing is waiting for a decision.
		h.HandleAnswerBlockerKey()
		return true
	case keymap.ActionJobsOpen:
		h.ToggleJobsView()
		return true
	case keymap.ActionJobsClose:
		if h.model.ViewMode == ui.ViewJobs {
			h.closeJobsView()
		}
		return true
	case keymap.ActionJobsMaximizePane:
		if h.model.ViewMode != ui.ViewJobs {
			return true
		}
		if h.operationsFocused() {
			h.model.JobsView.Maximized = true
			return true
		}
		return false // let Enter reach the blocker pane's buttons
	case keymap.ActionJobsClearFinished:
		if h.model.ViewMode != ui.ViewJobs {
			return true
		}
		h.clearFinishedJobs()
		return true
	case keymap.ActionJobsQueueUp, keymap.ActionJobsQueueDown,
		keymap.ActionJobsCancel,
		keymap.ActionJobsPause, keymap.ActionJobsResume:
		if h.model.ViewMode != ui.ViewJobs {
			return true
		}
		switch actionID {
		case keymap.ActionJobsQueueUp:
			h.moveJobInQueue(-1)
		case keymap.ActionJobsQueueDown:
			h.moveJobInQueue(1)
		case keymap.ActionJobsCancel:
			h.cancelSelectedJob()
		case keymap.ActionJobsPause:
			h.pauseSelectedQueuedJob()
		case keymap.ActionJobsResume:
			h.resumeOrRetrySelectedJob()
		}
		return true
	case keymap.ActionJobsRateLimitIncrease, keymap.ActionJobsRateLimitDecrease:
		if h.model.ViewMode == ui.ViewJobs && h.DetailFocused() {
			return false // let the key reach the Details pane's tickrate keys
		}
		if h.model.ViewMode != ui.ViewJobs || h.model.JobsView.FocusPane != 0 {
			return true
		}
		h.adjustRateLimit(actionID == keymap.ActionJobsRateLimitIncrease)
		return true
	case keymap.ActionJobsResultFilter:
		if h.model.ViewMode == ui.ViewJobs {
			h.OpenResultFilterDialog()
		}
		return true
	case keymap.ActionJobsResetFilter:
		if h.model.ViewMode == ui.ViewJobs {
			h.ResetOperationsFilter()
		}
		return true
	case keymap.ActionJobsExpand, keymap.ActionJobsCollapse:
		if h.model.ViewMode == ui.ViewJobs {
			h.setJobExpanded(actionID == keymap.ActionJobsExpand)
		}
		return true
	case keymap.ActionJobsUndo:
		if h.model.ViewMode == ui.ViewJobs {
			h.undoSelectedJob()
		}
		return true
	case keymap.ActionJobsRateLimitClear:
		if h.model.ViewMode != ui.ViewJobs {
			return true
		}
		h.state.SetTransferRateLimit(0)
		return true
	case keymap.ActionJobsToggleOperationsLayout:
		if h.model.ViewMode == ui.ViewJobs {
			h.toggleOperationsLayout()
		}
		return true
	default:
		return false
	}
}

func (h *Handler) HandleJobsViewKey(event *tcell.EventKey) bool {
	if h.model.JobsResultFilterDialog.Open {
		h.HandleResultFilterDialogKey(event)
		return false
	}
	h.clampJobsFocusPane()
	// An open Operations fuzzy filter takes its editing keys before vi motion, leader letters
	// and the maximized pane's own keys.
	if h.operationsFocused() && h.handleOperationsFilterKey(event) {
		return false
	}
	if h.model.JobsView.Maximized {
		h.handleJobsMaximizedKey(event)
		return false
	}
	// Esc closes this screen only; it must not run the global quit action even if
	// the user binds quit to Esc in keybindings.toml.
	switch event.Key() {
	case tcell.KeyEsc:
		h.closeJobsView()
		return false
	case tcell.KeyTab:
		n := h.jobsViewFocusPaneCount()
		h.model.JobsView.FocusPane = (h.model.JobsView.FocusPane + 1) % n
		h.model.JobsView.ConflictButtonFocus = 0
		return false
	case tcell.KeyBacktab:
		n := h.jobsViewFocusPaneCount()
		h.model.JobsView.FocusPane = (h.model.JobsView.FocusPane + n - 1) % n
		h.model.JobsView.ConflictButtonFocus = 0
		return false
	}

	// vi-motion mode's hjkl-remap and direct-fire leader letters are skipped while the
	// blocker/conflict pane has focus, so a bare letter never fires a queue action while the
	// user is mid-decision on a blocker (Alt+letter blocker-decision shortcuts still work via
	// handleJobsConflictPaneKey below).
	blockerFocused := h.jobsViewConflictVisible() && h.model.JobsView.FocusPane == 1
	if h.model.ViMotionMode && !blockerFocused {
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
	if !blockerFocused && h.host.DispatchLeaderLetter(event) {
		return false
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if h.host.OpenMenuByShortcut(event.Rune()) {
			return false
		}
	}

	skipJobsClose := nextAction == keymap.ActionJobsClose &&
		event.Key() == tcell.KeyLeft &&
		h.jobsViewConflictVisible() &&
		h.model.JobsView.FocusPane == 1
	if nextAction != "" && !skipJobsClose && h.TryDispatch(nextAction) {
		return false
	}
	if nextAction != "" && h.host.TryDispatchAuxiliaryScreens(nextAction) {
		return false
	}
	if nextAction == keymap.ActionPanelExternalBrowser {
		h.host.Dispatch(nextAction)
		return false
	}

	conflictVis := h.jobsViewConflictVisible()
	fp := h.model.JobsView.FocusPane

	if conflictVis && fp == 1 {
		return h.handleJobsConflictPaneKey(event)
	}
	if nextAction == "" && plainRune(event) && h.operationsFocused() {
		h.typeOperationsRune(event.Rune())
		return false
	}

	h.handleJobsPaneScrollKey(event, fp)
	return false
}

// handleJobsConflictPaneKey handles key input while the conflict/blocker pane (FocusPane 1
// when a conflict panel is visible) has focus: Alt+letter decision shortcuts and
// Left/Right/Up/Down/Tab/Backtab/Enter navigation between the blocker dialog's buttons.
func (h *Handler) handleJobsConflictPaneKey(event *tcell.EventKey) bool {
	sel := h.selectedJobEntry()
	maxB := ui.JobsBlockerMaxButtonIndex(sel)
	disk := sel.PendingBlocker != nil && sel.PendingBlocker.Kind == jobs.BlockerKindDiskSpace

	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if disk {
			switch event.Rune() {
			case 'r', 'R':
				h.submitJobsConflictDecision(jobs.DecisionRetry)
				return false
			case 'b', 'B':
				h.submitJobsConflictDecision(jobs.DecisionCancel)
				return false
			}
			return false
		}
		id, ok := ui.JobBlockerPanelMnemonic(sel, event.Rune())
		switch {
		case !ok:
		case id == dialog.ConflictAdvanced:
			h.openAdvancedFromPanel(sel)
		default:
			if d, ok := dialog.ConflictDecisionOf(id); ok {
				h.submitJobsConflictDecision(d)
			}
		}
		return false
	}
	switch event.Key() {
	case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown, tcell.KeyTab, tcell.KeyBacktab:
		if sel.PendingBlocker == nil {
			return false
		}
		newFocus, handled := ui.JobsBlockerPanelMoveFocus(*sel.PendingBlocker, h.model.JobsView.ConflictButtonFocus, event.Key())
		if handled {
			if newFocus > maxB {
				newFocus = maxB
			}
			h.model.JobsView.ConflictButtonFocus = newFocus
		}
	case tcell.KeyEnter:
		if ui.JobBlockerPanelIsAdvanced(sel, h.model.JobsView.ConflictButtonFocus) {
			h.openAdvancedFromPanel(sel)
			return false
		}
		d := ui.JobBlockerDecisionFromFocus(sel, h.model.JobsView.ConflictButtonFocus)
		h.submitJobsConflictDecision(d)
	}
	return false
}

// handleJobsPaneScrollKey handles Up/Down/PgUp/PgDn/Home/End for whichever pane currently has
// focus (list, Details or Operations). The conflict pane is handled by handleJobsConflictPaneKey.
func (h *Handler) handleJobsPaneScrollKey(event *tcell.EventKey, fp int) {
	order, rects := h.jobsPaneLayout()
	if fp < 0 || fp >= len(order) {
		return
	}
	pane := order[fp]
	if pane == ui.JobsPaneList {
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
		return
	}
	contentH := ui.JobsPanelContentRows(rects[pane])
	switch pane {
	case ui.JobsPaneDetail:
		if h.handleJobsDetailPaneKey(event) {
			return
		}
		scrollPane(event.Key(), &h.model.JobsView.DetailScroll, max(0, h.jobsDetailLineCountForSelection()-contentH))
	case ui.JobsPaneOperations:
		h.scrollOperations(event.Key(), contentH)
	}
}

// handleJobsDetailPaneKey adjusts the graph tickrate with + / - while Details has focus. It is a
// session-only change (not written to config).
func (h *Handler) handleJobsDetailPaneKey(event *tcell.EventKey) bool {
	if !plainRune(event) {
		return false
	}
	switch event.Rune() {
	case '+', '=':
		h.host.AdjustJobsThroughputChartColumnMS(-config.ThroughputChartColumnMSLiveStep)
	case '-', '_':
		h.host.AdjustJobsThroughputChartColumnMS(config.ThroughputChartColumnMSLiveStep)
	default:
		return false
	}
	return true
}

// scrollPane applies Up/Down (one row), PgUp/PgDn (five rows) and Home/End (top/bottom) to
// *scroll, clamped to [0, maxScroll].
func scrollPane(key tcell.Key, scroll *int, maxScroll int) {
	switch key {
	case tcell.KeyHome:
		*scroll = 0
	case tcell.KeyEnd:
		*scroll = maxScroll
	case tcell.KeyUp:
		*scroll = max(0, *scroll-1)
	case tcell.KeyDown:
		*scroll = min(maxScroll, *scroll+1)
	case tcell.KeyPgUp:
		*scroll = max(0, *scroll-5)
	case tcell.KeyPgDn:
		*scroll = min(maxScroll, *scroll+5)
	}
}

// MoveSelection moves the jobs-list cursor by delta rows (jobs and expanded source sub-rows).
func (h *Handler) MoveSelection(delta int) {
	rows := h.model.JobsView.JobsListRows(h.model.JobsList)
	if len(rows) == 0 {
		return
	}
	row := min(max(0, h.model.JobsView.SelectedRow(rows)+delta), len(rows)-1)
	h.setSelection(rows, rows[row])
}

// SelectEdge moves the jobs-list cursor to the first (toEnd=false) or last (toEnd=true) row.
func (h *Handler) SelectEdge(toEnd bool) {
	rows := h.model.JobsView.JobsListRows(h.model.JobsList)
	if len(rows) == 0 {
		return
	}
	row := 0
	if toEnd {
		row = len(rows) - 1
	}
	h.setSelection(rows, rows[row])
}

// FocusListRow focuses the queue list and moves its cursor to row index row of
// JobsListRows (a mouse click).
func (h *Handler) FocusListRow(row int) {
	h.model.JobsView.FocusPane = 0
	rows := h.model.JobsView.JobsListRows(h.model.JobsList)
	if row < 0 || row >= len(rows) {
		return
	}
	h.setSelection(rows, rows[row])
}

// setSelection moves the Queue cursor to r, one of rows (the current JobsListRows).
func (h *Handler) setSelection(rows []ui.JobsListRow, r ui.JobsListRow) {
	st := &h.model.JobsView
	beforeSel, beforeSub := st.Selected, st.SelectedSub
	st.SelectRow(r)
	if beforeSel != st.Selected {
		st.DetailScroll = 0
		st.ConflictButtonFocus = 0
	}
	if beforeSel != st.Selected || beforeSub != st.SelectedSub {
		st.OpsScroll = 0
	}
	h.scrollToSelection(rows)
}

func (h *Handler) jobsViewConflictVisible() bool {
	sel := h.selectedJobEntry()
	return ui.JobEntryShowsConflictPanel(sel)
}

func (h *Handler) selectedJobEntry() ui.JobEntry {
	if h.model.JobsView.Selected >= 0 && h.model.JobsView.Selected < len(h.model.JobsList) {
		return h.model.JobsList[h.model.JobsView.Selected]
	}
	return ui.JobEntry{}
}

func (h *Handler) submitJobsConflictDecision(d jobs.ConflictDecision) {
	id := h.selectedJobEntry().ID
	if id == "" {
		return
	}
	h.state.SubmitBlockerDecision(id, d)
	h.model.JobsView.ConflictButtonFocus = 0
}

func (h *Handler) clampJobsFocusPane() {
	maxPane := h.jobsViewFocusPaneCount() - 1
	if h.model.JobsView.FocusPane > maxPane {
		h.model.JobsView.FocusPane = maxPane
	}
	if h.model.JobsView.Maximized && !h.operationsFocused() {
		h.model.JobsView.Maximized = false
	}
}

// handleJobsMaximizedKey handles keys while the Operations pane is maximized: Esc, Enter and
// Left restore the layout, printable runes type into the fuzzy filter, Up/Down/PgUp/PgDn/Home/End
// scroll by row, the rest is swallowed.
func (h *Handler) handleJobsMaximizedKey(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyEnter, tcell.KeyLeft:
		h.model.JobsView.Maximized = false
		return
	case tcell.KeyRune:
		if plainRune(event) {
			h.typeOperationsRune(event.Rune())
		}
		return
	}
	width, height := h.screen.Size()
	h.scrollOperations(event.Key(), ui.JobsPanelContentRows(ui.JobsMaximizedRect(h.host.LayoutForTerminalSize(width, height))))
}

// scrollOperations scrolls the Operations pane (contentRows rows tall) by row within the
// current view's row count.
func (h *Handler) scrollOperations(key tcell.Key, contentRows int) {
	h.refreshOpsWindow(true)
	win := h.model.JobsOpsWindow
	scrollPane(key, &h.model.JobsView.OpsScroll, ui.JobsOpsMaxScroll(win.Total, win.RecordRows(contentRows)))
}

func (h *Handler) jobsDetailLineCountForSelection() int {
	var sel ui.JobEntry
	if h.model.JobsView.Selected >= 0 && h.model.JobsView.Selected < len(h.model.JobsList) {
		sel = h.model.JobsList[h.model.JobsView.Selected]
	}
	return ui.JobDetailLineCount(sel, time.Now(), h.config.Jobs.ThroughputChartEnabled)
}

// jobsPaneLayout returns the focusable panes of the jobs view for the selected job and the
// current terminal size (ui.JobsPaneOrder), with the frame of each scrollable pane. A terminal
// too small for the jobs layout yields only list and Details with empty frames.
func (h *Handler) jobsPaneLayout() ([]ui.JobsPane, map[ui.JobsPane]ui.Rect) {
	width, height := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(width, height)
	if layout.TooSmall {
		return ui.JobsPaneOrder(false, false), nil
	}
	r := h.jobsViewRects(layout)
	rects := map[ui.JobsPane]ui.Rect{
		ui.JobsPaneDetail:     r.Detail,
		ui.JobsPaneOperations: r.Operations,
	}
	return ui.JobsPaneOrder(ui.JobEntryShowsConflictPanel(h.selectedJobEntry()), r.Operations.Height > 0), rects
}

// jobsViewRects returns the jobs view pane frames for the selected job (ui.JobsViewLayout).
func (h *Handler) jobsViewRects(layout ui.Layout) ui.JobsViewRects {
	return ui.JobsViewLayout(layout, h.model.JobsOpsBottom, h.selectedJobEntry(), time.Now(), h.config.Jobs.ThroughputChartEnabled)
}

// toggleOperationsLayout moves the Operations pane between the right column and the full-width
// bottom for this session, then refits the panes to their new frames.
func (h *Handler) toggleOperationsLayout() {
	h.model.JobsOpsBottom = !h.model.JobsOpsBottom
	h.clampJobsFocusPane()
	h.scrollToSelection(h.model.JobsView.JobsListRows(h.model.JobsList))
	h.refreshOpsWindow(true)
	win := h.model.JobsOpsWindow
	h.model.JobsView.OpsScroll = min(h.model.JobsView.OpsScroll, ui.JobsOpsMaxScroll(win.Total, win.RecordRows(h.opsContentRows())))
	label := "side"
	if h.model.JobsOpsBottom {
		label = "bottom"
	}
	h.host.SetTransientMessage("Operations: "+label, ui.MessageUrgencyInfo)
}

func (h *Handler) jobsViewFocusPaneCount() int {
	order, _ := h.jobsPaneLayout()
	return len(order)
}

const rateLimitStepBPS int64 = 10 * 1024 * 1024        // 10MB/s (binary MB, matches jobs.FormatThroughput)
const rateLimitNoJobStartBPS int64 = 100 * 1024 * 1024 // 100MB/s fallback when nothing is transferring

// adjustRateLimit steps the global transfer rate limit by one 10MB/s increment.
// Enabling it from Unlimited (0), via either + or -, starts from the active job's
// current speed (rounded to the nearest step), or rateLimitNoJobStartBPS if no job
// is running. Decreasing floors at one step; only ActionJobsRateLimitClear returns
// to Unlimited.
func (h *Handler) adjustRateLimit(increase bool) {
	cur := h.state.TransferRateLimit()
	switch {
	case cur == 0:
		cur = h.startingRateLimit()
	case increase:
		cur += rateLimitStepBPS
	default:
		cur -= rateLimitStepBPS
		if cur < rateLimitStepBPS {
			cur = rateLimitStepBPS
		}
	}
	h.state.SetTransferRateLimit(cur)
}

func (h *Handler) startingRateLimit() int64 {
	active := h.state.ActiveJob()
	if active == nil {
		return rateLimitNoJobStartBPS
	}
	bps := jobs.EffectiveDisplayThroughputBPS(active.Status, active.StartedAt, time.Now(), active.DoneBytes, active.DisplaySpeedBPS)
	if bps <= 0 {
		return rateLimitNoJobStartBPS
	}
	steps := max(int64(math.Round(bps/float64(rateLimitStepBPS))), 1)
	return steps * rateLimitStepBPS
}

func (h *Handler) moveJobInQueue(delta int) {
	q := h.jobQueue()
	qi, ok := h.selectedQueueIndex()
	if !ok {
		return
	}
	nj := qi + delta
	if nj < 0 || nj >= q.Len() || qi == nj {
		return
	}
	if !q.SwapQueued(qi, nj) {
		return
	}
	h.SyncJobsList()
	// Keep selection on the same job row (swap moves the job).
	h.ensureJobsViewSelectionVisible()
}

func (h *Handler) selectedQueueIndex() (qi int, ok bool) {
	row := h.model.JobsView.Selected
	if row < 0 || row >= len(h.model.JobsList) {
		return 0, false
	}
	id := h.model.JobsList[row].ID
	q := h.jobQueue()
	for i, j := range q.AllJobs() {
		if j != nil && j.ID == id {
			if j.Status.IsFinished() {
				return 0, false
			}
			return i, true
		}
	}
	return 0, false
}

func (h *Handler) cancelSelectedJob() {
	n := len(h.model.JobsList)
	if n == 0 {
		return
	}
	sel := h.model.JobsView.Selected
	if sel < 0 || sel >= n {
		return
	}
	id := h.model.JobsList[sel].ID
	if !h.state.CancelJob(id) {
		h.host.SetTransientMessage("Could not cancel job", ui.MessageUrgencyWarn)
		return
	}
	h.SyncJobsList()
	h.SyncJobPathMarks()
	h.ensureJobsViewSelectionVisible()
}

func (h *Handler) pauseSelectedQueuedJob() {
	n := len(h.model.JobsList)
	if n == 0 {
		return
	}
	sel := h.model.JobsView.Selected
	if sel < 0 || sel >= n {
		return
	}
	if h.model.JobsList[sel].Status != string(jobs.StatusQueued) {
		h.host.SetTransientMessage("Only queued jobs can be paused", ui.MessageUrgencyWarn)
		return
	}
	id := h.model.JobsList[sel].ID
	if !h.state.PauseQueuedJob(id) {
		h.host.SetTransientMessage("Could not pause job", ui.MessageUrgencyWarn)
		return
	}
	h.SyncJobsList()
	h.SyncJobPathMarks()
	h.ensureJobsViewSelectionVisible()
}

// resumeOrRetrySelectedJob resumes a paused job or retries a failed or canceled one, whichever
// the selected job is — the same ActionJobsResume key/menu entry doubles as Retry on a failed job.
func (h *Handler) resumeOrRetrySelectedJob() {
	n := len(h.model.JobsList)
	if n == 0 {
		return
	}
	sel := h.model.JobsView.Selected
	if sel < 0 || sel >= n {
		return
	}
	id := h.model.JobsList[sel].ID
	switch jobs.Status(h.model.JobsList[sel].Status) {
	case jobs.StatusPaused:
		if !h.state.ResumeJob(id) {
			h.host.SetTransientMessage("Could not resume job", ui.MessageUrgencyWarn)
			return
		}
	case jobs.StatusFailed, jobs.StatusCanceled:
		if !h.state.RetryJob(id) {
			h.host.SetTransientMessage("Could not retry job", ui.MessageUrgencyWarn)
			return
		}
	default:
		h.host.SetTransientMessage("Selected job is not paused, failed or canceled", ui.MessageUrgencyWarn)
		return
	}
	h.SyncJobsList()
	h.SyncJobPathMarks()
	h.ensureJobsViewSelectionVisible()
}

// undoSelectedJob asks to confirm the undo of the selected job, or says why it cannot be undone.
func (h *Handler) undoSelectedJob() {
	id := h.selectedJobEntry().ID
	j := h.state.Job(id)
	if j == nil {
		return
	}
	if !j.Undoable() {
		reason := j.UndoLabel()
		if reason == "" || reason == "available" {
			reason = "not available for this job"
		}
		h.host.SetTransientMessage("Cannot undo: "+reason, ui.MessageUrgencyWarn)
		return
	}
	var created, moved int
	for _, s := range j.Undo {
		if s.Kind == ops.UndoMoveBack {
			moved++
		} else {
			created++
		}
	}
	line2 := fmt.Sprintf("Remove %d created %s.", created, jobbridge.Plural(created, "item", "items"))
	if moved > 0 {
		line2 = fmt.Sprintf("Move %d %s back.", moved, jobbridge.Plural(moved, "item", "items"))
	}
	h.model.UndoConfirm = dialog.UndoConfirmState{
		Open:  true,
		Focus: dialog.UndoNo,
		JobID: id,
		Line1: fmt.Sprintf("Undo %s: %d %s", j.Type, len(j.Undo), jobbridge.Plural(len(j.Undo), "step", "steps")),
		Line2: line2,
	}
}

// ConfirmUndo queues the undo job for job id after the user confirmed.
func (h *Handler) ConfirmUndo(id string) {
	if _, ok := h.state.EnqueueUndo(id); !ok {
		h.host.SetTransientMessage("Could not undo job", ui.MessageUrgencyWarn)
		return
	}
	h.SyncJobsList()
	h.SyncJobPathMarks()
	h.ensureJobsViewSelectionVisible()
}

// RecordRename records a finished rename or mass rename (one step per renamed item) as a
// completed job so it shows in the jobs view and can be undone.
func (h *Handler) RecordRename(steps []ops.UndoStep) {
	if len(steps) == 0 {
		return
	}
	sources := make([]pathloc.Path, len(steps))
	for i := range steps {
		sources[i] = steps[i].From
		steps[i].Source = int32(i) // each renamed item is its own source
	}
	h.state.AddFinishedJob(&jobs.Job{
		ID:          jobs.NewJobID(),
		Type:        jobs.TypeRename,
		Sources:     sources,
		Destination: steps[0].Path.Parent(),
		DestIsDir:   true,
		TotalFiles:  len(steps),
		DoneFiles:   len(steps),
		Undo:        steps,
	})
	h.SyncJobsList()
}

// SelectedJobStatus returns the status of the currently selected job in the jobs view, or ""
// when nothing is selected. Single source of truth for the Resume/Retry footer hint and
// leader-menu label.
func (h *Handler) SelectedJobStatus() jobs.Status {
	sel := h.model.JobsView.Selected
	if sel < 0 || sel >= len(h.model.JobsList) {
		return ""
	}
	return jobs.Status(h.model.JobsList[sel].Status)
}

func (h *Handler) clearFinishedJobs() {
	h.jobQueue().ClearFinished()
	h.state.ClearFinishedArchive()
	h.SyncJobsList()
	h.SyncJobPathMarks()
	if h.model.ViewMode == ui.ViewJobs {
		h.ensureJobsViewSelectionVisible()
	}
}

func (h *Handler) jobQueue() *jobs.Queue {
	return h.state.Queue()
}

func (h *Handler) progressUIWakeDebounce() time.Duration {
	return time.Duration(h.config.Jobs.ProgressUIWakeDebounceMS) * time.Millisecond
}

// onJobEmitted wakes PollEvent so the main loop can drain jobs.Events().
// Progress emits use [jobs].progress_ui_wake_debounce_ms so the menu-bar strip can update
// without blocking the event loop; other event types wake immediately.
func (h *Handler) OnJobEmitted(ev jobs.Event) {
	if ev.Type == jobs.EventProgress {
		h.armJobsWakeDebounced()
		return
	}
	h.postJobsWakeImmediate()
}

func (h *Handler) armJobsWakeDebounced() {
	h.wakeMu.Lock()
	defer h.wakeMu.Unlock()
	if h.wakeTimer != nil {
		if !h.wakeTimer.Stop() {
			select {
			case <-h.wakeTimer.C:
			default:
			}
		}
	}
	h.wakeTimer = time.AfterFunc(h.progressUIWakeDebounce(), func() {
		h.wakeMu.Lock()
		h.wakeTimer = nil
		h.wakeMu.Unlock()
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(WakePayload{}))
	})
}

func (h *Handler) postJobsWakeImmediate() {
	h.stopJobsWakeTimer()
	_ = h.screen.PostEvent(tcell.NewEventInterrupt(WakePayload{}))
}

func (h *Handler) stopJobsWakeTimer() {
	h.wakeMu.Lock()
	defer h.wakeMu.Unlock()
	if h.wakeTimer == nil {
		return
	}
	if !h.wakeTimer.Stop() {
		select {
		case <-h.wakeTimer.C:
		default:
		}
	}
	h.wakeTimer = nil
}

func (h *Handler) drainJobEventChannel() []jobs.Event {
	var batch []jobs.Event
	for {
		select {
		case ev := <-h.state.Events():
			batch = append(batch, ev)
		default:
			return jobbridge.CoalesceEventBatch(batch)
		}
	}
}

// MaxProgressEventsDiscardPerDrain bounds how many progress events the browser discards per
// key/drain call so a full channel cannot starve navigation and other PollEvent work.
// MaxProgressEventsDiscardPerDrain bounds progress events discarded per browser key/drain.
const MaxProgressEventsDiscardPerDrain = 64

// drainDiscardProgressEvents pulls progress events from the channel during key handling so the UI
// stays responsive; progress is applied in coalesced batches (see MaxProgressEventsDiscardPerDrain).
// Non-progress events are applied immediately after flushing any pending progress.
func (h *Handler) DrainDiscardProgressEvents() {
	if h.model.ViewMode == ui.ViewJobs {
		return
	}
	var pending []jobs.Event
	var progressBuf []jobs.Event
	for {
		select {
		case ev := <-h.state.Events():
			if ev.Type == jobs.EventProgress {
				progressBuf = append(progressBuf, ev)
				if len(progressBuf) >= MaxProgressEventsDiscardPerDrain {
					h.applyJobEventBatch(jobbridge.CoalesceEventBatch(progressBuf))
					if len(pending) > 0 {
						h.applyJobEventBatch(jobbridge.CoalesceEventBatch(pending))
					}
					return
				}
				continue
			}
			if len(progressBuf) > 0 {
				h.applyJobEventBatch(jobbridge.CoalesceEventBatch(progressBuf))
				progressBuf = progressBuf[:0]
			}
			pending = append(pending, ev)
		default:
			if len(progressBuf) > 0 {
				h.applyJobEventBatch(jobbridge.CoalesceEventBatch(progressBuf))
			}
			if len(pending) > 0 {
				h.applyJobEventBatch(jobbridge.CoalesceEventBatch(pending))
			}
			return
		}
	}
}

// applyJobEventBatch merges worker events into UI model state and sets refresh/repaint flags.
func (h *Handler) applyJobEventBatch(batch []jobs.Event) {
	if len(batch) == 0 {
		return
	}
	viewJobs := h.model.ViewMode == ui.ViewJobs
	flags := h.scanBatchFlags(batch)
	if flags.Terminal {
		h.refreshTerminal = true
		h.refreshProgress = false
		h.refreshDestListing = false
	} else {
		if flags.Progress && !h.refreshTerminal {
			h.refreshProgress = true
		}
		// MarkUpdate (started/resumed/…) and progress both need a dest-panel readdir so
		// newly written names appear in the file list with job icons while the job runs. A
		// moved active source (ActiveSource) always comes with progress.
		if (flags.Progress || flags.MarkUpdate) && !h.refreshTerminal {
			h.refreshDestListing = true
		}
	}
	if flags.MarkUpdate {
		h.SyncJobPathMarks()
	}
	if viewJobs {
		h.SyncJobsList()
		h.ensureJobsViewSelectionVisible()
	} else if flags.Terminal || flags.Blocker || flags.MarkUpdate || flags.Progress {
		h.listStale = true
	}
	h.affectVisible = viewJobs || h.batchAffectsVisible(flags)
	h.lastBatchMenuBarStripOnly = !viewJobs && h.batchIsMenuBarStripOnly(flags)
}

// batchFlags summarizes what categories of event a batch contained, computed by
// scanBatchFlags and consumed by applyJobEventBatch's post-loop bookkeeping.
type batchFlags struct {
	Terminal   bool
	Progress   bool
	Blocker    bool
	MarkUpdate bool
	// ActiveSource is set when a multi-source job's executor moved to another source: its
	// path marks were updated in place (followActiveSource), without a SyncJobPathMarks rebuild.
	ActiveSource bool
}

// scanBatchFlags applies each event in the batch to UI model state (jobs.State, activity log,
// job message) and reports which categories of event it contained, moved out of
// applyJobEventBatch's accumulation loop.
func (h *Handler) scanBatchFlags(batch []jobs.Event) batchFlags {
	var f batchFlags
	var progressed []string
	for _, ev := range batch {
		h.state.ApplyEvent(ev)
		h.updateJobMessage(ev)
		switch ev.Type {
		case jobs.EventCompleted:
			f.Terminal = true
			if job := h.state.Job(ev.JobID); job != nil {
				h.applyNewFileMarksForCompletedJob(job)
				h.stashDanglingSourcesForCompletedJob(job)
			}
		case jobs.EventFailed, jobs.EventCanceled:
			f.Terminal = true
		case jobs.EventProgress, jobs.EventVerifyProgress:
			f.Progress = true
			if !slices.Contains(progressed, ev.JobID) {
				progressed = append(progressed, ev.JobID)
			}
		case jobs.EventScanProgress:
			f.Progress = true
		case jobs.EventJobBlockerRequest:
			f.Blocker = true
		}
		if jobbridge.EventUpdatesMarks(ev.Type) {
			f.MarkUpdate = true
		}
	}
	// Once per job after the whole batch is applied: only its final active source matters.
	for _, id := range progressed {
		if h.followActiveSource(id) {
			f.ActiveSource = true
		}
	}
	return f
}

// followActiveSource moves job id's file-list marks to the source its executor works on now
// (O(1) per progress event, the path index is kept) and reports whether it moved.
func (h *Handler) followActiveSource(id string) bool {
	job := h.state.Job(id)
	if job == nil {
		return false
	}
	return ui.SetJobPathMarkSources(h.model.JobPathMarks, id, job.ActiveSource(), job.SourcePhases)
}

// batchAffectsVisible reports whether the batch requires an immediate visible-area repaint
// (independent of whether the Jobs view itself is open — the caller ORs in viewJobs), moved
// out of applyJobEventBatch's dense affectVisible boolean expression.
func (h *Handler) batchAffectsVisible(f batchFlags) bool {
	return f.Terminal || f.Blocker ||
		((f.MarkUpdate || f.ActiveSource) && (ui.PanelTouchedByJobs(h.model.Primary.PathString(), h.model.JobPathMarks) ||
			ui.PanelTouchedByJobs(h.model.Secondary.PathString(), h.model.JobPathMarks) ||
			h.state.JobsWaitingDecision() > 0)) ||
		(f.Progress && h.model.MenuBarLayoutReserved() && h.state.HasNonFinishedJob())
}

// batchIsMenuBarStripOnly reports whether the batch only needs the menu-bar activity strip
// repainted (progress with a live non-finished job, and nothing terminal/blocker/mark-related),
// moved out of applyJobEventBatch's dense lastBatchMenuBarStripOnly boolean expression.
func (h *Handler) batchIsMenuBarStripOnly(f batchFlags) bool {
	return f.Progress && !f.Terminal && !f.Blocker && !f.MarkUpdate && !f.ActiveSource &&
		h.model.MenuBarLayoutReserved() && h.state.HasNonFinishedJob()
}

// pollJobEvents drains pending worker events into UI model state.
// It returns whether any event was processed (caller should repaint when appropriate).
func (h *Handler) PollEvents() bool {
	h.affectVisible = false
	h.lastBatchMenuBarStripOnly = false
	batch := h.drainJobEventChannel()
	if len(batch) == 0 {
		return false
	}
	h.applyJobEventBatch(batch)
	return true
}

// applyJobRefreshes executes the filesystem I/O side-effects that were deferred
// by pollJobEvents(). It must only be called from the jobsWakePayload handler in
// Run() — never during key-event handling — so that statfs/readdir syscalls never
// block the UI event loop while the user is navigating or making selections.
// The return value is true when both panels were reloaded after a terminal job event.
func (h *Handler) ApplyRefreshes() bool {
	switch {
	case h.refreshTerminal:
		h.refreshTerminal = false
		h.refreshProgress = false
		h.refreshDestListing = false
		h.applyJobsRetention()
		h.SyncJobPathMarks()
		h.host.RefreshBothPanels()
		h.promptDanglingDirsIfAny()
		return true
	default:
		didProgress := h.refreshProgress
		reloadDest := didProgress || h.refreshDestListing
		h.refreshProgress = false
		h.refreshDestListing = false
		if didProgress && h.config.Jobs.FreeSpaceOnProgressWake {
			h.host.RequestBothPanelsVolumeSpaceRefreshAsync()
		}
		if reloadDest {
			h.reloadJobWriteDestinationPanels()
		}
		return false
	}
}

// reloadJobWriteDestinationPanels re-lists panels whose cwd sits inside a non-finished job's
// write (destination) tree so names appear in the file list as soon as they exist on disk.
// Uses panel.Refresh (async when a scheduler is wired); progress wakes are already debounced
// via progress_ui_wake_debounce_ms, and same-directory loads supersede in-flight ones.
func (h *Handler) reloadJobWriteDestinationPanels() {
	marks := h.model.JobPathMarks
	if len(marks) == 0 {
		return
	}
	for _, pan := range []*panel.State{h.host.PrimaryPanel(), h.host.SecondaryPanel()} {
		if pan == nil {
			continue
		}
		if marked, _ := ui.PanelInsideJobWriteTree(pan.PathString(), marks); marked {
			_ = pan.Refresh(0)
		}
	}
}

func (h *Handler) updateJobMessage(ev jobs.Event) {
	switch ev.Type {
	case jobs.EventStarted:
		h.host.SetTransientMessage("Job started", ui.MessageUrgencyInfo)
	case jobs.EventCompleted:
		h.host.SetTransientMessage("Job completed", ui.MessageUrgencySuccess)
	case jobs.EventFailed:
		h.host.SetJobFailedTransientMessage(ev.Err, ev.Error)
	case jobs.EventCanceled:
		h.host.SetTransientMessage("Job canceled", ui.MessageUrgencyInfo)
	}
}

func (h *Handler) commitJob(job *jobs.Job) {
	h.state.AddJob(job)
	h.SyncJobsList()
	h.SyncJobPathMarks()
}

type transferEnqueueOpts struct {
	kind                   dialog.TransferKind
	jobType                jobs.Type
	unsupportedSameDirMove bool
	toastVerb              string
}

func (h *Handler) enqueueTransferJob(opts transferEnqueueOpts) {
	sources, dest := h.sourceAndDestination()
	if len(sources) == 0 {
		h.host.SetTransientMessage("No source files selected", ui.MessageUrgencyWarn)
		return
	}
	if opts.unsupportedSameDirMove {
		active := h.host.ActivePanel()
		if len(sources) == 1 && dest == active.PathString() {
			h.host.SetUnsupportedMessage("Rename/Move (same-directory rename not supported yet)")
			return
		}
	}
	destLoc, err := pathloc.Parse(dest)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Invalid destination: %v", err), ui.MessageUrgencyWarn)
		return
	}
	absDest := destLoc.String()
	srcLocs := make([]pathloc.Path, len(sources))
	for i, src := range sources {
		srcLocs[i] = pathloc.MustParse(src)
	}
	nSelf := ops.SelfTargetCount(srcLocs, destLoc, false)
	if nSelf > 0 {
		if len(sources) > 1 {
			h.host.SetTransientMessage("Cannot transfer multiple items when some would overwrite themselves", ui.MessageUrgencyWarn)
			return
		}
		h.host.OpenTransferDialogSelfCopyRename(opts.kind, absDest, sources[0])
		return
	}
	h.AddTransferJob(TransferJobRequest{
		Type:     opts.jobType,
		Sources:  sources,
		Dest:     dest,
		Preserve: jobs.TransferPreserveFromConfig(h.config.Operations.PreservePermissions, h.config.Operations.PreserveTimestamps, h.config.Operations.DereferenceSymlinks),
	})
	h.host.ActivePanel().ClearSelection()
	h.host.SetTransientMessage(fmt.Sprintf("%s queued (%d %s)", opts.toastVerb, len(sources), jobbridge.Plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
}

func (h *Handler) EnqueueCopyJob() {
	h.enqueueTransferJob(transferEnqueueOpts{
		kind:      dialog.TransferKindCopy,
		jobType:   jobs.TypeCopy,
		toastVerb: "Copy",
	})
}

func (h *Handler) EnqueueMoveJob() {
	h.enqueueTransferJob(transferEnqueueOpts{
		kind:                   dialog.TransferKindMove,
		jobType:                jobs.TypeMove,
		unsupportedSameDirMove: true,
		toastVerb:              "Move",
	})
}

// AddTransferJob enqueues a copy or move job after scanning.
func (h *Handler) AddTransferJob(req TransferJobRequest) {
	srcLocs, err := pathloc.ParseAll(req.Sources)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue job: %v", err), ui.MessageUrgencyError)
		return
	}
	destLoc, err := pathloc.Parse(req.Dest)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue job: %v", err), ui.MessageUrgencyError)
		return
	}
	job := &jobs.Job{
		ID:                  jobs.NewJobID(),
		Type:                req.Type,
		Status:              jobs.StatusScanning,
		Sources:             srcLocs,
		Destination:         destLoc,
		DestIsDir:           ops.DestinationIsDirAtEnqueue(destLoc),
		PausedAfterScan:     req.StartPaused,
		PreservePermissions: req.Preserve.PreservePermissions,
		PreserveTimestamps:  req.Preserve.PreserveTimestamps,
		FlattenIntoDest:     req.Preserve.FlattenIntoDest,
		// DereferenceSymlinks is Copy-only: this is the single place that decides the value
		// written onto the job, and it is unconditionally false for Move regardless of what
		// the dialog/request asked for.
		DereferenceSymlinks: req.Preserve.DereferenceSymlinks && req.Type == jobs.TypeCopy,
		PromptDanglingDirs:  req.Type == jobs.TypeMove && h.config.Operations.RemoveDanglingDirs,
	}
	h.applySizeEstimate(job, req.Sources)
	h.commitJob(job)
}

// applySizeEstimate seeds a copy or move job's byte total from what the active panel already
// knows about its sources (ui.SourceSizeEstimates: listing sizes and the disk-usage cache, no
// filesystem access). Callers enqueue before clearing the selection so off-listing metadata is
// still there. A copy takes any positive estimate as a floor its counting walk replaces; a move,
// which has no counting walk, takes it only when every source is known, since an unknown source
// would never be credited and stall the byte percent.
func (h *Handler) applySizeEstimate(job *jobs.Job, sources []string) {
	if job.Type != jobs.TypeCopy && job.Type != jobs.TypeMove {
		return
	}
	active := h.host.ActivePanel()
	if active == nil {
		return
	}
	lookup := func(p string) (localfs.Entry, bool) {
		if e, ok := active.ListingEntryAt(p); ok {
			return e, true
		}
		return active.OffListingMetaAt(p)
	}
	per, total, complete := ui.SourceSizeEstimates(sources, lookup, active.Path.IsRemote(), h.host.DiskUsagePainter())
	if total <= 0 || (job.Type == jobs.TypeMove && !complete) {
		return
	}
	job.EstimatedBytes = total
	job.TotalBytes = total
	if job.Type == jobs.TypeMove {
		job.SourceSizeHints = per
	}
}

// EnqueueDeleteJob queues a delete job. promptDangling requests the post-completion
// "remove directories left empty" prompt for browser-initiated deletes; callers with
// their own empty-dirs confirm (dedup) or that are themselves the dangling-dirs
// cleanup pass false to avoid double-prompting. The [operations].remove_dangling_directories
// gate is applied here, the single place that decides the job field — callers only
// say whether they want prompting at all.
func (h *Handler) EnqueueDeleteJob(sources []string, removeEmptyDirs, promptDangling bool) {
	srcLocs, err := pathloc.ParseAll(sources)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue delete: %v", err), ui.MessageUrgencyError)
		return
	}
	job := &jobs.Job{
		ID:                    jobs.NewJobID(),
		Type:                  jobs.TypeDelete,
		Status:                jobs.StatusQueued,
		Sources:               srcLocs,
		TotalFiles:            len(sources),
		DeleteRemoveEmptyDirs: removeEmptyDirs,
		PromptDanglingDirs:    promptDangling && h.config.Operations.RemoveDanglingDirs,
	}
	h.commitJob(job)
}

func (h *Handler) EnqueueExtractJob(sources []string, dest string) {
	srcLocs, err := pathloc.ParseAll(sources)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue extract: %v", err), ui.MessageUrgencyError)
		return
	}
	destLoc, err := pathloc.Parse(dest)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue extract: %v", err), ui.MessageUrgencyError)
		return
	}
	job := &jobs.Job{
		ID:          jobs.NewJobID(),
		Type:        jobs.TypeExtract,
		Status:      jobs.StatusQueued,
		Sources:     srcLocs,
		Destination: destLoc,
		DestIsDir:   ops.DestinationIsDirAtEnqueue(destLoc),
		TotalFiles:  len(sources),
	}
	h.commitJob(job)
}

// AddFlattenJob enqueues a flatten (move children + optional empty-dir cleanup) job.
func (h *Handler) AddFlattenJob(req FlattenJobRequest) {
	srcLocs, err := pathloc.ParseAll(req.Roots)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue job: %v", err), ui.MessageUrgencyError)
		return
	}
	destLoc, err := pathloc.Parse(req.Dest)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue job: %v", err), ui.MessageUrgencyError)
		return
	}
	job := &jobs.Job{
		ID:                 jobs.NewJobID(),
		Type:               jobs.TypeFlatten,
		Status:             jobs.StatusQueued,
		Sources:            srcLocs,
		Destination:        destLoc,
		DestIsDir:          ops.DestinationIsDirAtEnqueue(destLoc),
		FlattenRemoveEmpty: req.RemoveEmpty,
		FlattenRecursive:   req.Recursive,
	}
	h.commitJob(job)
}

func (h *Handler) sourceAndDestination() (sources []string, dest string) {
	sources = h.host.ActivePanelSources()
	if sources == nil {
		return nil, ""
	}
	dest = h.host.InactivePanel().PathString()
	return sources, dest
}

func (h *Handler) AffectVisible() bool             { return h.affectVisible }
func (h *Handler) LastBatchMenuBarStripOnly() bool { return h.lastBatchMenuBarStripOnly }
func (h *Handler) ListStale() bool                 { return h.listStale }
func (h *Handler) SetListStale(v bool)             { h.listStale = v }
func (h *Handler) StopWakeTimer()                  { h.stopJobsWakeTimer() }
