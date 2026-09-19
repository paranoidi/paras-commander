package app

import (
	"context"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// swapFetchListingForPanelRefresh replaces the package-level fetch seam for the duration of the
// test and restores it on cleanup, so a fake fetch never touches the real filesystem.
func swapFetchListingForPanelRefresh(t *testing.T, fn func(context.Context, panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error)) {
	t.Helper()
	orig := fetchListingForPanelRefresh
	fetchListingForPanelRefresh = fn
	t.Cleanup(func() { fetchListingForPanelRefresh = orig })
}

// pollUntilPanelRefreshIdle waits until panelRefreshInFlight[panelID] reads false, failing the
// test if it doesn't within timeout.
func pollUntilPanelRefreshIdle(t *testing.T, app *App, panelID int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for app.panelRefreshInFlight[panelID].Load() {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for panel refresh to go idle")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestSchedulePanelListingRefreshHonoursBackoffDeadline proves a still-pending backoff deadline
// (set by a previous slow refresh) stops schedulePanelListingRefresh from even starting the
// fetch goroutine, and a past/zero deadline lets it through as normal.
func TestSchedulePanelListingRefreshHonoursBackoffDeadline(t *testing.T) {
	app := testAppMinimal(t)

	started := make(chan struct{})
	block := make(chan struct{})
	swapFetchListingForPanelRefresh(t, func(ctx context.Context, snap panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
		close(started)
		<-block
		return panel.FetchListing(ctx, snap)
	})

	app.panelRefreshNotBefore[ui.PrimaryPanel].Store(time.Now().Add(time.Hour).UnixNano())
	app.schedulePanelListingRefresh(ui.PrimaryPanel)

	if app.panelRefreshInFlight[ui.PrimaryPanel].Load() {
		t.Fatal("refresh blocked by a future deadline must not flip in-flight")
	}
	select {
	case <-started:
		t.Fatal("refresh blocked by a future deadline must not start the fetch")
	default:
	}

	app.panelRefreshNotBefore[ui.PrimaryPanel].Store(0)
	app.schedulePanelListingRefresh(ui.PrimaryPanel)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for fetch to start once the deadline has passed")
	}
	if !app.panelRefreshInFlight[ui.PrimaryPanel].Load() {
		t.Fatal("refresh past its deadline should be in-flight")
	}
	close(block)
	pollUntilPanelRefreshIdle(t, app, ui.PrimaryPanel, 2*time.Second)
}

// TestPanelListingRefreshRecordsStartToStartDeadline proves the goroutine records a start-to-start
// deadline of start + factor*elapsed, not a gap-to-gap one measured from completion.
func TestPanelListingRefreshRecordsStartToStartDeadline(t *testing.T) {
	app := testAppMinimal(t)

	started := make(chan struct{})
	block := make(chan struct{})
	swapFetchListingForPanelRefresh(t, func(ctx context.Context, snap panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
		close(started)
		<-block
		return panel.FetchListing(ctx, snap)
	})

	before := time.Now()
	app.schedulePanelListingRefresh(ui.PrimaryPanel)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for fetch to start")
	}
	time.Sleep(20 * time.Millisecond)
	close(block)
	pollUntilPanelRefreshIdle(t, app, ui.PrimaryPanel, 2*time.Second)
	after := time.Now()

	d := time.Unix(0, app.panelRefreshNotBefore[ui.PrimaryPanel].Load())

	minElapsed := 20 * time.Millisecond
	lowerBound := before.Add(time.Duration(config.DefaultPanelRefreshSlowBackoffFactor) * minElapsed)
	if d.Before(lowerBound) {
		t.Fatalf("deadline %v is before lower bound %v (elapsed was at least %v)", d, lowerBound, minElapsed)
	}
	maxElapsed := after.Sub(before)
	upperBound := after.Add(time.Duration(config.DefaultPanelRefreshSlowBackoffFactor) * maxElapsed)
	if d.After(upperBound) {
		t.Fatalf("deadline %v is after upper bound %v", d, upperBound)
	}
}
