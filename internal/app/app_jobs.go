package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsvol"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func (a *App) toggleJobsView() {
	if a.model.ViewMode == ui.ViewJobs {
		a.closeJobsView()
		return
	}
	a.openJobsView()
}

func (a *App) openJobsView() {
	a.model.ViewMode = ui.ViewJobs
	a.model.ActiveSubFocus = ui.SubFocusFileList
	a.model.MenuDefinitions = menu.JobsDefinitions(a.keys, a.keysJobs)
	a.model.Menu.ActiveMenu = menu.DefaultIndexJobs()
	a.model.JobsView = ui.JobsViewState{Selected: 0, FocusPane: 0, ListScroll: 0, DetailScroll: 0, ActivityScroll: 0, ConflictButtonFocus: 0}
	a.syncJobsList()
	a.ensureJobsViewSelectionVisible()
}

func (a *App) closeJobsView() {
	a.model.ViewMode = ui.ViewBrowser
	a.model.ActiveSubFocus = ui.SubFocusFileList
	a.model.MenuDefinitions = menu.BrowserDefinitions(a.keys)
	a.model.Menu.ActiveMenu = menu.DefaultIndex()
	a.model.JobsView = ui.JobsViewState{}
}

func (a *App) applyJobsRetention() {
	a.jobState.ApplyRetention(jobs.RetentionPolicy{
		ShowFinished: a.config.Jobs.ShowFinished,
		KeepFinished: a.config.Jobs.KeepFinished,
	})
}

func (a *App) syncJobsList() {
	a.applyJobsRetention()
	a.model.JobsList = ui.JobEntriesFromJobs(a.jobState.AllJobs())
}

func (a *App) ensureJobsViewSelectionVisible() {
	n := len(a.model.JobsList)
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		a.model.JobsView.EnsureSelectionVisible(n, 0)
		return
	}
	visible := ui.PanelListRows(layout.Left)
	a.model.JobsView.EnsureSelectionVisible(n, visible)
}

// tryDispatchJobs handles jobs.* actions. It returns true if actionID is a jobs-domain
// action (consumed here, including deliberate no-ops outside the jobs view).
func (a *App) tryDispatchJobs(actionID string) bool {
	switch actionID {
	case keymap.ActionJobsOpen:
		a.toggleJobsView()
		return true
	case keymap.ActionJobsClose:
		if a.model.ViewMode == ui.ViewJobs {
			a.closeJobsView()
		}
		return true
	case keymap.ActionJobsClearFinished:
		if a.model.ViewMode != ui.ViewJobs {
			return true
		}
		a.clearFinishedJobs()
		return true
	case keymap.ActionJobsQueueUp, keymap.ActionJobsQueueDown, keymap.ActionJobsCancel, keymap.ActionJobsPause, keymap.ActionJobsResume:
		if a.model.ViewMode != ui.ViewJobs {
			return true
		}
		switch actionID {
		case keymap.ActionJobsQueueUp:
			a.moveJobInQueue(-1)
		case keymap.ActionJobsQueueDown:
			a.moveJobInQueue(1)
		case keymap.ActionJobsCancel:
			a.cancelSelectedJob()
		case keymap.ActionJobsPause:
			a.pauseSelectedQueuedJob()
		case keymap.ActionJobsResume:
			a.resumeSelectedPausedJob()
		}
		return true
	default:
		return false
	}
}

