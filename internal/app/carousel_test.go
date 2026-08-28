package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

func TestCarouselOptionStartsPrimaryPanelInCarousel(t *testing.T) {
	root := t.TempDir()
	screen := uitest.Screen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:      func() (string, error) { return root, nil },
		Config:   config.Default(),
		Carousel: true,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	if !app.model.Primary.CarouselMode {
		t.Fatal("Primary.CarouselMode = false, want true with Carousel option")
	}
	if app.model.Primary.ListLayout != panel.ListLayoutFlat {
		t.Fatalf("Primary.ListLayout = %v, want ListLayoutFlat", app.model.Primary.ListLayout)
	}
}

func TestToggleCarouselFlipsActivePanel(t *testing.T) {
	app := testAppMinimal(t)
	if app.model.Primary.CarouselMode {
		t.Fatal("carousel should start off")
	}
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleCarousel)
	if !app.model.Primary.CarouselMode {
		t.Fatal("carousel should be on after toggle")
	}
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleCarousel)
	if app.model.Primary.CarouselMode {
		t.Fatal("carousel should be off after second toggle")
	}
}

func TestCycleListingFormatBlockedInCarousel(t *testing.T) {
	app := testAppMinimal(t)
	app.model.Primary.CarouselMode = true
	before := app.model.Primary.ListFormat
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelCycleListingFormat)
	if app.model.Primary.ListFormat != before {
		t.Fatal("listing format should not change in carousel mode")
	}
}

func TestCarouselTogglePerPanelFromMenuScope(t *testing.T) {
	app := testAppMinimal(t)
	app.model.ActivePanel = ui.SecondaryPanel
	app.activateScopedPanelMenu(ui.SecondaryPanel, menu.Item{
		Action: keymap.ActionPanelToggleCarousel,
		Label:  "Carousel view",
	})
	if !app.model.Secondary.CarouselMode {
		t.Fatal("right panel carousel should be on")
	}
	if app.model.Primary.CarouselMode {
		t.Fatal("left panel carousel should stay off")
	}
}

func TestCarouselForcesPanelZoomRegardlessOfConfigAndWidth(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(160, 30)

	cfg := config.Default()
	cfg.Carousel.AutohideInactivePanel = false
	cfg.UI.Zoom.ActivePanel = false
	cfg.UI.Zoom.DisabledAboveWidth = 155
	cfg.UI.Zoom.ActivePercent = 70
	cfg.UI.Zoom.InactivePercent = 30

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.model.Primary.CarouselMode = true
	lay := app.layoutForTerminalSize(160, 30)
	if lay.Primary.Width != 112 || lay.Secondary.Width != 48 {
		t.Fatalf("carousel on active panel Left=%d Right=%d want 70/30 zoom at width 160", lay.Primary.Width, lay.Secondary.Width)
	}

	app.model.ActivePanel = ui.SecondaryPanel
	app.model.Secondary.CarouselMode = true
	layRight := app.layoutForTerminalSize(160, 30)
	if layRight.Primary.Width != 48 || layRight.Secondary.Width != 112 {
		t.Fatalf("carousel on right panel Left=%d Right=%d want 30/70 zoom", layRight.Primary.Width, layRight.Secondary.Width)
	}

	app.model.Secondary.CarouselMode = false
	app.model.Primary.CarouselMode = true
	layInactive := app.layoutForTerminalSize(160, 30)
	if layInactive.Primary.Width != 80 || layInactive.Secondary.Width != 80 {
		t.Fatalf("carousel only on inactive panel Left=%d Right=%d want even split", layInactive.Primary.Width, layInactive.Secondary.Width)
	}
}

func TestToggleZoomIgnoredInCarouselView(t *testing.T) {
	app := testAppMinimal(t)
	app.model.Primary.CarouselMode = true
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleZoomActivePanel)
	if !strings.Contains(app.model.Message, "carousel") {
		t.Fatalf("message = %q, want carousel zoom hint", app.model.Message)
	}
}

