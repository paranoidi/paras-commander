package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// removedRowsFixture is an app with the active panel in source (holding the given directories)
// and the inactive panel in dest.
func removedRowsFixture(t *testing.T, dirs ...string) (app *App, screen tcell.SimulationScreen, source, dest string) {
	t.Helper()
	root := t.TempDir()
	source, dest = filepath.Join(root, "source"), filepath.Join(root, "dest")
	for _, d := range []string{source, dest} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range dirs {
		if err := os.Mkdir(filepath.Join(source, name), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(source, name, "inner.txt"))
	}
	screen = newScreen(t, 80, 24)
	app = newApp(t, screen, source)
	if err := app.inactivePanel().Load(dest); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	return app, screen, source, dest
}

func panelListsRow(p *panel.State, name string) bool {
	return slices.ContainsFunc(p.Entries, func(e localfs.Entry) bool { return e.Name == name })
}

// pumpJobsUntil runs the main loop's job handling (poll, apply refreshes, async listings) until
// cond holds.
func pumpJobsUntil(t *testing.T, app *App, screen tcell.SimulationScreen, cond func() bool) {
	t.Helper()
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool {
		app.jobsCtrl.PollEvents()
		_ = app.jobsCtrl.ApplyRefreshes()
		return cond()
	})
}

// A multi-source move stays unfinished (here: the conflict on its second source), yet the source
// it already moved must leave the source panel, not linger as a row the other panel "also has".
func TestMoveDropsMovedSourceRowWhileJobRuns(t *testing.T) {
	app, screen, source, dest := removedRowsFixture(t, "alder")
	writeFile(t, filepath.Join(source, "birch.txt"))
	writeFile(t, filepath.Join(dest, "birch.txt")) // blocks the job on a conflict
	p := app.activePanel()
	p.AddSelection(filepath.Join(source, "alder"))
	p.AddSelection(filepath.Join(source, "birch.txt"))

	app.dialogCtrl.OpenMoveDialog()
	app.dialogCtrl.ApplyTransferEvent(dlg.Event{ID: dlg.OK})

	var job *jobs.Job
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool {
		app.jobsCtrl.PollEvents()
		_ = app.jobsCtrl.ApplyRefreshes()
		for _, j := range app.jobState.AllJobs() {
			job = j
		}
		return job != nil && job.Status == jobs.StatusWaitingDecision && !p.ListingPending && !panelListsRow(p, "alder")
	})
	if _, err := os.Stat(filepath.Join(dest, "alder")); err != nil {
		t.Fatalf("alder should have moved: %v", err)
	}
	if !panelListsRow(p, "birch.txt") {
		t.Fatal("birch.txt has not moved yet and must keep its row")
	}
	app.jobState.SubmitBlockerDecision(job.ID, jobs.DecisionSkip)
}

// Once a job finishes no row of a source it removed may remain, in either of the ways out.
func TestFinishedJobsDropRemovedSourceRows(t *testing.T) {
	names := []string{"alder", "birch", "cedar"}
	cases := map[string]func(app *App, source string){
		"move": func(app *App, source string) {
			for _, n := range names {
				app.activePanel().AddSelection(filepath.Join(source, n))
			}
			app.dialogCtrl.OpenMoveDialog()
			app.dialogCtrl.ApplyTransferEvent(dlg.Event{ID: dlg.OK})
		},
		"delete": func(app *App, source string) {
			paths := make([]string, len(names))
			for i, n := range names {
				paths[i] = filepath.Join(source, n)
			}
			app.jobsCtrl.EnqueueDeleteJob(paths, false, false)
		},
	}
	for name, start := range cases {
		t.Run(name, func(t *testing.T) {
			app, screen, source, _ := removedRowsFixture(t, append([]string{"keeper"}, names...)...)
			p := app.activePanel()
			start(app, source)
			pumpJobsUntil(t, app, screen, func() bool {
				return !app.jobState.HasUnfinishedWork() && !p.ListingPending && !slices.ContainsFunc(names, func(n string) bool { return panelListsRow(p, n) })
			})
			if !panelListsRow(p, "keeper") {
				t.Fatal("untouched row keeper vanished")
			}
		})
	}
}

// The periodic refresh yields to a job on the volume, except for the directory that job is
// removing rows from: there it is the safety net when no job refresh arrives.
func TestPeriodicRefreshDropsRowOfVanishedJobSource(t *testing.T) {
	app, screen, source, dest := removedRowsFixture(t, "alder", "birch")
	p := app.activePanel()
	app.jobState.AddJob(&jobs.Job{
		ID:          "moving",
		Type:        jobs.TypeMove,
		Status:      jobs.StatusPaused,
		Sources:     pathloc.PathsForTest(filepath.Join(source, "alder"), filepath.Join(source, "birch")),
		Destination: pathloc.MustParse(dest),
	})
	app.jobsCtrl.SyncJobPathMarks()
	if !app.pathVolumeContendsWithActiveJob(p.PathString()) {
		t.Fatal("source should contend with the job")
	}
	if err := os.RemoveAll(filepath.Join(source, "alder")); err != nil {
		t.Fatal(err)
	}

	app.schedulePanelListingRefresh(ui.PrimaryPanel)

	t.Cleanup(func() { app.jobState.CancelJob("moving") })
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool { return !panelListsRow(p, "alder") })
}

// A periodic listing read before a job's reload must not land ahead of it: applying it bumps the
// listing epoch, and the reload's newer result would then be dropped as stale.
func TestPeriodicRefreshYieldsToPendingReload(t *testing.T) {
	app, _, source, _ := removedRowsFixture(t, "alder")
	p := app.activePanel()
	entries := panel.BackendEntriesFromPanel(p.Entries)
	entries = append(entries, fsbackend.Entry{Name: "ghost", Type: fsbackend.EntryDirectory})
	epoch := p.ListingEpoch
	p.ListingPending, p.ListingPendingPath = true, source

	if app.applyPanelListingRefresh(panelRefreshApplyPayload{PanelID: ui.PrimaryPanel, Path: p.Path, Entries: entries, ListingEpoch: epoch}) {
		t.Fatal("stale periodic listing applied over a pending reload")
	}
	if p.ListingEpoch != epoch {
		t.Fatalf("epoch moved to %d, a pending reload would be dropped", p.ListingEpoch)
	}
}