func (a *App) handleJobsViewKey(event *tcell.EventKey) bool {
	a.clampJobsFocusPane()
	// Esc closes this screen only; it must not run the global quit action even if
	// the user binds quit to Esc in keybindings.toml.
	switch event.Key() {
	case tcell.KeyEsc:
		a.closeJobsView()
		return false
	case tcell.KeyTab:
		n := a.jobsViewFocusPaneCount()
		a.model.JobsView.FocusPane = (a.model.JobsView.FocusPane + 1) % n
		a.model.JobsView.ConflictButtonFocus = 0
		return false
	case tcell.KeyBacktab:
		n := a.jobsViewFocusPaneCount()
		a.model.JobsView.FocusPane = (a.model.JobsView.FocusPane + n - 1) % n
		a.model.JobsView.ConflictButtonFocus = 0
		return false
	}

	nextAction := a.actionFromKeyEvent(event)
	if nextAction == keymap.ActionAppQuit {
		return a.handleQuit()
	}
	if nextAction == keymap.ActionAppOpenMenu {
		a.openMenu()
		return false
	}

	if nextAction != "" && a.tryDispatchJobs(nextAction) {
		return false
	}
	if nextAction == keymap.ActionPanelExternalBrowser {
		a.dispatch(nextAction)
		return false
	}

	conflictVis := a.jobsViewConflictVisible()
	detailPane, activityPane := 1, 2
	if conflictVis {
		detailPane, activityPane = 2, 3
	}

	n := len(a.model.JobsList)
	fp := a.model.JobsView.FocusPane

	if conflictVis && fp == 1 {
		sel := a.selectedJobEntry()
		maxB := ui.JobsBlockerMaxButtonIndex(sel)
		disk := sel.PendingBlocker != nil && sel.PendingBlocker.Kind == jobs.BlockerKindDiskSpace

		if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModAlt {
			if disk {
				switch event.Rune() {
				case 'r', 'R':
					a.submitJobsConflictDecision(jobs.DecisionRetry)
					return false
				case 'b', 'B':
					a.submitJobsConflictDecision(jobs.DecisionCancel)
					return false
				}
				return false
			}
			switch event.Rune() {
			case 'o', 'O':
				a.submitJobsConflictDecision(jobs.DecisionOverwrite)
				return false
			case 's', 'S':
				a.submitJobsConflictDecision(jobs.DecisionSkip)
				return false
			case 'a', 'A':
				a.submitJobsConflictDecision(jobs.DecisionOverwriteAll)
				return false
			case 'l', 'L':
				a.submitJobsConflictDecision(jobs.DecisionSkipAll)
				return false
			case 'c', 'C':
				a.submitJobsConflictDecision(jobs.DecisionCancel)
				return false
			}
		}
		switch event.Key() {
		case tcell.KeyLeft:
			if a.model.JobsView.ConflictButtonFocus > 0 {
				a.model.JobsView.ConflictButtonFocus--
			}
		case tcell.KeyRight:
			if a.model.JobsView.ConflictButtonFocus < maxB {
				a.model.JobsView.ConflictButtonFocus++
			}
		case tcell.KeyEnter:
			d := ui.JobBlockerDecisionFromFocus(sel, a.model.JobsView.ConflictButtonFocus)
			a.submitJobsConflictDecision(d)
		}
		return false
	}

	switch fp {
	case 0:
		beforeSel := a.model.JobsView.Selected
		switch event.Key() {
		case tcell.KeyUp:
			if a.model.JobsView.Selected > 0 {
				a.model.JobsView.Selected--
			}
			if beforeSel != a.model.JobsView.Selected {
				a.model.JobsView.DetailScroll = 0
				a.model.JobsView.ActivityScroll = 0
			}
			a.ensureJobsViewSelectionVisible()
		case tcell.KeyDown:
			if a.model.JobsView.Selected < n-1 {
				a.model.JobsView.Selected++
			}
			if beforeSel != a.model.JobsView.Selected {
				a.model.JobsView.DetailScroll = 0
				a.model.JobsView.ActivityScroll = 0
			}
			a.ensureJobsViewSelectionVisible()
		case tcell.KeyPgUp:
			a.model.JobsView.Selected = max(0, a.model.JobsView.Selected-5)
			if beforeSel != a.model.JobsView.Selected {
				a.model.JobsView.DetailScroll = 0
				a.model.JobsView.ActivityScroll = 0
			}
			a.ensureJobsViewSelectionVisible()
		case tcell.KeyPgDn:
			a.model.JobsView.Selected = min(n-1, a.model.JobsView.Selected+5)
			if beforeSel != a.model.JobsView.Selected {
				a.model.JobsView.DetailScroll = 0
				a.model.JobsView.ActivityScroll = 0
			}
			a.ensureJobsViewSelectionVisible()
		case tcell.KeyHome:
			a.model.JobsView.Selected = 0
			if beforeSel != a.model.JobsView.Selected {
				a.model.JobsView.DetailScroll = 0
				a.model.JobsView.ActivityScroll = 0
			}
			a.ensureJobsViewSelectionVisible()
		case tcell.KeyEnd:
			if n > 0 {
				a.model.JobsView.Selected = n - 1
			}
			if beforeSel != a.model.JobsView.Selected {
				a.model.JobsView.DetailScroll = 0
				a.model.JobsView.ActivityScroll = 0
			}
			a.ensureJobsViewSelectionVisible()
		}
		if beforeSel != a.model.JobsView.Selected {
			a.model.JobsView.ConflictButtonFocus = 0
		}
	case detailPane:
		contentH := a.jobsDetailContentHeight()
		maxScroll := a.maxDetailScroll(contentH)
		switch event.Key() {
		case tcell.KeyUp:
			if a.model.JobsView.DetailScroll > 0 {
				a.model.JobsView.DetailScroll--
			}
		case tcell.KeyDown:
			if a.model.JobsView.DetailScroll < maxScroll {
				a.model.JobsView.DetailScroll++
			}
		case tcell.KeyPgUp:
			a.model.JobsView.DetailScroll = max(0, a.model.JobsView.DetailScroll-5)
		case tcell.KeyPgDn:
			a.model.JobsView.DetailScroll = min(maxScroll, a.model.JobsView.DetailScroll+5)
		}
	case activityPane:
		contentH := a.jobsActivityContentHeight()
		maxScroll := a.maxActivityScroll(contentH)
		switch event.Key() {
		case tcell.KeyUp:
			if a.model.JobsView.ActivityScroll > 0 {
				a.model.JobsView.ActivityScroll--
			}
		case tcell.KeyDown:
			if a.model.JobsView.ActivityScroll < maxScroll {
				a.model.JobsView.ActivityScroll++
			}
		case tcell.KeyPgUp:
			a.model.JobsView.ActivityScroll = max(0, a.model.JobsView.ActivityScroll-5)
		case tcell.KeyPgDn:
			a.model.JobsView.ActivityScroll = min(maxScroll, a.model.JobsView.ActivityScroll+5)
		}
	default:
		// Defensive: unknown pane index.
	}
	return false
}

