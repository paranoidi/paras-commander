package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
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
		ui.ApplyNewFileMarksFromJob(h.host.PrimaryPanel(), job)
		ui.ApplyNewFileMarksFromJob(h.host.SecondaryPanel(), job)
		return
	}
}

// stashDanglingSourcesForCompletedJob queues job.Sources for the dangling-dirs check
// (see promptDanglingDirsIfAny) when the just-completed job was enqueued with
// PromptDanglingDirs. No filesystem I/O here — this runs on the event-batch path.
func (h *Handler) stashDanglingSourcesForCompletedJob(jobID string) {
	for _, job := range h.state.AllJobs() {
		if job == nil || job.ID != jobID {
			continue
		}
		if job.PromptDanglingDirs {
			h.pendingDanglingSources = append(h.pendingDanglingSources, job.Sources...)
		}
		return
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
	n := len(h.model.JobsList)
	width, height := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(width, height)
	if layout.TooSmall {
		h.model.JobsView.EnsureSelectionVisible(n, 0)
		return
	}
	visible := ui.PanelListRows(layout.Primary)
	h.model.JobsView.EnsureSelectionVisible(n, visible)
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
	fp := h.model.JobsView.FocusPane

	if conflictVis && fp == 1 {
		return h.handleJobsConflictPaneKey(event)
	}

	h.handleJobsPaneScrollKey(event, fp, conflictVis)
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
	case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown, tcell.KeyTab, tcell.KeyBacktab:
		if sel.PendingBlocker == nil {
			return false
		}
		newFocus, handled := ui.JobBlockerDialogMoveFocus(*sel.PendingBlocker, h.model.JobsView.ConflictButtonFocus, event.Key())
		if handled {
			if newFocus > maxB {
				newFocus = maxB
			}
			h.model.JobsView.ConflictButtonFocus = newFocus
		}
	case tcell.KeyEnter:
		d := ui.JobBlockerDecisionFromFocus(sel, h.model.JobsView.ConflictButtonFocus)
		h.submitJobsConflictDecision(d)
	}
	return false
}

// handleJobsPaneScrollKey handles Up/Down/PgUp/PgDn (and Home/End on the list pane) for
// whichever pane currently has focus (list, detail, or activity). detailPane/activityPane
// indices shift by one when the conflict panel is visible (see conflictVis).
func (h *Handler) handleJobsPaneScrollKey(event *tcell.EventKey, fp int, conflictVis bool) {
	detailPane, activityPane := 1, 2
	if conflictVis {
		detailPane, activityPane = 2, 3
	}
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
}

// MoveSelection moves the jobs-list primary-pane cursor by delta rows (clamped), scrolling
// into view and resetting detail/activity scroll + conflict focus when the row actually
// changes. Shared by the raw arrow-key handling below and by help-dialog activation.
func (h *Handler) MoveSelection(delta int) {
	n := len(h.model.JobsList)
	sel := max(0, h.model.JobsView.Selected+delta)
	if n > 0 && sel > n-1 {
		sel = n - 1
	}
	h.setSelection(sel)
}

// SelectEdge moves the jobs-list cursor to the first (toEnd=false) or last (toEnd=true) row.
func (h *Handler) SelectEdge(toEnd bool) {
	n := len(h.model.JobsList)
	sel := 0
	if toEnd && n > 0 {
		sel = n - 1
	}
	h.setSelection(sel)
}

