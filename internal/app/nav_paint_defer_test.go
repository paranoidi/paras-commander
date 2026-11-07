package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// navDeferApp builds an app with a few files, a painted first frame, one queued event and a
// fresh lastNavPaintAt, the state in which a nav key inside Run defers its paint.
func navDeferApp(t *testing.T, runLoop bool) (*App, tcell.SimulationScreen) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"amber", "birch", "cedar", "delta"} {
		writeFile(t, filepath.Join(dir, n))
	}
	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	app.render()
	if err := screen.PostEvent(tcell.NewEventInterrupt(nil)); err != nil {
		t.Fatalf("PostEvent: %v", err)
	}
	app.runLoopActive = runLoop
	app.lastNavPaintAt = time.Now()
	return app, screen
}

func pressDown(app *App) {
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
}

// TestJobsViewScrollPaintDeferredWhenEventsQueued: a held scroll key in the jobs view coalesces
// its paints like file-list nav, and a key that is not a scroll still paints at once.
func TestJobsViewScrollPaintDeferredWhenEventsQueued(t *testing.T) {
	app, screen := navDeferApp(t, true)
	app.jobsCtrl.OpenJobsView()
	app.render()
	app.lastNavPaintAt = time.Now()
	pressDown(app)
	if !app.navPaintDeferred {
		t.Fatal("navPaintDeferred = false, want the paint coalesced")
	}
	drainPendingEvents(screen)
	app.flushDeferredNavPaint()
	if app.navPaintDeferred {
		t.Fatal("flush left the paint deferred")
	}
	if err := screen.PostEvent(tcell.NewEventInterrupt(nil)); err != nil {
		t.Fatalf("PostEvent: %v", err)
	}
	app.lastNavPaintAt = time.Now()
	app.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if app.navPaintDeferred {
		t.Fatal("Tab deferred its paint")
	}
}

func TestNavPaintNotDeferredOutsideRunLoop(t *testing.T) {
	app, screen := navDeferApp(t, false)
	before := ui.HashScreenLogical(screen)
	cur := app.model.Primary.Cursor
	pressDown(app)
	if app.model.Primary.Cursor == cur {
		t.Fatal("cursor did not move")
	}
	if app.navPaintDeferred {
		t.Fatal("navPaintDeferred = true outside Run, want false")
	}
	if ui.HashScreenLogical(screen) == before {
		t.Fatal("screen unchanged, want an immediate paint")
	}
}

func TestNavPaintDeferredWhenEventsQueued(t *testing.T) {
	app, screen := navDeferApp(t, true)
	before := ui.HashScreenLogical(screen)
	cur := app.model.Primary.Cursor
	pressDown(app)
	if app.model.Primary.Cursor == cur {
		t.Fatal("cursor did not move")
	}
	if !app.navPaintDeferred || app.navPaintDeferredPanel != app.model.ActivePanel {
		t.Fatalf("deferred = %v panel = %d, want deferred on the active panel", app.navPaintDeferred, app.navPaintDeferredPanel)
	}
	if ui.HashScreenLogical(screen) != before {
		t.Fatal("screen changed, want no paint while deferred")
	}
}

func TestFlushDeferredNavPaintWaitsForQueueThenPaints(t *testing.T) {
	app, screen := navDeferApp(t, true)
	before := ui.HashScreenLogical(screen)
	pressDown(app)
	app.flushDeferredNavPaint()
	if !app.navPaintDeferred {
		t.Fatal("flush with events queued and a fresh frame painted, want it held")
	}
	screen.PollEvent() // drain the queued event
	app.flushDeferredNavPaint()
	if app.navPaintDeferred {
		t.Fatal("navPaintDeferred = true after flush with a drained queue")
	}
	if ui.HashScreenLogical(screen) == before {
		t.Fatal("screen unchanged after flush, want the deferred paint")
	}
}

func TestFlushDeferredNavPaintPaintsWhenStale(t *testing.T) {
	app, _ := navDeferApp(t, true)
	pressDown(app)
	app.lastNavPaintAt = time.Now().Add(-2 * time.Hour)
	app.flushDeferredNavPaint()
	if app.navPaintDeferred {
		t.Fatal("stale deferred frame was not flushed with events still queued")
	}
}

func TestNavPaintNotDeferredWhenStale(t *testing.T) {
	app, _ := navDeferApp(t, true)
	app.lastNavPaintAt = time.Now().Add(-2 * time.Hour)
	pressDown(app)
	if app.navPaintDeferred {
		t.Fatal("deferred with a stale last paint, want an immediate paint")
	}
}

func TestRenderClearsDeferredNavPaint(t *testing.T) {
	app, _ := navDeferApp(t, true)
	pressDown(app)
	if !app.navPaintDeferred {
		t.Fatal("setup: expected a deferred paint")
	}
	app.render()
	if app.navPaintDeferred {
		t.Fatal("render() left navPaintDeferred set")
	}
}

func TestNonCursorNavActionsAreNeverDeferred(t *testing.T) {
	app, _ := navDeferApp(t, true)
	if !app.browserListNavPartialRenderEligibleFor(keymap.ActionNavParent) || listNavAction(keymap.ActionNavParent) {
		t.Fatal("setup: nav.parent must be a partial-render folder change, not a list-nav action")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if app.navPaintDeferred {
		t.Fatal("folder-change action deferred its paint")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if app.navPaintDeferred {
		t.Fatal("non-nav action deferred its paint")
	}
}