func (a *App) jobsViewConflictVisible() bool {
	sel := a.selectedJobEntry()
	return ui.JobEntryShowsConflictPanel(sel)
}

func (a *App) selectedJobEntry() ui.JobEntry {
	if a.model.JobsView.Selected >= 0 && a.model.JobsView.Selected < len(a.model.JobsList) {
		return a.model.JobsList[a.model.JobsView.Selected]
	}
	return ui.JobEntry{}
}

func (a *App) submitJobsConflictDecision(d jobs.ConflictDecision) {
	id := a.selectedJobEntry().ID
	if id == "" {
		return
	}
	a.jobState.SubmitConflictDecision(id, d)
	a.model.JobsView.ConflictButtonFocus = 0
}

func (a *App) clampJobsFocusPane() {
	maxPane := a.jobsViewFocusPaneCount() - 1
	if a.model.JobsView.FocusPane > maxPane {
		a.model.JobsView.FocusPane = maxPane
	}
}

func (a *App) jobsDetailLineCountForSelection() int {
	var sel ui.JobEntry
	if a.model.JobsView.Selected >= 0 && a.model.JobsView.Selected < len(a.model.JobsList) {
		sel = a.model.JobsList[a.model.JobsView.Selected]
	}
	return ui.JobDetailLineCount(sel, time.Now())
}

func (a *App) jobsViewFocusPaneCount() int {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 2
	}
	sel := a.selectedJobEntry()
	showConflict := ui.JobEntryShowsConflictPanel(sel)
	_, _, activityRect := ui.SplitJobsRightPanels(layout.Right, showConflict, a.jobsDetailLineCountForSelection())
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

func (a *App) jobsDetailContentHeight() int {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	sel := a.selectedJobEntry()
	show := ui.JobEntryShowsConflictPanel(sel)
	_, detailRect, activityRect := ui.SplitJobsRightPanels(layout.Right, show, a.jobsDetailLineCountForSelection())
	if activityRect.Height > 0 {
		return ui.JobsPanelContentRows(detailRect)
	}
	return ui.JobsPanelContentRows(detailRect)
}

func (a *App) jobsActivityContentHeight() int {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	sel := a.selectedJobEntry()
	show := ui.JobEntryShowsConflictPanel(sel)
	_, _, activityRect := ui.SplitJobsRightPanels(layout.Right, show, a.jobsDetailLineCountForSelection())
	return ui.JobsPanelContentRows(activityRect)
}

