package dialog

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestPathPickerBookmarksLoadOffEventLoop(t *testing.T) {
	h, host := newBookmarkTestHandler(t)
	release := stallBookmarkIO(t)
	harbor := filepath.Join(host.active.PathString(), "harbor")

	done := make(chan struct{})
	go func() {
		h.OpenPathPickerForTransferBookmarks()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("OpenPathPickerForTransferBookmarks blocked on delayed filesystem")
	}

	if !h.model.PathPicker.Open {
		t.Fatal("picker should open before the marks file becomes readable")
	}
	if len(h.model.PathPicker.Items) != 0 {
		t.Fatalf("items = %d, want 0 until the load completes", len(h.model.PathPicker.Items))
	}

	close(release)
	p := waitBookmarkIOPayload(t, h.screen.(tcell.SimulationScreen))
	h.ApplyBookmarkIO(p)
	if len(h.model.PathPicker.Items) != 1 || h.model.PathPicker.Items[0].Path != harbor {
		t.Fatalf("items after apply = %+v, want [%s]", h.model.PathPicker.Items, harbor)
	}
}

func TestPathPickerBookmarkLoadStaleCompletionDiscarded(t *testing.T) {
	h, _ := newBookmarkTestHandler(t)
	release := stallBookmarkIO(t)

	h.OpenPathPickerForTransferBookmarks()
	if !h.model.PathPicker.Open {
		t.Fatal("expected picker open")
	}
	staleGen := h.pathPickerMissingGen
	h.ClosePathPicker()
	close(release)

	p := waitBookmarkIOPayload(t, h.screen.(tcell.SimulationScreen))
	if p.Gen != staleGen {
		t.Fatalf("payload gen = %d, want stalled open gen %d", p.Gen, staleGen)
	}
	h.ApplyBookmarkIO(p)
	if h.model.PathPicker.Open {
		t.Fatal("stale load must not reopen a closed picker")
	}
	if len(h.model.PathPicker.Items) != 0 {
		t.Fatalf("stale load applied %d items, want discarded", len(h.model.PathPicker.Items))
	}
}

func TestPathPickerBookmarksNotFilledByLoadAllOnCallerGoroutine(t *testing.T) {
	h, _ := newBookmarkTestHandler(t)
	release := stallBookmarkIO(t)

	h.OpenPathPickerForTransferBookmarks()
	if !h.model.PathPicker.Open {
		t.Fatal("picker should open without waiting for LoadAll")
	}
	if len(h.model.PathPicker.Items) != 0 {
		t.Fatalf("items = %d, want 0; LoadAll must not fill the model on the caller goroutine", len(h.model.PathPicker.Items))
	}

	close(release)
	_ = waitBookmarkIOPayload(t, h.screen.(tcell.SimulationScreen))
}
