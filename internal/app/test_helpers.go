package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/testutil"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

// applyNextInterruptEvent blocks until screen posts an EventInterrupt (e.g. from an async
// scheduler goroutine such as treeChildLoadScheduler/asyncLoadScheduler) and applies it via
// handleInterruptPayload, then runs reconcileAfterEvent — mirroring what Run()'s event loop does
// for every event, interrupts included. That reconcile pass is what retries things like
// ReconcilePendingPanelFocus (rename/mkdir/duplicate's deferred select-and-center) once the async
// reload they were waiting on has just landed. Used by tests that dispatch an action triggering
// async work and need the result (and its follow-on reconcile) applied before asserting on state.
func applyNextInterruptEvent(t *testing.T, app *App, screen tcell.SimulationScreen) {
	t.Helper()
	done := make(chan tcell.Event, 1)
	go func() { done <- screen.PollEvent() }()
	select {
	case ev := <-done:
		interruptEv, ok := ev.(*tcell.EventInterrupt)
		if !ok {
			t.Fatalf("PollEvent returned %T, want *tcell.EventInterrupt", ev)
		}
		app.handleInterruptPayload(interruptEv.Data())
		app.reconcileAfterEvent()
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for async event")
	}
}

// drainInterruptEventsUntil applies every EventInterrupt currently queued on screen (each via
// handleInterruptPayload + reconcileAfterEvent, same as applyNextInterruptEvent), then checks
// cond; it repeats until cond reports true or timeout elapses, failing the test on timeout. Use
// this instead of applyNextInterruptEvent when the exact number/order of async events an action
// schedules isn't easy to predict — e.g. it races unrelated background events (job progress,
// find-index wake-ups) for the same bounded screen event queue, or depends on generation-counter
// supersession ordering. cond may do its own polling as a side effect (e.g. draining a non-screen
// event source like jobsCtrl) before reporting whether the wait is over.
func drainInterruptEventsUntil(t *testing.T, app *App, screen tcell.SimulationScreen, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		for screen.HasPendingEvent() {
			ev := screen.PollEvent()
			if interruptEv, ok := ev.(*tcell.EventInterrupt); ok {
				app.handleInterruptPayload(interruptEv.Data())
				app.reconcileAfterEvent()
			}
		}
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for async event(s) to land")
		}
		time.Sleep(time.Millisecond)
	}
}

func waitUntilAppJobsFinished(t *testing.T, app *App, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		idle := true
		for _, j := range app.jobState.AllJobs() {
			if j != nil && !j.Status.IsFinished() {
				idle = false
				break
			}
		}
		if idle {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout waiting for jobs to finish")
}

func activateFileMenuItem(t *testing.T, app *App, shortcut rune) {
	t.Helper()
	app.dispatch(keymap.ActionAppOpenMenu)
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, shortcut, tcell.ModNone))
	if quit {
		t.Fatal("menu item should not quit")
	}
	if app.model.Menu.Open {
		t.Fatal("menu should be closed after activation")
	}
}

func loadTestTheme(t *testing.T) (theme.Theme, config.Paths) {
	t.Helper()
	data := theme.TestThemeBytes(t, nil)
	dir := t.TempDir()
	path := filepath.Join(dir, "test-theme.toml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	th, err := theme.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	paths := config.Paths{ThemesDir: dir}.WithResolvedLocations()
	return th, paths
}

func testAppMinimal(t *testing.T) *App {
	t.Helper()
	app := newTestApp(t, newScreen(t, 80, 24), testOptions(t.TempDir()))
	// The persistent subshell would grab a real PTY and /dev/tty under a simulation screen.
	app.config.Shell.Persistent = false
	return app
}

// testOptions returns the Options an app-level test starts from: cwd fixed at dir with the
// built-in default config. Callers tweak the returned struct before passing it to newTestApp.
func testOptions(dir string) Options {
	return Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: config.Default(),
	}
}

// newTestApp builds an App and lands both panels' startup listings. Production does the latter
// from Run() (see loadStartupPanelListings); tests never reach Run(), so panels built by
// NewWithOptions are empty until settleStartupPanelListings runs. Build test apps through here
// rather than calling New/NewWithOptions directly, so that rule lives in one place.
func newTestApp(t *testing.T, screen tcell.SimulationScreen, opts Options) *App {
	t.Helper()
	app, err := NewWithOptions(screen, opts)
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	settleStartupPanelListings(t, app)
	t.Cleanup(app.stopWorker)
	return app
}

// settleStartupPanelListings applies both panels' first directory listing synchronously.
// Startup schedules it through the same off-the-UI-thread path as every later navigation (see
// App.loadStartupPanelListings), so a test that inspects entries right after building an App has
// to land it first; doing that here rather than pumping the interrupt queue keeps tests
// deterministic and leaves the event queue untouched. Bumping the generation counter first makes
// applyPanelAsyncLoad drop the in-flight scheduled result as superseded.
func settleStartupPanelListings(t *testing.T, app *App) {
	t.Helper()
	for i, p := range []*panel.State{&app.model.Primary, &app.model.Secondary} {
		app.panelAsyncLoadGen[i].Add(1)
		// A start-path or chooser navigation scheduled during construction is still in flight;
		// land that target rather than the pre-navigation path.
		target := p.PathString()
		if p.ListingPending && p.ListingPendingPath != "" {
			target = p.ListingPendingPath
		}
		sched := p.ScheduleAsyncLoad
		p.ScheduleAsyncLoad = nil
		p.ListingPending = false
		p.ListingPendingPath = ""
		err := p.Load(target)
		p.ScheduleAsyncLoad = sched
		if err != nil {
			t.Fatalf("startup listing for panel %d: %v", i, err)
		}
	}
}

func newScreen(t *testing.T, w, h int) tcell.SimulationScreen {
	return uitest.Screen(t, w, h)
}

func newApp(t *testing.T, screen tcell.SimulationScreen, dir string) *App {
	t.Helper()
	app := newTestApp(t, screen, testOptions(dir))
	app.config.UI.KeyRepeatDebounceMS = 0
	// The persistent subshell would grab a real PTY and /dev/tty under a simulation screen.
	app.config.Shell.Persistent = false
	// Registered after newTestApp's stopWorker cleanup, so LIFO order flushes before stopping.
	t.Cleanup(func() {
		if !app.jobStopOnce {
			flushBackgroundJobs(t, app)
		}
	})
	return app
}

func screenLine(screen tcell.SimulationScreen, y, width int) string {
	var builder strings.Builder
	for x := range width {
		cell, _, _ := screen.Get(x, y)
		if cell == "" {
			cell = " "
		}
		builder.WriteString(cell)
	}
	return builder.String()
}

func writeFile(t *testing.T, path string) {
	testutil.WriteFile(t, path)
}

func flushBackgroundJobs(t *testing.T, app *App) {
	t.Helper()
	const maxIter = 2000
	for i := 0; i < maxIter; i++ {
		app.jobsCtrl.PollEvents()
		busy := false
		for _, j := range app.jobState.AllJobs() {
			switch j.Status {
			case jobs.StatusScanning, jobs.StatusQueued, jobs.StatusRunning, jobs.StatusWaitingDecision:
				busy = true
			}
			if busy {
				break
			}
		}
		if !busy {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for jobs to finish")
}

func selectPanelEntryByName(t *testing.T, p *panel.State, name string) {
	t.Helper()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == name {
			p.Cursor = i
			return
		}
	}
	t.Fatalf("entry %q not visible in panel %q", name, p.PathString())
}