func (a *App) maxDetailScroll(contentH int) int {
	if contentH <= 0 {
		return 0
	}
	var sel ui.JobEntry
	if a.model.JobsView.Selected >= 0 && a.model.JobsView.Selected < len(a.model.JobsList) {
		sel = a.model.JobsList[a.model.JobsView.Selected]
	}
	total := ui.JobDetailLineCount(sel, time.Now())
	return max(0, total-contentH)
}

func (a *App) maxActivityScroll(contentH int) int {
	if contentH <= 0 {
		return 0
	}
	var sel ui.JobEntry
	if a.model.JobsView.Selected >= 0 && a.model.JobsView.Selected < len(a.model.JobsList) {
		sel = a.model.JobsList[a.model.JobsView.Selected]
	}
	act := a.model.JobActivity[sel.ID]
	total := ui.JobActivityLineCount(act)
	return max(0, total-contentH)
}

func (a *App) moveJobInQueue(delta int) {
	q := a.jobQueue()
	qi, ok := a.selectedQueueIndex()
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
	a.syncJobsList()
	// Keep selection on the same job row (swap moves the job).
	a.ensureJobsViewSelectionVisible()
}

func (a *App) selectedQueueIndex() (qi int, ok bool) {
	row := a.model.JobsView.Selected
	if row < 0 {
		return 0, false
	}
	active := a.jobState.ActiveJob()
	if active != nil {
		if row == 0 {
			return 0, false
		}
		qi = row - 1
	} else {
		qi = row
	}
	if qi < 0 || qi >= a.jobQueue().Len() {
		return 0, false
	}
	return qi, true
}

func (a *App) cancelSelectedJob() {
	n := len(a.model.JobsList)
	if n == 0 {
		return
	}
	sel := a.model.JobsView.Selected
	if sel < 0 || sel >= n {
		return
	}
	id := a.model.JobsList[sel].ID
	if !a.jobState.CancelJob(id) {
		a.setTransientMessage("Could not cancel job", ui.MessageUrgencyWarn)
		return
	}
	a.syncJobsList()
	a.ensureJobsViewSelectionVisible()
}

func (a *App) pauseSelectedQueuedJob() {
	n := len(a.model.JobsList)
	if n == 0 {
		return
	}
	sel := a.model.JobsView.Selected
	if sel < 0 || sel >= n {
		return
	}
	if a.model.JobsList[sel].Status != string(jobs.StatusQueued) {
		a.setTransientMessage("Only queued jobs can be paused", ui.MessageUrgencyWarn)
		return
	}
	id := a.model.JobsList[sel].ID
	if !a.jobState.PauseQueuedJob(id) {
		a.setTransientMessage("Could not pause job", ui.MessageUrgencyWarn)
		return
	}
	a.syncJobsList()
	a.ensureJobsViewSelectionVisible()
}

func (a *App) resumeSelectedPausedJob() {
	n := len(a.model.JobsList)
	if n == 0 {
		return
	}
	sel := a.model.JobsView.Selected
	if sel < 0 || sel >= n {
		return
	}
	id := a.model.JobsList[sel].ID
	if a.model.JobsList[sel].Status != string(jobs.StatusPaused) {
		a.setTransientMessage("Selected job is not paused", ui.MessageUrgencyWarn)
		return
	}
	if !a.jobState.ResumeJob(id) {
		a.setTransientMessage("Could not resume job", ui.MessageUrgencyWarn)
		return
	}
	a.syncJobsList()
	a.ensureJobsViewSelectionVisible()
}

func (a *App) clearFinishedJobs() {
	a.jobQueue().ClearFinished()
	a.jobState.ClearFinishedArchive()
	a.syncJobsList()
	if a.model.ViewMode == ui.ViewJobs {
		a.ensureJobsViewSelectionVisible()
	}
}

func (a *App) jobQueue() *jobs.Queue {
	return a.jobState.Queue()
}

func (a *App) jobsWakeDebounce() time.Duration {
	return time.Duration(a.config.Jobs.RefreshDebounceMS) * time.Millisecond
}

// onJobEmitted wakes PollEvent so the main loop can drain jobs.Events().
// Progress emits are debounced; other emits wake immediately.
func (a *App) onJobEmitted(ev jobs.Event) {
	if ev.Type == jobs.EventProgress {
		a.armJobsWakeDebounced()
		return
	}
	a.postJobsWakeImmediate()
}

