package jobs

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/app/jobbridge"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
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
	h.model.JobsView = ui.JobsViewState{Selected: 0, FocusPane: 0, ListScroll: 0, DetailScroll: 0, ActivityScroll: 0, ConflictButtonFocus: 0}
	if h.listStale {
		h.SyncJobsList()
	} else if len(h.model.JobsList) == 0 {
		h.SyncJobsList()
	}
	h.focusJobsViewOnFirstPendingBlocker()
}

func (h *Handler) focusJobsViewOnFirstPendingBlocker() {
	if idx := ui.FirstJobEntryWaitingDecisionIndex(h.model.JobsList); idx >= 0 {
		h.model.JobsView.Selected = idx
		h.model.JobsView.FocusPane = 1
		h.model.JobsView.ConflictButtonFocus = 0
		h.model.JobsView.DetailScroll = 0
		h.model.JobsView.ActivityScroll = 0
	}
	h.ensureJobsViewSelectionVisible()
}

func (h *Handler) closeJobsView() {
	h.model.ViewMode = ui.ViewBrowser
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.model.MenuDefinitions = menu.BrowserDefinitions(h.keys, h.host.DevMode())
	h.model.Menu.ActiveMenu = menu.DefaultIndex()
	h.model.JobsView = ui.JobsViewState{}
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
	h.model.JobsList = ui.JobEntriesFromJobs(all, h.config.Jobs.ThroughputChartEnabled, queueETAs)
	h.listStale = false
	h.listVersion++
}

func (h *Handler) SyncJobPathMarks() {
	h.model.JobPathMarks = ui.JobPathMarksFromJobs(h.state.AllJobs())
	h.pathMarksVersion++
}

func (h *Handler) applyNewFileMarksForCompletedJob(jobID string) {
	for _, job := range h.state.AllJobs() {
		if job == nil || job.ID != jobID {
			continue
		}
		ui.ApplyNewFileMarksFromJob(h.host.LeftPanel(), job)
		ui.ApplyNewFileMarksFromJob(h.host.RightPanel(), job)
		return
	}
}

func (h *Handler) ensureJobsViewSelectionVisible() {
	n := len(h.model.JobsList)
	width, height := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(width, height)
	if layout.TooSmall {
		h.model.JobsView.EnsureSelectionVisible(n, 0)
		return
	}
	visible := ui.PanelListRows(layout.Left)
	h.model.JobsView.EnsureSelectionVisible(n, visible)
}

// tryDispatchJobs handles jobs.* actions. It returns true if actionID is a jobs-domain
// action (consumed here, including deliberate no-ops outside the jobs view).
func (h *Handler) TryDispatch(actionID string) bool {
	switch actionID {
	case keymap.ActionJobsOpen:
		h.ToggleJobsView()
		return true
	case keymap.ActionJobsClose:
		if h.model.ViewMode == ui.ViewJobs {
			h.closeJobsView()
		}
		return true
	case keymap.ActionJobsClearFinished:
		if h.model.ViewMode != ui.ViewJobs {
			return true
		}
		h.clearFinishedJobs()
		return true
	case keymap.ActionJobsQueueUp, keymap.ActionJobsQueueDown, keymap.ActionJobsCancel, keymap.ActionJobsPause, keymap.ActionJobsResume:
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
			h.resumeSelectedPausedJob()
		}
		return true
	default:
		return false
	}
}

