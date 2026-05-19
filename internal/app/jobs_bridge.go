package app

import (
	"context"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/app/jobbridge"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) openJobsView() { a.jobsCtrl.OpenJobsView() }

func (a *App) tryDispatchJobs(actionID string) bool { return a.jobsCtrl.TryDispatch(actionID) }

func (a *App) handleJobsViewKey(event *tcell.EventKey) bool { return a.jobsCtrl.HandleJobsViewKey(event) }

func (a *App) pollJobEvents() bool { return a.jobsCtrl.PollEvents() }

func (a *App) applyJobRefreshes() { a.jobsCtrl.ApplyRefreshes() }

func (a *App) drainDiscardProgressEvents() { a.jobsCtrl.DrainDiscardProgressEvents() }

func (a *App) onJobEmitted(ev jobs.Event) { a.jobsCtrl.OnJobEmitted(ev) }

func (a *App) syncJobsList() { a.jobsCtrl.SyncJobsList() }

func (a *App) syncJobPathMarks() { a.jobsCtrl.SyncJobPathMarks() }

func (a *App) enqueueCopyJob() { a.jobsCtrl.EnqueueCopyJob() }

func (a *App) enqueueMoveJob() { a.jobsCtrl.EnqueueMoveJob() }

func (a *App) enqueueDeleteJob(sources []string) { a.jobsCtrl.EnqueueDeleteJob(sources) }

func (a *App) addTransferJob(jobType jobs.Type, sources []string, dest string, startPaused bool) {
	a.jobsCtrl.AddTransferJob(jobType, sources, dest, startPaused)
}

func plural(n int, singular, pluralForm string) string {
	return jobbridge.Plural(n, singular, pluralForm)
}

func (a *App) menuBarJobsStripSnapshot() ui.MenuBarJobsStrip {
	return a.jobsCtrl.MenuBarStripSnapshot()
}

func jobScanFunc() jobs.ScanFunc { return jobbridge.ScanFunc() }

func jobTransferFunc(opsCfg config.OperationsConfig, jobsCfg config.JobsConfig) func(context.Context, *jobs.Job, func(jobs.Event), func(jobs.BlockerRequest) jobs.ConflictDecision) error {
	return jobbridge.TransferFunc(opsCfg, jobsCfg)
}

// jobsWakePayload is an alias so the Run loop can match apphandler wake events.
type jobsWakePayload = jobsctrl.WakePayload