func (a *App) armJobsWakeDebounced() {
	a.jobsWakeMu.Lock()
	defer a.jobsWakeMu.Unlock()
	if a.jobsWakeTimer != nil {
		if !a.jobsWakeTimer.Stop() {
			select {
			case <-a.jobsWakeTimer.C:
			default:
			}
		}
	}
	a.jobsWakeTimer = time.AfterFunc(a.jobsWakeDebounce(), func() {
		a.jobsWakeMu.Lock()
		a.jobsWakeTimer = nil
		a.jobsWakeMu.Unlock()
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(jobsWakePayload{}))
	})
}

func (a *App) postJobsWakeImmediate() {
	a.stopJobsWakeTimer()
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(jobsWakePayload{}))
}

func (a *App) stopJobsWakeTimer() {
	a.jobsWakeMu.Lock()
	defer a.jobsWakeMu.Unlock()
	if a.jobsWakeTimer == nil {
		return
	}
	if !a.jobsWakeTimer.Stop() {
		select {
		case <-a.jobsWakeTimer.C:
		default:
		}
	}
	a.jobsWakeTimer = nil
}

// pollJobEvents drains pending worker events into UI model state.
// It returns whether any event was processed (caller should repaint when appropriate).
func (a *App) pollJobEvents() bool {
	drained := false
	for {
		select {
		case ev := <-a.jobState.Events():
			drained = true
			a.jobState.ApplyEvent(ev)
			a.appendJobActivity(ev)
			a.updateJobMessage(ev)
			switch ev.Type {
			case jobs.EventCompleted, jobs.EventFailed, jobs.EventCanceled:
				a.applyJobsRetention()
				a.refreshBothPanels()
			}
			a.syncJobsList()
			if a.model.ViewMode == ui.ViewJobs {
				a.ensureJobsViewSelectionVisible()
			}
		default:
			return drained
		}
	}
}

func (a *App) appendJobActivity(ev jobs.Event) {
	if ev.Type != jobs.EventProgress || ev.CurrentPath == "" {
		return
	}
	if a.model.JobActivity == nil {
		a.model.JobActivity = make(map[string][]string)
	}
	lines := a.model.JobActivity[ev.JobID]
	label := jobActivityDetailLabel(a.jobState.ActiveJob(), ev)
	if len(lines) > 0 && lines[len(lines)-1] == label {
		return
	}
	lines = append(lines, label)
	a.model.JobActivity[ev.JobID] = ui.CapJobActivityLines(lines)
}

// jobActivityDetailLabel is shown under Activity: paths relative to the job destination
// root when known (same logical strip as removing Destination from the target path).
func jobActivityDetailLabel(active *jobs.Job, ev jobs.Event) string {
	if active != nil && active.ID == ev.JobID &&
		ev.CurrentDestPath != "" && active.Destination != "" {
		root := filepath.Clean(active.Destination)
		dst := filepath.Clean(ev.CurrentDestPath)
		rel, err := filepath.Rel(root, dst)
		if err == nil && rel != "." &&
			rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return rel
		}
	}
	label := filepath.Base(ev.CurrentPath)
	if label == "." || label == "/" {
		return ev.CurrentPath
	}
	return label
}

func (a *App) stopWorker() {
	if a.commandsCancel != nil {
		a.commandsCancel()
	}
	if !a.jobStopOnce {
		a.jobStopOnce = true
		close(a.jobStopCh)
	}
	a.stopJobsWakeTimer()
	a.stopSpinnerRedrawTimer()
	a.stopDiskUsageRedrawDebounce()
	a.invalidateIdleDiskSortBothPanels()
	if a.diskUsage != nil {
		a.diskUsage.Abort()
	}
}

func (a *App) updateJobMessage(ev jobs.Event) {
	switch ev.Type {
	case jobs.EventStarted:
		a.setTransientMessage("Job started", ui.MessageUrgencyInfo)
	case jobs.EventCompleted:
		a.setTransientMessage("Job completed", ui.MessageUrgencyInfo)
	case jobs.EventFailed:
		a.setTransientMessage(fmt.Sprintf("Job failed: %s", jobFailureBannerDetail(ev.Err, ev.Error)), ui.MessageUrgencyError)
	case jobs.EventCanceled:
		a.setTransientMessage("Job canceled", ui.MessageUrgencyInfo)
	}
}

