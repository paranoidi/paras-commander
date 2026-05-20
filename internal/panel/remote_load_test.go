package panel

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestScheduleRemoteLoadReturnsBeforeListingCompletes(t *testing.T) {
	root := pathloc.MustParse("sftp://user@example.com/")
	child, err := root.Join("subdir")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}

	var started atomic.Bool
	block := make(chan struct{})
	state := &State{Path: root}
	state.ScheduleRemoteLoad = func(req RemoteLoadRequest) bool {
		started.Store(true)
		go func() {
			<-block
			_ = state.ApplyListing(req.Loc, []fsbackend.Entry{}, req.SelectedName, req.ViewportRows, req.IndexFallback)
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
		t.Fatal("NavigateToPath blocked waiting for remote listing")
	}
	if !started.Load() {
		t.Fatal("ScheduleRemoteLoad was not invoked")
	}
	if !state.ListingPending {
		t.Fatal("ListingPending should be true while listing is in flight")
	}
	close(block)
}