func (h *Handler) HandleJobsViewKey(event *tcell.EventKey) bool {
	h.clampJobsFocusPane()
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
	detailPane, activityPane := 1, 2
	if conflictVis {
		detailPane, activityPane = 2, 3
	}

	n := len(h.model.JobsList)
	fp := h.model.JobsView.FocusPane

	if conflictVis && fp == 1 {
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
			switch event.Rune() {
			case 'o', 'O':
				h.submitJobsConflictDecision(jobs.DecisionOverwrite)
				return false
			case 's', 'S':
				h.submitJobsConflictDecision(jobs.DecisionSkip)
				return false
			case 'a', 'A':
				h.submitJobsConflictDecision(jobs.DecisionOverwriteAll)
				return false
			case 'l', 'L':
				h.submitJobsConflictDecision(jobs.DecisionSkipAll)
				return false
			case 'c', 'C':
				h.submitJobsConflictDecision(jobs.DecisionCancel)
				return false
			}
		}
		switch event.Key() {
		case tcell.KeyLeft:
			if h.model.JobsView.ConflictButtonFocus > 0 {
				h.model.JobsView.ConflictButtonFocus--
			}
		case tcell.KeyRight:
			if h.model.JobsView.ConflictButtonFocus < maxB {
				h.model.JobsView.ConflictButtonFocus++
			}
		case tcell.KeyEnter:
			d := ui.JobBlockerDecisionFromFocus(sel, h.model.JobsView.ConflictButtonFocus)
			h.submitJobsConflictDecision(d)
		}
		return false
	}

	switch fp {
	case 0:
		beforeSel := h.model.JobsView.Selected
		switch event.Key() {
		case tcell.KeyUp:
			if h.model.JobsView.Selected > 0 {
				h.model.JobsView.Selected--
			}
			if beforeSel != h.model.JobsView.Selected {
				h.model.JobsView.DetailScroll = 0
				h.model.JobsView.ActivityScroll = 0
			}
			h.ensureJobsViewSelectionVisible()
		case tcell.KeyDown:
			if h.model.JobsView.Selected < n-1 {
				h.model.JobsView.Selected++
			}
			if beforeSel != h.model.JobsView.Selected {
				h.model.JobsView.DetailScroll = 0
				h.model.JobsView.ActivityScroll = 0
			}
			h.ensureJobsViewSelectionVisible()
		case tcell.KeyPgUp:
			h.model.JobsView.Selected = max(0, h.model.JobsView.Selected-5)
			if beforeSel != h.model.JobsView.Selected {
				h.model.JobsView.DetailScroll = 0
				h.model.JobsView.ActivityScroll = 0
			}
			h.ensureJobsViewSelectionVisible()
		case tcell.KeyPgDn:
			h.model.JobsView.Selected = min(n-1, h.model.JobsView.Selected+5)
			if beforeSel != h.model.JobsView.Selected {
				h.model.JobsView.DetailScroll = 0
				h.model.JobsView.ActivityScroll = 0
			}
			h.ensureJobsViewSelectionVisible()
		case tcell.KeyHome:
			h.model.JobsView.Selected = 0
			if beforeSel != h.model.JobsView.Selected {
				h.model.JobsView.DetailScroll = 0
				h.model.JobsView.ActivityScroll = 0
			}
			h.ensureJobsViewSelectionVisible()
		case tcell.KeyEnd:
			if n > 0 {
				h.model.JobsView.Selected = n - 1
			}
			if beforeSel != h.model.JobsView.Selected {
				h.model.JobsView.DetailScroll = 0
				h.model.JobsView.ActivityScroll = 0
			}
			h.ensureJobsViewSelectionVisible()
		}
		if beforeSel != h.model.JobsView.Selected {
			h.model.JobsView.ConflictButtonFocus = 0
		}
	case detailPane:
		contentH := h.jobsDetailContentHeight()
		maxScroll := h.maxDetailScroll(contentH)
		switch event.Key() {
		case tcell.KeyUp:
			if h.model.JobsView.DetailScroll > 0 {
				h.model.JobsView.DetailScroll--
			}
		case tcell.KeyDown:
			if h.model.JobsView.DetailScroll < maxScroll {
				h.model.JobsView.DetailScroll++
			}
		case tcell.KeyPgUp:
			h.model.JobsView.DetailScroll = max(0, h.model.JobsView.DetailScroll-5)
		case tcell.KeyPgDn:
			h.model.JobsView.DetailScroll = min(maxScroll, h.model.JobsView.DetailScroll+5)
		}
	case activityPane:
		contentH := h.jobsActivityContentHeight()
		maxScroll := h.maxActivityScroll(contentH)
		switch event.Key() {
		case tcell.KeyUp:
			if h.model.JobsView.ActivityScroll > 0 {
				h.model.JobsView.ActivityScroll--
			}
		case tcell.KeyDown:
			if h.model.JobsView.ActivityScroll < maxScroll {
				h.model.JobsView.ActivityScroll++
			}
		case tcell.KeyPgUp:
			h.model.JobsView.ActivityScroll = max(0, h.model.JobsView.ActivityScroll-5)
		case tcell.KeyPgDn:
			h.model.JobsView.ActivityScroll = min(maxScroll, h.model.JobsView.ActivityScroll+5)
		}
	default:
		// Defensive: unknown pane index.
	}
	return false
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
	h.state.SubmitConflictDecision(id, d)
	h.model.JobsView.ConflictButtonFocus = 0
}