func jobTransferFunc(opsCfg config.OperationsConfig, jobsCfg config.JobsConfig) func(ctx context.Context, job *jobs.Job, emit func(jobs.Event), waitBlocker func(jobs.BlockerRequest) jobs.ConflictDecision) error {
	return func(ctx context.Context, job *jobs.Job, emit func(jobs.Event), waitBlocker func(jobs.BlockerRequest) jobs.ConflictDecision) error {
		opts := ops.Options{
			PreservePermissions:        opsCfg.PreservePermissions,
			PreserveTimestamps:         opsCfg.PreserveTimestamps,
			CopyBufferKiB:              opsCfg.CopyBufferKiB,
			SyncAfterEachFile:          opsCfg.SyncAfterEachFile,
			DiskSpaceCheckMinFileBytes: opsCfg.DiskSpaceCheckMinFileBytes,
			CowFileCloning:             opsCfg.CowFileCloning,
		}
		throttle := ops.ProgressEmitThrottle{
			MinBytes:    int64(jobsCfg.ProgressEmitMinBytes),
			MinInterval: time.Duration(jobsCfg.ProgressEmitMinIntervalMS) * time.Millisecond,
		}
		resolver := func(src, dst string, facts ops.FileConflictFacts) (bool, error) {
			kind := facts.Kind
			if kind == "" {
				kind = "file"
			}
			req := jobs.ConflictRequest{
				JobID:           job.ID,
				Source:          src,
				Destination:     dst,
				ExistingDetails: kind + " exists",
				SourceSize:      ops.FormatConflictSize(facts.SourceSize),
				SourceTime:      ops.FormatConflictTime(facts.SourceMod),
				DestSize:        ops.FormatConflictSize(facts.DestSize),
				DestTime:        ops.FormatConflictTime(facts.DestMod),
			}
			decision := waitBlocker(jobs.BlockerRequest{
				Kind:     jobs.BlockerKindConflict,
				Conflict: &req,
			})
			switch decision {
			case jobs.DecisionOverwrite, jobs.DecisionOverwriteAll:
				return true, nil
			case jobs.DecisionSkip, jobs.DecisionSkipAll:
				return false, nil
			case jobs.DecisionCancel:
				return false, fmt.Errorf("canceled by user")
			case jobs.DecisionRetry:
				return false, fmt.Errorf("unexpected retry decision for file conflict")
			default:
				return false, nil
			}
		}
		plan, tf, tb, planErr := ops.BuildCopyPlanWithTotals(job.Sources, job.Destination)
		if planErr == nil {
			emit(jobs.Event{
				Type:       jobs.EventPlanTotals,
				JobID:      job.ID,
				Status:     jobs.StatusRunning,
				TotalFiles: tf,
				TotalBytes: tb,
			})
			if job.Type == jobs.TypeCopy && tb > 0 {
				if err := ops.EnsureDiskSpace(waitBlocker, job.Destination, tb, ""); err != nil {
					return err
				}
			}
		}
		progress := func(sourcePath, destPath string, doneFiles int, doneBytes int64) {
			emit(jobs.Event{
				Type:            jobs.EventProgress,
				JobID:           job.ID,
				Status:          jobs.StatusRunning,
				DoneFiles:       doneFiles,
				DoneBytes:       doneBytes,
				CurrentPath:     sourcePath,
				CurrentDestPath: destPath,
			})
		}
		var doneFiles int
		var doneBytes int64
		var err error
		switch job.Type {
		case jobs.TypeCopy:
			if planErr != nil {
				doneFiles, doneBytes, err = ops.ExecuteCopy(ctx, job.Sources, job.Destination, opts, throttle, progress, resolver, waitBlocker)
			} else {
				doneFiles, doneBytes, err = ops.ExecuteCopyUsingPlan(ctx, plan, job.Sources, job.Destination, opts, throttle, progress, resolver, waitBlocker)
			}
		case jobs.TypeMove:
			doneFiles, doneBytes, err = ops.ExecuteMove(ctx, job.Sources, job.Destination, opts, throttle, progress, resolver, waitBlocker)
		default:
			return fmt.Errorf("unknown job type: %s", job.Type)
		}
		if err == nil {
			// Last progress events may be dropped (bounded channel) or debounced in the UI; align with executor totals.
			job.DoneFiles = doneFiles
			job.DoneBytes = doneBytes
		}
		return err
	}
}