func TestFirstListNavAfterChdirPaintsCachedChildDuringCoalesce(t *testing.T) {
	root := t.TempDir()
	season01 := filepath.Join(root, "Season 01")
	season02 := filepath.Join(root, "Season 02")
	for _, dir := range []string{season01, season02} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "episode.mkv"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 160, 30)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 500
	app.model.Primary.CarouselMode = true

	left := app.panelByID(ui.PrimaryPanel)
	app.scheduleCarouselChildSnapshot(ui.PrimaryPanel, 20)
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool { return left.CarouselSideCache.ChildOK })
	if _, ok := left.SnapshotChild(); !ok {
		t.Fatal("SnapshotChild on Season 01 = false, want true")
	}
	app.previewCtrl.SyncCarouselChildPreviewCoalesceFlags()
	if app.model.Primary.CarouselChildPreviewCoalesce {
		t.Fatal("coalesce should be off before first debounced nav")
	}

	app.previewCtrl.EnsureCarouselChildCacheBeforeListNav()
	app.previewCtrl.BeginCarouselPreviewNavCoalesce()
	if !app.model.Primary.CarouselChildPreviewCoalesce {
		t.Fatal("coalesce should be on before Move on first debounced nav")
	}
	if !left.SelectVisibleEntry("Season 02") {
		t.Fatal("Season 02 not found")
	}
	app.previewCtrl.ArmCarouselPreviewNavCoalesceAfterListNav()

	if !app.model.Primary.CarouselChildPreviewCoalesce {
		t.Fatal("coalesce should stay on after first nav arm")
	}
	_, _, child, _ := panelcarousel.BuildColumns(app.model.Primary, 20, false, false)
	if !child.Populated {
		t.Fatal("first coalesced frame should repaint cached child column, not leave it blank")
	}
	if child.Snapshot.Path.String() != season01 {
		t.Fatalf("coalesced child path = %q, want cached %q", child.Snapshot.Path.String(), season01)
	}
}

// TestCarouselChildSnapshotDispatchedWithoutNavKey is a regression test: the child preview column
// must load for whatever the cursor already sits on after a chdir, without waiting for the user to
// press a nav key. Dispatch used to be hooked only to the nav-key paths, so entering a directory
// whose default highlight is a subdirectory left the child column empty (and the previously cached
// listing on screen) until the cursor was moved off that entry and back.
func TestCarouselChildSnapshotDispatchedWithoutNavKey(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "walnut")
	nested := filepath.Join(inner, "acorn")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 160, 30)
	app := newApp(t, screen, root)
	app.model.Primary.CarouselMode = true

	left := app.panelByID(ui.PrimaryPanel)
	selectPanelEntryByName(t, left, "walnut")
	// Enter walnut; its default highlight is the "acorn" subdirectory. No nav key is pressed after
	// the chdir — only the reconcile pass that Run() performs after every event.
	app.dispatch(keymap.ActionNavOpen)
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool {
		app.reconcileAfterEvent()
		return left.CarouselSideCache.ChildOK
	})

	if got := left.CarouselSideCache.ChildCursorDir; got != nested {
		t.Fatalf("child cache dir = %q, want %q (child must load for the default highlight)", got, nested)
	}
	if _, ok := left.SnapshotChild(); !ok {
		t.Fatal("SnapshotChild = false, want the child column populated without a nav keypress")
	}
}

// TestCarouselNavPaintDeferredUntilParentSnapshotLands is a regression test for carousel flicker
// on folder change: the carousel's column geometry is measured from the parent listing, so a
// navigation must not paint while the parent snapshot still describes the previous directory —
// doing so laid the columns out against the old parent's name lengths and then visibly re-laid
// them out one frame later. The paint is held until the parent snapshot lands (or its deadline).
func TestCarouselNavPaintDeferredUntilParentSnapshotLands(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "walnut")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 160, 30)
	app := newApp(t, screen, inner)
	app.model.Primary.CarouselMode = true
	left := app.panelByID(ui.PrimaryPanel)

	// Parent cache still tagged for some other directory: exactly the window right after a chdir,
	// before this panel's parent snapshot fetch has landed.
	left.CarouselSideCache.ParentOK = true
	left.CarouselSideCache.ParentSourceDir = root
	if !app.carouselParentPaintPending(ui.PrimaryPanel) {
		t.Fatal("parent paint should be pending while the parent cache is stale for the current dir")
	}

	app.renderAfterAsyncApply(ui.PrimaryPanel)
	if !app.carouselPaintDefer[ui.PrimaryPanel].active {
		t.Fatal("repaint should be deferred while the carousel parent snapshot is stale")
	}

	// The parent snapshot landing releases the held paint.
	app.scheduleCarouselParentSnapshot(ui.PrimaryPanel, 20)
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool {
		return !app.carouselPaintDefer[ui.PrimaryPanel].active
	})
	if !left.CarouselParentCacheValid() {
		t.Fatal("parent cache should be valid for the current directory once the snapshot landed")
	}
	if app.carouselParentPaintPending(ui.PrimaryPanel) {
		t.Fatal("parent paint should no longer be pending after the snapshot landed")
	}
}

