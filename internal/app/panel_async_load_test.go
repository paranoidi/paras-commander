package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// swapFetchListingForAsyncLoad replaces the package-level fetch seam for the duration of the
// test and restores it on cleanup, so a fake fetch never touches the real filesystem.
func swapFetchListingForAsyncLoad(t *testing.T, fn func(context.Context, panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error)) {
	t.Helper()
	orig := fetchListingForAsyncLoad
	fetchListingForAsyncLoad = fn
	t.Cleanup(func() { fetchListingForAsyncLoad = orig })
}

// TestAsyncLoadSchedulerTimesOutStuckFetch proves the give-up timer, not the (for local paths,
// inert) context, is what rescues a navigation whose fetch never returns — mirroring a wedged
// autofs/CIFS mount. The panel must fall back to its prior path (nothing ever calls ApplyListing)
// and ListingPending must clear once the timeout fires, instead of hanging forever.
func TestAsyncLoadSchedulerTimesOutStuckFetch(t *testing.T) {
	screen := newScreen(t, 80, 24)
	root := t.TempDir()
	app := newApp(t, screen, root)
	app.config.SFTP.ListTimeoutSecs = 1 // real wall-clock wait, kept minimal

	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	started := make(chan struct{})
	swapFetchListingForAsyncLoad(t, func(ctx context.Context, snap panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
		close(started)
		<-block // never returns before the test's cleanup unblocks it
		return nil, pathloc.Path{}, false, false, nil
	})

	pan := app.panelByID(ui.PrimaryPanel)
	before := pan.PathString()
	if err := pan.NavigateTo(sub, "", app.activeViewportRows()); err != nil {
		t.Fatalf("NavigateTo: %v", err)
	}
	<-started // the fetch goroutine has read/invoked the swapped-in fake before cleanup can restore it
	if !pan.ListingPending {
		t.Fatal("ListingPending should be true while the fetch is stuck")
	}

	// The stuck fetch's give-up timer (1s) now races the working-indicator delay timer (500ms,
	// see dir_loading_indicator.go), which also posts an EventInterrupt to this screen but doesn't
	// clear ListingPending — so wait for the real timeout event rather than assuming it's next.
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool { return !pan.ListingPending })

	if pan.ListingPending {
		t.Fatal("ListingPending should clear once the timeout fires")
	}
	if got := pan.PathString(); got != before {
		t.Fatalf("panel path = %q, want unchanged %q (stuck fetch must not apply)", got, before)
	}
}

// TestDirLoadingIndicatorArmsAfterDelayThenClears proves the working-indicator icon (see
// dir_loading_indicator.go) only arms once a pending navigation load has run longer than
// panel.LoadingIndicatorDelay, targets the entry actually being navigated into, and clears once
// the load lands.
func TestDirLoadingIndicatorArmsAfterDelayThenClears(t *testing.T) {
	screen := newScreen(t, 80, 24)
	root := t.TempDir()
	app := newApp(t, screen, root)
	app.config.SFTP.ListTimeoutSecs = 5 // stays well clear of the 500ms indicator delay

	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	loc, err := pathloc.File(sub)
	if err != nil {
		t.Fatalf("pathloc.File: %v", err)
	}

	block := make(chan struct{})
	started := make(chan struct{})
	swapFetchListingForAsyncLoad(t, func(ctx context.Context, snap panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
		close(started)
		<-block
		return nil, loc, false, false, nil
	})

	pan := app.panelByID(ui.PrimaryPanel)
	if err := pan.NavigateTo(sub, "", app.activeViewportRows()); err != nil {
		t.Fatalf("NavigateTo: %v", err)
	}
	<-started
	if pan.ShowLoadingIcon {
		t.Fatal("ShowLoadingIcon should not be set before the indicator delay elapses")
	}

	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool { return pan.ShowLoadingIcon })
	if got := pan.ListingPendingPath; got != sub {
		t.Fatalf("ListingPendingPath = %q, want %q", got, sub)
	}

	close(block)
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool { return !pan.ListingPending })
	if pan.ShowLoadingIcon {
		t.Fatal("ShowLoadingIcon should clear once the load applies")
	}
	if pan.ListingPendingPath != "" {
		t.Fatalf("ListingPendingPath = %q, want cleared", pan.ListingPendingPath)
	}
}

