package preview

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// carouselRetryHandler builds a handler whose primary panel is a carousel sitting in a real
// subdirectory, so CarouselParentPreviewTarget resolves and the parent cache starts cold.
func carouselRetryHandler(t *testing.T) (*Handler, *fakeHost, string) {
	t.Helper()
	root := t.TempDir()
	inner := filepath.Join(root, "walnut")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	h, fh := newTestHandler(t, 160, 30)
	state, err := panel.New(inner)
	if err != nil {
		t.Fatal(err)
	}
	state.CarouselMode = true
	fh.model.Primary = state
	return h, fh, inner
}

// TestCarouselParentRetryAfterFailedFetch is a regression test: a parent-column fetch that comes
// back with an error (a real failure, or the raceAsyncListingFetch give-up timeout on a saturated
// volume) used to wedge the column permanently. ReconcileCarouselSidePreview only dispatches when
// the cache is invalid AND no fetch is pending for that target; a failure left the pending marker
// set and the cache invalid, so neither condition could be satisfied again while the panel stayed
// in that directory — and SnapshotParent reads the cache without checking which directory it holds,
// so the column kept painting the last successfully fetched directory's listing indefinitely.
func TestCarouselParentRetryAfterFailedFetch(t *testing.T) {
	h, fh, inner := carouselRetryHandler(t)

	h.ReconcileCarouselSidePreview(ui.PrimaryPanel)
	if len(fh.scheduledParent) != 1 {
		t.Fatalf("parent dispatches = %d, want 1 on the first reconcile with a cold cache", len(fh.scheduledParent))
	}
	// While that fetch is in flight, further reconcile passes must not pile on duplicates.
	h.ReconcileCarouselSidePreview(ui.PrimaryPanel)
	if len(fh.scheduledParent) != 1 {
		t.Fatalf("parent dispatches = %d, want 1 while the first fetch is still pending", len(fh.scheduledParent))
	}

	h.NoteCarouselSnapshotFailed(ui.PrimaryPanel, false, inner)

	// The cooldown holds off the immediate retry that the cache-validity check would otherwise
	// fire on every Run-loop pass.
	for range 3 {
		h.ReconcileCarouselSidePreview(ui.PrimaryPanel)
	}
	if len(fh.scheduledParent) != 1 {
		t.Fatalf("parent dispatches = %d, want 1 while the retry cooldown is in effect", len(fh.scheduledParent))
	}

	// Once it expires, the target is retried.
	h.carouselSide[ui.PrimaryPanel].parent.retry.after = time.Now().Add(-time.Second)
	h.ReconcileCarouselSidePreview(ui.PrimaryPanel)
	if len(fh.scheduledParent) != 2 {
		t.Fatalf("parent dispatches = %d, want 2 after the retry cooldown expired", len(fh.scheduledParent))
	}
}

// TestCarouselRetryGateDoesNotDelayADifferentDirectory: the gate is keyed by target, so a failure
// must not make the user wait out a cooldown they earned somewhere else — navigating away from a
// directory whose listing failed has to dispatch straight away.
func TestCarouselRetryGateDoesNotDelayADifferentDirectory(t *testing.T) {
	h, fh, inner := carouselRetryHandler(t)

	h.ReconcileCarouselSidePreview(ui.PrimaryPanel)
	h.NoteCarouselSnapshotFailed(ui.PrimaryPanel, false, inner)
	h.ReconcileCarouselSidePreview(ui.PrimaryPanel)
	if len(fh.scheduledParent) != 1 {
		t.Fatalf("parent dispatches = %d, want 1 (same directory still cooling down)", len(fh.scheduledParent))
	}

	// chdir to a sibling: a different parent target, so the cooldown does not apply.
	sibling := filepath.Join(filepath.Dir(inner), "acorn")
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	state, err := panel.New(sibling)
	if err != nil {
		t.Fatal(err)
	}
	state.CarouselMode = true
	fh.model.Primary = state

	h.ReconcileCarouselSidePreview(ui.PrimaryPanel)
	if len(fh.scheduledParent) != 2 {
		t.Fatalf("parent dispatches = %d, want 2 (a different target must dispatch immediately)", len(fh.scheduledParent))
	}
}