// TestCarouselDeferredPaintNotReleasedByChildSnapshot is a regression test: ReconcileCarouselSidePreview
// dispatches the parent and child fetches in the same pass and they race, so a small child directory
// routinely lands before a large parent one. Only the parent drives column geometry, so releasing the
// held paint on the child's arrival re-created the exact flicker the deferral exists to prevent — the
// frame painted with the parent's fit width still unmeasured, which resolveWidths falls back to the
// configured cap for, throwing the center column across the panel until the parent landed.
func TestCarouselDeferredPaintNotReleasedByChildSnapshot(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "walnut")
	nested := filepath.Join(inner, "acorn")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 160, 30)
	app := newApp(t, screen, inner)
	app.model.Primary.CarouselMode = true
	left := app.panelByID(ui.PrimaryPanel)
	selectPanelEntryByName(t, left, "acorn")

	// Parent cache still tagged for another directory: the window right after a chdir.
	left.CarouselSideCache.ParentOK = true
	left.CarouselSideCache.ParentSourceDir = root
	app.renderAfterAsyncApply(ui.PrimaryPanel)
	if !app.carouselPaintDefer[ui.PrimaryPanel].active {
		t.Fatal("repaint should be deferred while the carousel parent snapshot is stale")
	}

	// Child lands first, on its own — the parent fetch is never dispatched here.
	app.scheduleCarouselChildSnapshot(ui.PrimaryPanel, 20)
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool {
		return left.CarouselSideCache.ChildOK
	})
	if !app.carouselPaintDefer[ui.PrimaryPanel].active {
		t.Fatal("child snapshot must not release a paint deferred on the parent column's geometry")
	}

	// The parent landing is what releases it.
	app.scheduleCarouselParentSnapshot(ui.PrimaryPanel, 20)
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool {
		return !app.carouselPaintDefer[ui.PrimaryPanel].active
	})
	if app.carouselPaintDefer[ui.PrimaryPanel].active {
		t.Fatal("parent snapshot landing should release the deferred paint")
	}
}

func TestCarouselPreviewNavDebounceDefersSideSnapshotUntilFlush(t *testing.T) {
	root := t.TempDir()
	maple := filepath.Join(root, "maple")
	oak := filepath.Join(root, "oak")
	for _, dir := range []string{maple, oak} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 160, 30)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 500
	app.model.Primary.CarouselMode = true

	left := app.panelByID(ui.PrimaryPanel)
	selectPanelEntryByName(t, left, "maple")
	app.scheduleCarouselChildSnapshot(ui.PrimaryPanel, 20)
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool {
		return left.CarouselSideCache.ChildOK && left.CarouselSideCache.Child.Path.String() == maple
	})
	if _, ok := left.SnapshotChild(); !ok {
		t.Fatal("SnapshotChild on maple = false, want true")
	}
	first := app.model.Primary.CarouselSideCache.Child
	if !app.model.Primary.CarouselSideCache.ChildOK || first.Path.String() != maple {
		t.Fatalf("initial child cache = %+v ok=%v, want maple dir cached", first, app.model.Primary.CarouselSideCache.ChildOK)
	}

	app.dispatch(keymap.ActionNavDown)
	app.previewCtrl.SyncCarouselChildPreviewCoalesceFlags()

	if !app.model.Primary.CarouselChildPreviewCoalesce {
		t.Fatal("CarouselChildPreviewCoalesce = false, want true during debounce")
	}
	still := app.model.Primary.CarouselSideCache.Child
	if !app.model.Primary.CarouselSideCache.ChildOK || still.Path.String() != maple {
		t.Fatalf("child cache after debounced nav = %+v ok=%v, want still maple", still, app.model.Primary.CarouselSideCache.ChildOK)
	}

	if !app.previewCtrl.FlushCarouselPreviewNow() {
		t.Fatal("FlushCarouselPreviewNow should accept flush and load child preview")
	}
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool {
		return app.model.Primary.CarouselSideCache.ChildOK && app.model.Primary.CarouselSideCache.Child.Path.String() == oak
	})
	app.previewCtrl.SyncCarouselChildPreviewCoalesceFlags()
	if app.model.Primary.CarouselChildPreviewCoalesce {
		t.Fatal("coalesce should be off after flush")
	}
	after := app.model.Primary.CarouselSideCache.Child
	if !app.model.Primary.CarouselSideCache.ChildOK || after.Path.String() != oak {
		t.Fatalf("child cache after flush = %+v ok=%v, want oak", after, app.model.Primary.CarouselSideCache.ChildOK)
	}
}