// TestAsyncLoadSchedulerAppliesFastResult proves a fetch that returns well within the timeout
// applies normally, exercising the same settled/gen plumbing from the other side of the race.
func TestAsyncLoadSchedulerAppliesFastResult(t *testing.T) {
	screen := newScreen(t, 80, 24)
	root := t.TempDir()
	app := newApp(t, screen, root)
	app.config.SFTP.ListTimeoutSecs = 5

	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	loc, err := pathloc.File(sub)
	if err != nil {
		t.Fatalf("pathloc.File: %v", err)
	}
	swapFetchListingForAsyncLoad(t, func(ctx context.Context, snap panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
		return nil, loc, false, false, nil
	})

	pan := app.panelByID(ui.PrimaryPanel)
	if err := pan.NavigateTo(sub, "", app.activeViewportRows()); err != nil {
		t.Fatalf("NavigateTo: %v", err)
	}

	applyNextInterruptEvent(t, app, screen)

	if pan.ListingPending {
		t.Fatal("ListingPending should be false after the fetch result is applied")
	}
	if got := pan.PathString(); got != sub {
		t.Fatalf("panel path = %q, want %q", got, sub)
	}
}

// dropPostEventScreen simulates a saturated tcell queue: PostEvent always fails with
// ErrEventQFull (the production drop path), while PostEventWait still delivers onto the
// wrapped simulation screen.
type dropPostEventScreen struct {
	tcell.SimulationScreen
}

func (s dropPostEventScreen) PostEvent(tcell.Event) error {
	return tcell.ErrEventQFull
}

// saturateSimulationEventQueue fills tcell's 10-slot interrupt queue so a later PostEvent is
// dropped (ErrEventQFull). Callers must drain leftover startup posts first.
func saturateSimulationEventQueue(t *testing.T, screen tcell.SimulationScreen) {
	t.Helper()
	for i := 0; i < 10; i++ {
		if err := screen.PostEvent(tcell.NewEventInterrupt(struct{ n int }{n: i})); err != nil {
			t.Fatalf("saturating PostEvent(%d): %v", i, err)
		}
	}
	if err := screen.PostEvent(tcell.NewEventInterrupt(struct{}{})); err != tcell.ErrEventQFull {
		t.Fatalf("queue should be full, PostEvent = %v, want ErrEventQFull", err)
	}
}

func drainScreenInterrupts(app *App, screen tcell.SimulationScreen) {
	for screen.HasPendingEvent() {
		ev := screen.PollEvent()
		if ie, ok := ev.(*tcell.EventInterrupt); ok {
			app.handleInterruptPayload(ie.Data())
		}
	}
}

// TestPanelAsyncLoadSurvivesSaturatedEventQueue is the characterizing test for dropped listing
// completions: tcell's 10-slot queue is filled, then the fetch finishes. The result must still
// apply exactly once (ListingPending clears, path lands, OnDirectoryChange fires once).
func TestPanelAsyncLoadSurvivesSaturatedEventQueue(t *testing.T) {
	screen := newScreen(t, 80, 24)
	root := t.TempDir()
	app := newApp(t, screen, root)
	app.config.SFTP.ListTimeoutSecs = 5
	drainScreenInterrupts(app, screen)

	sub := filepath.Join(root, "harbor")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	loc, err := pathloc.File(sub)
	if err != nil {
		t.Fatalf("pathloc.File: %v", err)
	}

	block := make(chan struct{})
	started := make(chan struct{})
	var startOnce sync.Once
	swapFetchListingForAsyncLoad(t, func(ctx context.Context, snap panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
		startOnce.Do(func() { close(started) })
		<-block
		return nil, loc, false, false, nil
	})

	pan := app.panelByID(ui.PrimaryPanel)
	var applies int
	pan.OnDirectoryChange = func() { applies++ }
	if err := pan.NavigateTo(sub, "", app.activeViewportRows()); err != nil {
		t.Fatalf("NavigateTo: %v", err)
	}
	<-started
	if !pan.ListingPending {
		t.Fatal("ListingPending should be true while the fetch is held")
	}

	app.screen = dropPostEventScreen{SimulationScreen: screen}
	saturateSimulationEventQueue(t, screen)
	close(block)

	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool { return !pan.ListingPending })
	if pan.ListingPending {
		t.Fatal("ListingPending should clear once even when the tcell queue was full")
	}
	if got := pan.PathString(); got != sub {
		t.Fatalf("panel path = %q, want %q", got, sub)
	}
	if applies != 1 {
		t.Fatalf("OnDirectoryChange fired %d times, want 1", applies)
	}

	drainScreenInterrupts(app, screen)
	if applies != 1 {
		t.Fatalf("second drain re-applied the listing (%d OnDirectoryChange calls)", applies)
	}
}

