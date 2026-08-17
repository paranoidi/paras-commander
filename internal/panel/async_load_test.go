package panel

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TestScheduleAsyncLoadReturnsBeforeListingCompletes proves NavigateToPath never blocks its
// caller once a scheduler is wired, for both a remote (sftp://) location and, critically, a
// local (file://) one too — a local directory can be just as slow as a remote one (network
// mount, autofs trigger), and this is the regression the freeze fix depends on.
func TestScheduleAsyncLoadReturnsBeforeListingCompletes(t *testing.T) {
	cases := []struct {
		name string
		root pathloc.Path
	}{
		{name: "remote", root: pathloc.MustParse("sftp://user@example.com/")},
		{name: "local", root: pathloc.MustParse(t.TempDir())},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			child, err := tc.root.Join("subdir")
			if err != nil {
				t.Fatalf("Join: %v", err)
			}

			var started atomic.Bool
			block := make(chan struct{})
			state := &State{Path: tc.root}
			state.ScheduleAsyncLoad = func(req AsyncLoadRequest) bool {
				started.Store(true)
				go func() {
					<-block
					_ = state.ApplyListing(req.Loc, []fsbackend.Entry{}, req.SelectedName, req.ViewportRows, req.IndexFallback, req.CenterRecalledCursor)
					state.ListingPending = false
				}()
				return true
			}

			done := make(chan error, 1)
			go func() {
				done <- state.NavigateToPath(child, "", 10)
			}()

			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("NavigateToPath: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("NavigateToPath blocked waiting for listing")
			}
			if !started.Load() {
				t.Fatal("ScheduleAsyncLoad was not invoked")
			}
			if !state.ListingPending {
				t.Fatal("ListingPending should be true while listing is in flight")
			}
			close(block)
		})
	}
}