func (h *Handler) setSelection(sel int) {
	beforeSel := h.model.JobsView.Selected
	h.model.JobsView.Selected = sel
	if beforeSel != sel {
		h.model.JobsView.DetailScroll = 0
		h.model.JobsView.ActivityScroll = 0
		h.model.JobsView.ConflictButtonFocus = 0
	}
	h.ensureJobsViewSelectionVisible()
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
	_, _, activityRect := ui.SplitJobsSecondaryPanels(layout.Secondary, showConflict, h.jobsDetailLineCountForSelection())
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
	_, detailRect, activityRect := ui.SplitJobsSecondaryPanels(layout.Secondary, show, h.jobsDetailLineCountForSelection())
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
	_, _, activityRect := ui.SplitJobsSecondaryPanels(layout.Secondary, show, h.jobsDetailLineCountForSelection())
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
	flags := h.scanBatchFlags(batch)
	if flags.Terminal {
		h.refreshTerminal = true
		h.refreshProgress = false
	} else if flags.Progress && !h.refreshTerminal {
		h.refreshProgress = true
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
}

// scanBatchFlags applies each event in the batch to UI model state (jobs.State, activity log,
// job message) and reports which categories of event it contained, moved out of
// applyJobEventBatch's accumulation loop.
func (h *Handler) scanBatchFlags(batch []jobs.Event) batchFlags {
	var f batchFlags
	for _, ev := range batch {
		h.state.ApplyEvent(ev)
		h.appendJobActivity(ev)
		h.updateJobMessage(ev)
		switch ev.Type {
		case jobs.EventCompleted:
			f.Terminal = true
			h.applyNewFileMarksForCompletedJob(ev.JobID)
			h.stashDanglingSourcesForCompletedJob(ev.JobID)
		case jobs.EventFailed, jobs.EventCanceled:
			f.Terminal = true
		case jobs.EventProgress, jobs.EventScanProgress:
			f.Progress = true
		case jobs.EventJobBlockerRequest:
			f.Blocker = true
		}
		if jobbridge.EventUpdatesMarks(ev.Type) {
			f.MarkUpdate = true
		}
	}
	return f
}

// batchAffectsVisible reports whether the batch requires an immediate visible-area repaint
// (independent of whether the Jobs view itself is open — the caller ORs in viewJobs), moved
// out of applyJobEventBatch's dense affectVisible boolean expression.
func (h *Handler) batchAffectsVisible(f batchFlags) bool {
	return f.Terminal || f.Blocker ||
		(f.MarkUpdate && (ui.PanelTouchedByJobs(h.model.Primary.PathString(), h.model.JobPathMarks) ||
			ui.PanelTouchedByJobs(h.model.Secondary.PathString(), h.model.JobPathMarks) ||
			h.state.JobsWaitingDecision() > 0)) ||
		(f.Progress && h.model.MenuBarLayoutReserved() && h.state.HasNonFinishedJob())
}

// batchIsMenuBarStripOnly reports whether the batch only needs the menu-bar activity strip
// repainted (progress with a live non-finished job, and nothing terminal/blocker/mark-related),
// moved out of applyJobEventBatch's dense lastBatchMenuBarStripOnly boolean expression.
func (h *Handler) batchIsMenuBarStripOnly(f batchFlags) bool {
	return f.Progress && !f.Terminal && !f.Blocker && !f.MarkUpdate &&
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
		h.applyJobsRetention()
		h.SyncJobPathMarks()
		h.host.RefreshBothPanels()
		h.promptDanglingDirsIfAny()
		return true
	case h.refreshProgress:
		h.refreshProgress = false
		if h.config.Jobs.FreeSpaceOnProgressWake {
			h.host.RequestBothPanelsVolumeSpaceRefreshAsync()
		}
		return false
	default:
		return false
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

func (h *Handler) commitJob(job *jobs.Job) {
	h.state.AddJob(job)
	h.SyncJobsList()
	h.SyncJobPathMarks()
	h.applyOptimisticListingForJob(job)
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
	h.host.ActivePanel().ClearSelection()
	h.AddTransferJob(TransferJobRequest{
		Type:     opts.jobType,
		Sources:  sources,
		Dest:     dest,
		Preserve: jobs.TransferPreserveFromConfig(h.config.Operations.PreservePermissions, h.config.Operations.PreserveTimestamps),
	})
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
		PromptDanglingDirs:  req.Type == jobs.TypeMove && h.config.Operations.RemoveDanglingDirs,
	}
	h.commitJob(job)
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
	rootLocs, err := pathloc.ParseAll(req.FlattenRoots)
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
		FlattenRemoveEmpty: req.RemoveEmpty,
		FlattenRoots:       rootLocs,
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