func historyHasPath(hist []string, path string) bool {
	want, err := pathloc.File(path)
	if err != nil {
		return false
	}
	wantS := want.String()
	for _, p := range hist {
		got, err := pathloc.Parse(p)
		if err == nil && got.Equal(want) {
			return true
		}
		if p == wantS || p == path {
			return true
		}
	}
	return false
}

type asyncNavGate struct {
	started chan struct{}
	start   sync.Once
	release chan error
}

func newAsyncNavGate() *asyncNavGate {
	return &asyncNavGate{started: make(chan struct{}), release: make(chan error, 1)}
}

func (g *asyncNavGate) hold() error {
	g.start.Do(func() { close(g.started) })
	return <-g.release
}

// TestAsyncNavigationHistoryOmitsSupersededVisits is the characterizing test for A→B→C
// history: a visit that never applied (superseded or failed) must not remain in the timeline.
func TestAsyncNavigationHistoryOmitsSupersededVisits(t *testing.T) {
	type step struct {
		nav     string
		release string
		err     error
	}
	cases := []struct {
		name     string
		steps    []step
		want     []string
		dontWant []string
	}{
		{
			name: "B superseded by C",
			steps: []step{
				{nav: "bravo"},
				{nav: "cedar"},
				{release: "cedar"},
				{release: "bravo"},
			},
			want:     []string{"cedar"},
			dontWant: []string{"bravo"},
		},
		{
			name: "B fails",
			steps: []step{
				{nav: "bravo"},
				{release: "bravo", err: errors.New("list failed")},
			},
			dontWant: []string{"bravo"},
		},
		{
			name: "C fails after B superseded",
			steps: []step{
				{nav: "bravo"},
				{nav: "cedar"},
				{release: "cedar", err: errors.New("list failed")},
				{release: "bravo"},
			},
			dontWant: []string{"bravo", "cedar"},
		},
		{
			name: "B applies then C applies",
			steps: []step{
				{nav: "bravo"},
				{release: "bravo"},
				{nav: "cedar"},
				{release: "cedar"},
			},
			want: []string{"bravo", "cedar"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			screen := newScreen(t, 80, 24)
			root := t.TempDir()
			app := newApp(t, screen, root)
			app.config.SFTP.ListTimeoutSecs = 5
			drainScreenInterrupts(app, screen)

			gates := map[string]*asyncNavGate{}
			locs := map[string]pathloc.Path{}
			for _, name := range []string{"bravo", "cedar"} {
				dir := filepath.Join(root, name)
				if err := os.Mkdir(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				loc, err := pathloc.File(dir)
				if err != nil {
					t.Fatal(err)
				}
				gates[name] = newAsyncNavGate()
				locs[name] = loc
			}

			swapFetchListingForAsyncLoad(t, func(ctx context.Context, snap panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
				for name, loc := range locs {
					if snap.Loc.Equal(loc) {
						if err := gates[name].hold(); err != nil {
							return nil, loc, false, false, err
						}
						return nil, loc, false, false, nil
					}
				}
				return nil, snap.Loc, false, false, nil
			})

			pan := app.panelByID(ui.PrimaryPanel)
			start := pan.PathString()
			for _, st := range tc.steps {
				if st.nav != "" {
					if err := pan.NavigateTo(locs[st.nav].String(), "", app.activeViewportRows()); err != nil {
						t.Fatalf("NavigateTo(%s): %v", st.nav, err)
					}
					select {
					case <-gates[st.nav].started:
					case <-time.After(2 * time.Second):
						t.Fatalf("timeout waiting for %s fetch to start", st.nav)
					}
				}
				if st.release != "" {
					released := filepath.Join(root, st.release)
					gates[st.release].release <- st.err
					drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool {
						if st.err == nil && pan.PathString() == locs[st.release].String() {
							return true
						}
						if !pan.ListingPending && !historyHasPath(pan.History, released) {
							return true
						}
						return false
					})
				}
			}

			for _, name := range tc.want {
				if !historyHasPath(pan.History, filepath.Join(root, name)) {
					t.Fatalf("History = %v, want %q present (start %q)", pan.History, name, start)
				}
			}
			for _, name := range tc.dontWant {
				if historyHasPath(pan.History, filepath.Join(root, name)) {
					t.Fatalf("History = %v, superseded/failed %q must not remain", pan.History, name)
				}
			}
			if !historyHasPath(pan.History, start) && start != "" {
				t.Fatalf("History = %v, want starting path %q retained", pan.History, start)
			}
		})
	}
}
