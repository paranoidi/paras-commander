package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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

	applyNextInterruptEvent(t, app, screen) // the timeout firing, since the fetch is still blocked

	if pan.ListingPending {
		t.Fatal("ListingPending should clear once the timeout fires")
	}
	if got := pan.PathString(); got != before {
		t.Fatalf("panel path = %q, want unchanged %q (stuck fetch must not apply)", got, before)
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