func (h *Handler) clampJobsFocusPane() {
	maxPane := h.jobsViewFocusPaneCount() - 1
	if h.model.JobsView.FocusPane > maxPane {
		h.model.JobsView.FocusPane = maxPane
	}
}

func (h *Handler) jobsDetailLineCountForSelection() int {
	var sel ui.JobEntry
	if h.model.JobsView.Selected >= 0 && h.model.JobsView.Selected < len(h.model.JobsList) {
		sel = h.model.JobsList[h.model.JobsView.Selected]
	}
	return ui.JobDetailLineCount(sel, time.Now(), h.config.Jobs.ThroughputChartEnabled)
}

func (h *Handler) jobsViewFocusPaneCount() int {
	width, height := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 2
	}
	sel := h.selectedJobEntry()
	showConflict := ui.JobEntryShowsConflictPanel(sel)
	_, _, activityRect := ui.SplitJobsRightPanels(layout.Right, showConflict, h.jobsDetailLineCountForSelection())
	if activityRect.Height == 0 {
		if showConflict {
			return 3 // list, conflict, detail (stacked)
		}
		return 2
	}
	if showConflict {
		return 4
	}
	return 3
}

func (h *Handler) jobsDetailContentHeight() int {
	width, height := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	sel := h.selectedJobEntry()
	show := ui.JobEntryShowsConflictPanel(sel)
	_, detailRect, activityRect := ui.SplitJobsRightPanels(layout.Right, show, h.jobsDetailLineCountForSelection())
	if activityRect.Height > 0 {
		return ui.JobsPanelContentRows(detailRect)
	}
	return ui.JobsPanelContentRows(detailRect)
}

func (h *Handler) jobsActivityContentHeight() int {
	width, height := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	sel := h.selectedJobEntry()
	show := ui.JobEntryShowsConflictPanel(sel)
	_, _, activityRect := ui.SplitJobsRightPanels(layout.Right, show, h.jobsDetailLineCountForSelection())
	return ui.JobsPanelContentRows(activityRect)
}

func (h *Handler) maxDetailScroll(contentH int) int {
	if contentH <= 0 {
		return 0
	}
	var sel ui.JobEntry
	if h.model.JobsView.Selected >= 0 && h.model.JobsView.Selected < len(h.model.JobsList) {
		sel = h.model.JobsList[h.model.JobsView.Selected]
	}
	total := ui.JobDetailLineCount(sel, time.Now(), h.config.Jobs.ThroughputChartEnabled)
	return max(0, total-contentH)
}