func (a *App) enqueueCopyJob() {
	sources, dest := a.sourceAndDestination()
	if len(sources) == 0 {
		a.setTransientMessage("No source files selected", ui.MessageUrgencyWarn)
		return
	}
	absDest := absPathClean(dest)
	nSelf := 0
	for _, src := range sources {
		if ops.ResolvedSameAsSource(src, absDest) {
			nSelf++
		}
	}
	if nSelf > 0 {
		if len(sources) > 1 {
			a.setTransientMessage("Cannot transfer multiple items when some would overwrite themselves", ui.MessageUrgencyWarn)
			return
		}
		a.openTransferDialogSelfCopyRename(ui.TransferKindCopy, absDest, sources[0])
		return
	}
	if a.rejectCopyIfInsufficientDisk(sources, dest) {
		return
	}
	a.activePanel().ClearSelection()
	a.addTransferJob(jobs.TypeCopy, sources, dest, false)
	a.setTransientMessage(fmt.Sprintf("Copy queued (%d %s)", len(sources), plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
}

func (a *App) enqueueMoveJob() {
	sources, dest := a.sourceAndDestination()
	if len(sources) == 0 {
		a.setTransientMessage("No source files selected", ui.MessageUrgencyWarn)
		return
	}
	// Same-directory move: treat as unsupported for foreground rename (plan 04).
	active := a.activePanel()
	if len(sources) == 1 && dest == active.Path {
		a.setUnsupportedMessage("Rename/Move (same-directory rename not supported yet)")
		return
	}
	absDest := absPathClean(dest)
	nSelf := 0
	for _, src := range sources {
		if ops.ResolvedSameAsSource(src, absDest) {
			nSelf++
		}
	}
	if nSelf > 0 {
		if len(sources) > 1 {
			a.setTransientMessage("Cannot transfer multiple items when some would overwrite themselves", ui.MessageUrgencyWarn)
			return
		}
		a.openTransferDialogSelfCopyRename(ui.TransferKindMove, absDest, sources[0])
		return
	}
	a.activePanel().ClearSelection()
	a.addTransferJob(jobs.TypeMove, sources, dest, false)
	a.setTransientMessage(fmt.Sprintf("Move queued (%d %s)", len(sources), plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
}

// rejectCopyIfInsufficientDisk opens a modal and returns true when a copy plan does not fit the destination volume.
func (a *App) rejectCopyIfInsufficientDisk(sources []string, dest string) bool {
	absDest := absPathClean(dest)
	_, totalBytes, planErr := ops.CopyPlanTotals(sources, dest)
	if planErr != nil || totalBytes <= 0 {
		return false
	}
	avail, _, ok := fsvol.VolumeBytes(absDest)
	if !ok || int64(avail) >= totalBytes {
		return false
	}
	msg := "Not enough free space on the destination volume.\n\nRequired: " +
		ui.FormatByteSize(totalBytes) + "\nAvailable: " +
		ui.FormatByteSize(int64(avail))
	a.openMessageDialogTwoButton("Not enough free space", msg)
	return true
}

func (a *App) addTransferJob(jobType jobs.Type, sources []string, dest string, startPaused bool) {
	st := jobs.StatusQueued
	if startPaused {
		st = jobs.StatusPaused
	}
	job := &jobs.Job{
		ID:          jobs.NewJobID(),
		Type:        jobType,
		Status:      st,
		Sources:     sources,
		Destination: dest,
		TotalFiles:  len(sources),
	}
	a.jobState.AddJob(job)
	a.syncJobsList()
}

func (a *App) sourceAndDestination() (sources []string, dest string) {
	sources = a.activePanelSources()
	if sources == nil {
		return nil, ""
	}
	// Destination is the inactive panel's current directory.
	dest = a.inactivePanel().Path
	return sources, dest
}

func (a *App) inactivePanel() *panel.State {
	if a.model.ActivePanel == ui.LeftPanel {
		return &a.model.Right
	}
	return &a.model.Left
}

func (a *App) inactivePanelID() int {
	if a.model.ActivePanel == ui.LeftPanel {
		return ui.RightPanel
	}
	return ui.LeftPanel
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
