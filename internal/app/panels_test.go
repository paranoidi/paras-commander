package app

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestRefreshBothPanelsInactiveWalksUpWhenDirectoryDeleted(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	child := filepath.Join(parent, "gone")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, parent)
	if err := app.inactivePanel().Load(child); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	if err := os.RemoveAll(child); err != nil {
		t.Fatal(err)
	}

	app.dialogCtrl.RefreshBothPanels()

	want := filepath.Clean(parent)
	if got := app.inactivePanel().PathString(); got != want {
		t.Fatalf("inactive path = %q, want %q", got, want)
	}
	if got := app.activePanel().PathString(); got != want {
		t.Fatalf("active path = %q, want unchanged %q", got, want)
	}
}

func TestRefreshBothPanelsActiveWalksUpWhenDirectoryDeleted(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "hollow")
	child := filepath.Join(parent, "pruned")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, parent)
	if err := app.activePanel().Load(child); err != nil {
		t.Fatalf("active Load: %v", err)
	}
	if err := os.RemoveAll(child); err != nil {
		t.Fatal(err)
	}

	app.dialogCtrl.RefreshBothPanels()

	want := filepath.Clean(parent)
	if got := app.activePanel().PathString(); got != want {
		t.Fatalf("active path = %q, want %q", got, want)
	}
}

func TestQuickViewUpdatesAfterDeletedDirectoryRefresh(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(alpha, "a.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	app.model.ActivePanel = ui.PrimaryPanel
	left := app.panelByID(ui.PrimaryPanel)
	selectPanelEntryByName(t, left, "alpha")
	app.model.QuickViewEnabled = true
	app.model.QuickViewPanel = ui.PrimaryPanel
	app.previewCtrl.ApplyQuickViewPreviewImmediately()
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		return !app.model.QuickViewDirOverlay.ListingPending
	})
	if got := filepath.Clean(app.model.QuickViewDirOverlay.Path.String()); got != filepath.Clean(alpha) {
		t.Fatalf("overlay path = %q, want %q", got, alpha)
	}

	if err := os.RemoveAll(alpha); err != nil {
		t.Fatal(err)
	}
	app.dialogCtrl.RefreshBothPanels()
	// Both panel reloads race an unrelated leftover async reload of the quick-view overlay's
	// pre-delete snapshot (scheduled by the setup ApplyQuickViewPreviewImmediately above, before
	// alpha was removed) for the same bounded screen event queue, so drain until both panels have
	// actually landed rather than assuming a fixed 2-event order.
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		return !app.model.Primary.ListingPending && !app.model.Secondary.ListingPending
	})

	if !app.model.QuickViewDirOverlayActive {
		t.Fatal("quick view overlay should stay active after delete refresh")
	}
	if got := filepath.Clean(app.model.QuickViewDirOverlay.Path.String()); got != filepath.Clean(beta) {
		t.Fatalf("overlay path = %q, want next highlight %q", got, beta)
	}
}

// TestLocalTreeRestoreUsesChildSchedulerKeepsLoopResponsive covers returning to a previously
// expanded local tree: ApplyListing must dispatch ScheduleTreeChildLoad instead of ReadDir on
// the event goroutine, so a still-blocked child fetch cannot stall further input.
func TestLocalTreeRestoreUsesChildSchedulerKeepsLoopResponsive(t *testing.T) {
	root := t.TempDir()
	harbor := filepath.Join(root, "harbor")
	if err := os.Mkdir(harbor, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(harbor, "willow.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "cinder.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	left := app.panelByID(ui.PrimaryPanel)
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleTree)
	applyNextInterruptEvent(t, app, screen) // tree child load for harbor
	if !left.TreeExpanded[harbor] {
		t.Fatal("expected harbor expanded before leaving")
	}
	if err := left.NavigateTo(other, "", 20); err != nil {
		t.Fatalf("NavigateTo other: %v", err)
	}
	applyNextInterruptEvent(t, app, screen)

	var scheduled atomic.Bool
	left.ScheduleTreeChildLoad = func(req panel.TreeChildLoadRequest) bool {
		scheduled.Store(true)
		return true
	}

	if err := left.NavigateTo(root, "", 20); err != nil {
		t.Fatalf("NavigateTo root: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // listing apply + restore dispatch
	if !scheduled.Load() {
		t.Fatal("returning to a locally expanded tree must use ScheduleTreeChildLoad")
	}
	if got := left.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount while restore is in flight = %d, want 2", got)
	}

	prior := left.Cursor
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if left.Cursor == prior {
		t.Fatal("Down during in-flight local tree restore did not move the cursor")
	}
}