func (h *Handler) maxActivityScroll(contentH int) int {
	if contentH <= 0 {
		return 0
	}
	var sel ui.JobEntry
	if h.model.JobsView.Selected >= 0 && h.model.JobsView.Selected < len(h.model.JobsList) {
		sel = h.model.JobsList[h.model.JobsView.Selected]
	}
	act := h.model.JobActivity[sel.ID]
	total := ui.JobActivityLineCount(act)
	return max(0, total-contentH)
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

func (h *Handler) resumeSelectedPausedJob() {
	n := len(h.model.JobsList)
	if n == 0 {
		return
	}
	sel := h.model.JobsView.Selected
	if sel < 0 || sel >= n {
		return
	}
	id := h.model.JobsList[sel].ID
	if h.model.JobsList[sel].Status != string(jobs.StatusPaused) {
		h.host.SetTransientMessage("Selected job is not paused", ui.MessageUrgencyWarn)
		return
	}
	if !h.state.ResumeJob(id) {
		h.host.SetTransientMessage("Could not resume job", ui.MessageUrgencyWarn)
		return
	}
	h.SyncJobsList()
	h.SyncJobPathMarks()
	h.ensureJobsViewSelectionVisible()
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
	var sawTerminal, sawProgress, sawBlocker, sawMarkUpdate bool
	for _, ev := range batch {
		h.state.ApplyEvent(ev)
		h.appendJobActivity(ev)
		h.updateJobMessage(ev)
		switch ev.Type {
		case jobs.EventCompleted:
			sawTerminal = true
			h.applyNewFileMarksForCompletedJob(ev.JobID)
		case jobs.EventFailed, jobs.EventCanceled:
			sawTerminal = true
		case jobs.EventProgress, jobs.EventScanProgress:
			sawProgress = true
		case jobs.EventJobBlockerRequest:
			sawBlocker = true
		}
		if jobbridge.EventUpdatesMarks(ev.Type) {
			sawMarkUpdate = true
		}
	}
	if sawTerminal {
		h.refreshTerminal = true
		h.refreshProgress = false
	} else if sawProgress && !h.refreshTerminal {
		h.refreshProgress = true
	}
	if sawMarkUpdate {
		h.SyncJobPathMarks()
	}
	if viewJobs {
		h.SyncJobsList()
		h.ensureJobsViewSelectionVisible()
	} else if sawTerminal || sawBlocker || sawMarkUpdate || sawProgress {
		h.listStale = true
	}
	h.affectVisible = viewJobs || sawTerminal || sawBlocker ||
		(sawMarkUpdate && (ui.PanelTouchedByJobs(h.model.Left.PathString(), h.model.JobPathMarks) ||
			ui.PanelTouchedByJobs(h.model.Right.PathString(), h.model.JobPathMarks) ||
			h.state.JobsWaitingDecision() > 0)) ||
		(sawProgress && h.model.MenuBarLayoutReserved() && h.state.HasNonFinishedJob())
	h.lastBatchMenuBarStripOnly = !viewJobs && sawProgress && !sawTerminal && !sawBlocker && !sawMarkUpdate &&
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
func (h *Handler) ApplyRefreshes() {
	switch {
	case h.refreshTerminal:
		h.refreshTerminal = false
		h.refreshProgress = false
		h.applyJobsRetention()
		h.host.RefreshBothPanels()
	case h.refreshProgress:
		h.refreshProgress = false
		if h.config.Jobs.FreeSpaceOnProgressWake {
			h.host.RequestBothPanelsVolumeSpaceRefreshAsync()
		}
	}
}

func (h *Handler) appendJobActivity(ev jobs.Event) {
	var label string
	switch ev.Type {
	case jobs.EventProgress:
		if ev.CurrentPath == "" {
			return
		}
		label = jobbridge.ActivityDetailLabel(h.state.ActiveJob(), ev)
	case jobs.EventFailed:
		label = jobbridge.ActivityFailureLabel(ev)
		if label == "" {
			return
		}
	default:
		return
	}
	if h.model.JobActivity == nil {
		h.model.JobActivity = make(map[string][]string)
	}
	lines := h.model.JobActivity[ev.JobID]
	if len(lines) > 0 && lines[len(lines)-1] == label {
		return
	}
	lines = append(lines, label)
	h.model.JobActivity[ev.JobID] = ui.CapJobActivityLines(lines)
}

func (h *Handler) updateJobMessage(ev jobs.Event) {
	switch ev.Type {
	case jobs.EventStarted:
		h.host.SetTransientMessage("Job started", ui.MessageUrgencyInfo)
	case jobs.EventCompleted:
		h.host.SetTransientMessage("Job completed", ui.MessageUrgencyInfo)
	case jobs.EventFailed:
		h.host.SetJobFailedTransientMessage(ev.Err, ev.Error)
	case jobs.EventCanceled:
		h.host.SetTransientMessage("Job canceled", ui.MessageUrgencyInfo)
	}
}

func (h *Handler) EnqueueCopyJob() {
	sources, dest := h.sourceAndDestination()
	if len(sources) == 0 {
		h.host.SetTransientMessage("No source files selected", ui.MessageUrgencyWarn)
		return
	}
	destLoc, err := pathloc.Parse(dest)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Invalid destination: %v", err), ui.MessageUrgencyWarn)
		return
	}
	absDest := destLoc.String()
	nSelf := 0
	for _, src := range sources {
		srcLoc := pathloc.MustParse(src)
		if ops.ResolvedSameAsSource(srcLoc, destLoc) {
			nSelf++
		}
	}
	if nSelf > 0 {
		if len(sources) > 1 {
			h.host.SetTransientMessage("Cannot transfer multiple items when some would overwrite themselves", ui.MessageUrgencyWarn)
			return
		}
		h.host.OpenTransferDialogSelfCopyRename(ui.TransferKindCopy, absDest, sources[0])
		return
	}
	h.host.ActivePanel().ClearSelection()
	h.AddTransferJob(jobs.TypeCopy, sources, dest, false)
	h.host.SetTransientMessage(fmt.Sprintf("Copy queued (%d %s)", len(sources), jobbridge.Plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
}

func (h *Handler) EnqueueMoveJob() {
	sources, dest := h.sourceAndDestination()
	if len(sources) == 0 {
		h.host.SetTransientMessage("No source files selected", ui.MessageUrgencyWarn)
		return
	}
	// Same-directory move: treat as unsupported for foreground rename (plan 04).
	active := h.host.ActivePanel()
	if len(sources) == 1 && dest == active.PathString() {
		h.host.SetUnsupportedMessage("Rename/Move (same-directory rename not supported yet)")
		return
	}
	destLoc, err := pathloc.Parse(dest)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Invalid destination: %v", err), ui.MessageUrgencyWarn)
		return
	}
	absDest := destLoc.String()
	nSelf := 0
	for _, src := range sources {
		srcLoc := pathloc.MustParse(src)
		if ops.ResolvedSameAsSource(srcLoc, destLoc) {
			nSelf++
		}
	}
	if nSelf > 0 {
		if len(sources) > 1 {
			h.host.SetTransientMessage("Cannot transfer multiple items when some would overwrite themselves", ui.MessageUrgencyWarn)
			return
		}
		h.host.OpenTransferDialogSelfCopyRename(ui.TransferKindMove, absDest, sources[0])
		return
	}
	h.host.ActivePanel().ClearSelection()
	h.AddTransferJob(jobs.TypeMove, sources, dest, false)
	h.host.SetTransientMessage(fmt.Sprintf("Move queued (%d %s)", len(sources), jobbridge.Plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
}

// AddTransferJob enqueues a copy or move job after scanning.
func (h *Handler) AddTransferJob(jobType jobs.Type, sources []string, dest string, startPaused bool) {
	srcLocs, err := pathloc.ParseAll(sources)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue job: %v", err), ui.MessageUrgencyError)
		return
	}
	destLoc, err := pathloc.Parse(dest)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue job: %v", err), ui.MessageUrgencyError)
		return
	}
	job := &jobs.Job{
		ID:              jobs.NewJobID(),
		Type:            jobType,
		Status:          jobs.StatusScanning,
		Sources:         srcLocs,
		Destination:     destLoc,
		DestIsDir:       ops.DestinationIsDirAtEnqueue(destLoc),
		PausedAfterScan: startPaused,
	}
	h.state.AddJob(job)
	h.SyncJobsList()
	h.SyncJobPathMarks()
}

func (h *Handler) EnqueueDeleteJob(sources []string) {
	srcLocs, err := pathloc.ParseAll(sources)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue delete: %v", err), ui.MessageUrgencyError)
		return
	}
	job := &jobs.Job{
		ID:         jobs.NewJobID(),
		Type:       jobs.TypeDelete,
		Status:     jobs.StatusQueued,
		Sources:    srcLocs,
		TotalFiles: len(sources),
	}
	h.state.AddJob(job)
	h.SyncJobsList()
	h.SyncJobPathMarks()
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
	h.state.AddJob(job)
	h.SyncJobsList()
	h.SyncJobPathMarks()
}

// AddFlattenJob enqueues a flatten (move children + optional empty-dir cleanup) job.
func (h *Handler) AddFlattenJob(sources []string, dest string, removeEmpty bool, flattenRoots []string) {
	srcLocs, err := pathloc.ParseAll(sources)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue job: %v", err), ui.MessageUrgencyError)
		return
	}
	destLoc, err := pathloc.Parse(dest)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue job: %v", err), ui.MessageUrgencyError)
		return
	}
	rootLocs, err := pathloc.ParseAll(flattenRoots)
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Queue job: %v", err), ui.MessageUrgencyError)
		return
	}
	job := &jobs.Job{
		ID:                 jobs.NewJobID(),
		Type:               jobs.TypeFlatten,
		Status:             jobs.StatusScanning,
		Sources:            srcLocs,
		Destination:        destLoc,
		DestIsDir:          ops.DestinationIsDirAtEnqueue(destLoc),
		FlattenRemoveEmpty: removeEmpty,
		FlattenRoots:       rootLocs,
	}
	h.state.AddJob(job)
	h.SyncJobsList()
	h.SyncJobPathMarks()
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
