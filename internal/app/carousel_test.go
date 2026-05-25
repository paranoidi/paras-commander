package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func TestToggleCarouselFlipsActivePanel(t *testing.T) {
	app := testAppMinimal(t)
	if app.model.Left.CarouselMode {
		t.Fatal("carousel should start off")
	}
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleCarousel)
	if !app.model.Left.CarouselMode {
		t.Fatal("carousel should be on after toggle")
	}
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleCarousel)
	if app.model.Left.CarouselMode {
		t.Fatal("carousel should be off after second toggle")
	}
}

func TestCycleListingFormatBlockedInCarousel(t *testing.T) {
	app := testAppMinimal(t)
	app.model.Left.CarouselMode = true
	before := app.model.Left.ListFormat
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelCycleListingFormat)
	if app.model.Left.ListFormat != before {
		t.Fatal("listing format should not change in carousel mode")
	}
}

func TestCarouselTogglePerPanelFromMenuScope(t *testing.T) {
	app := testAppMinimal(t)
	app.model.ActivePanel = ui.RightPanel
	app.activateScopedPanelMenu(ui.RightPanel, menu.Item{
		Action: keymap.ActionPanelToggleCarousel,
		Label:  "Carousel view",
	})
	if !app.model.Right.CarouselMode {
		t.Fatal("right panel carousel should be on")
	}
	if app.model.Left.CarouselMode {
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
	cfg.UI.ZoomActivePanel = false
	cfg.UI.ZoomActivePanelDisabledAboveWidth = 155
	cfg.UI.PanelZoomActivePercent = 70
	cfg.UI.PanelZoomInactivePercent = 30

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

	app.model.Left.CarouselMode = true
	lay := app.layoutForTerminalSize(160, 30)
	if lay.Left.Width != 112 || lay.Right.Width != 48 {
		t.Fatalf("carousel on active panel Left=%d Right=%d want 70/30 zoom at width 160", lay.Left.Width, lay.Right.Width)
	}

	app.model.ActivePanel = ui.RightPanel
	app.model.Right.CarouselMode = true
	layRight := app.layoutForTerminalSize(160, 30)
	if layRight.Left.Width != 48 || layRight.Right.Width != 112 {
		t.Fatalf("carousel on right panel Left=%d Right=%d want 30/70 zoom", layRight.Left.Width, layRight.Right.Width)
	}

	app.model.Right.CarouselMode = false
	app.model.Left.CarouselMode = true
	layInactive := app.layoutForTerminalSize(160, 30)
	if layInactive.Left.Width != 80 || layInactive.Right.Width != 80 {
		t.Fatalf("carousel only on inactive panel Left=%d Right=%d want even split", layInactive.Left.Width, layInactive.Right.Width)
	}
}

func TestToggleZoomIgnoredInCarouselView(t *testing.T) {
	app := testAppMinimal(t)
	app.model.Left.CarouselMode = true
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
	app.config.UI.CarouselPreviewDebounceMS = 500
	app.model.Left.CarouselMode = true

	left := app.panelByID(ui.LeftPanel)
	if _, ok := left.SnapshotChild(20); !ok {
		t.Fatal("SnapshotChild on Season 01 = false, want true")
	}
	if !left.CarouselSideCache.ChildOK {
		t.Fatal("child cache should be warm before first list nav")
	}
	app.carouselPreviewNavSkipSnapshot.Store(false)
	app.syncCarouselChildPreviewCoalesceFlags()
	if app.model.Left.CarouselChildPreviewCoalesce {
		t.Fatal("coalesce should be off before first debounced nav")
	}

	app.ensureCarouselChildCacheBeforeListNav()
	app.beginCarouselPreviewNavCoalesce()
	if !app.model.Left.CarouselChildPreviewCoalesce {
		t.Fatal("coalesce should be on before Move on first debounced nav")
	}
	if !left.SelectVisibleEntry("Season 02") {
		t.Fatal("Season 02 not found")
	}
	app.armCarouselPreviewNavCoalesceAfterListNav()

	if !app.model.Left.CarouselChildPreviewCoalesce {
		t.Fatal("coalesce should stay on after first nav arm")
	}
	_, _, child := panelcarousel.BuildColumns(app.model.Left, 20, false)
	if !child.Populated {
		t.Fatal("first coalesced frame should repaint cached child column, not leave it blank")
	}
	if child.Snapshot.Path.String() != season01 {
		t.Fatalf("coalesced child path = %q, want cached %q", child.Snapshot.Path.String(), season01)
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
	app.config.UI.CarouselPreviewDebounceMS = 500
	app.model.Left.CarouselMode = true

	left := app.panelByID(ui.LeftPanel)
	selectPanelEntryByName(t, left, "maple")
	if _, ok := left.SnapshotChild(20); !ok {
		t.Fatal("SnapshotChild on maple = false, want true")
	}
	first := app.model.Left.CarouselSideCache.Child
	if !app.model.Left.CarouselSideCache.ChildOK || first.Path.String() != maple {
		t.Fatalf("initial child cache = %+v ok=%v, want maple dir cached", first, app.model.Left.CarouselSideCache.ChildOK)
	}

	app.dispatch(keymap.ActionNavDown)
	app.syncCarouselChildPreviewCoalesceFlags()

	if !app.model.Left.CarouselChildPreviewCoalesce {
		t.Fatal("CarouselChildPreviewCoalesce = false, want true during debounce")
	}
	still := app.model.Left.CarouselSideCache.Child
	if !app.model.Left.CarouselSideCache.ChildOK || still.Path.String() != maple {
		t.Fatalf("child cache after debounced nav = %+v ok=%v, want still maple", still, app.model.Left.CarouselSideCache.ChildOK)
	}

	if !app.applyCarouselPreviewFlush(carouselPreviewFlushPayload{gen: app.carouselPreviewDebounceGen.Load()}) {
		t.Fatal("applyCarouselPreviewFlush should accept flush and load child preview")
	}
	app.syncCarouselChildPreviewCoalesceFlags()
	if app.model.Left.CarouselChildPreviewCoalesce {
		t.Fatal("coalesce should be off after flush")
	}
	after := app.model.Left.CarouselSideCache.Child
	if !app.model.Left.CarouselSideCache.ChildOK || after.Path.String() != oak {
		t.Fatalf("child cache after flush = %+v ok=%v, want oak", after, app.model.Left.CarouselSideCache.ChildOK)
	}
}
