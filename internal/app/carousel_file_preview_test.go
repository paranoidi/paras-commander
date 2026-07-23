package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestReconcileCarouselFilePreviewStartsPreview(t *testing.T) {
	root := t.TempDir()
	scroll := filepath.Join(root, "scroll.txt")
	if err := os.WriteFile(scroll, []byte("river delta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 200, 30)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0
	app.model.HideInactivePanel = true
	app.model.Primary.CarouselMode = true
	if !app.model.Primary.SelectVisibleEntry("scroll.txt") {
		t.Fatal("scroll.txt not found")
	}

	app.reconcileCarouselFilePreview()

	app.commandsMu.RLock()
	open := app.model.CarouselFilePreview.Open
	path := app.model.CarouselFilePreview.Path
	app.commandsMu.RUnlock()
	if !open {
		t.Fatal("CarouselFilePreview.Open = false, want true after reconcile")
	}
	if path != scroll {
		t.Fatalf("CarouselFilePreview.Path = %q, want %q", path, scroll)
	}
}

func TestReconcileCarouselFilePreviewClosesOnDirectoryCursor(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 200, 30)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0
	app.model.HideInactivePanel = true
	app.model.Primary.CarouselMode = true
	app.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Path = filepath.Join(root, "stale.txt")
	})
	app.carouselFilePreviewLastFingerprint = "f:" + filepath.Join(root, "stale.txt")

	if !app.model.Primary.SelectVisibleEntry("nested") {
		t.Fatal("nested not found")
	}
	app.reconcileCarouselFilePreview()

	app.commandsMu.RLock()
	open := app.model.CarouselFilePreview.Open
	app.commandsMu.RUnlock()
	if open {
		t.Fatal("CarouselFilePreview.Open = true, want closed on directory cursor")
	}
}

func TestReconcileCarouselFilePreviewPreservesOpenDuringQuickFilter(t *testing.T) {
	root := t.TempDir()
	scroll := filepath.Join(root, "scroll.txt")
	if err := os.WriteFile(scroll, []byte("river delta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 200, 30)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0
	app.model.HideInactivePanel = true
	app.model.Primary.CarouselMode = true
	if !app.model.Primary.SelectVisibleEntry("scroll.txt") {
		t.Fatal("scroll.txt not found")
	}
	app.reconcileCarouselFilePreview()
	app.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.Path = scroll
		st.CombinedText = "river delta\n"
	})

	app.model.Primary.OpenFilter(app.activeViewportRows())
	app.model.Primary.AppendFilterRune('n', app.activeViewportRows()) // jumps to nested dir match
	app.reconcileCarouselFilePreview()

	app.commandsMu.RLock()
	open := app.model.CarouselFilePreview.Open
	app.commandsMu.RUnlock()
	if !open {
		t.Fatal("CarouselFilePreview.Open = false, want preserved while quick filter is active")
	}
}

func TestReconcileCarouselFilePreviewUpdatesToMatchDuringQuickFilter(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha.txt")
	beta := filepath.Join(root, "beta.txt")
	if err := os.WriteFile(alpha, []byte("alpha body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(beta, []byte("beta body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 200, 30)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0
	app.model.HideInactivePanel = true
	app.model.Primary.CarouselMode = true
	if !app.model.Primary.SelectVisibleEntry("alpha.txt") {
		t.Fatal("alpha.txt not found")
	}
	app.reconcileCarouselFilePreview()

	// Fuzzy-filter to beta.txt; the inline preview should follow the matched file.
	app.model.Primary.OpenFilter(app.activeViewportRows())
	for _, r := range "beta" {
		app.model.Primary.AppendFilterRune(r, app.activeViewportRows())
	}
	app.reconcileCarouselFilePreview()

	app.commandsMu.RLock()
	open := app.model.CarouselFilePreview.Open
	path := app.model.CarouselFilePreview.Path
	app.commandsMu.RUnlock()
	if !open || path != beta {
		t.Fatalf("CarouselFilePreview during filter: open=%v path=%q, want open=true path=%q", open, path, beta)
	}
}

func TestReconcileQuickViewPreviewUpdatesToMatchDuringQuickFilter(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha.txt")
	beta := filepath.Join(root, "beta.txt")
	if err := os.WriteFile(alpha, []byte("alpha body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(beta, []byte("beta body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 200, 30)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0
	app.model.ActivePanel = ui.PrimaryPanel
	app.model.QuickViewEnabled = true
	app.model.QuickViewPanel = ui.PrimaryPanel
	if !app.model.Primary.SelectVisibleEntry("alpha.txt") {
		t.Fatal("alpha.txt not found")
	}
	app.reconcileQuickViewPreview()

	// Fuzzy-filter to beta.txt; the quick-view preview should follow the matched file.
	app.model.Primary.OpenFilter(app.activeViewportRows())
	for _, r := range "beta" {
		app.model.Primary.AppendFilterRune(r, app.activeViewportRows())
	}
	app.reconcileQuickViewPreview()

	app.commandsMu.RLock()
	open := app.model.FilePreview.Open
	path := app.model.FilePreview.Path
	app.commandsMu.RUnlock()
	if !open || path != beta {
		t.Fatalf("QuickView preview during filter: open=%v path=%q, want open=true path=%q", open, path, beta)
	}
}

func TestReconcileCarouselFilePreviewOpensImmediatelyFromDirectory(t *testing.T) {
	root := t.TempDir()
	scroll := filepath.Join(root, "scroll.txt")
	if err := os.WriteFile(scroll, []byte("river delta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "child.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 200, 30)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 80 // debounce enabled (production default)
	app.model.HideInactivePanel = true
	app.model.Primary.CarouselMode = true

	// Cursor on the directory: no file preview.
	if !app.model.Primary.SelectVisibleEntry("nested") {
		t.Fatal("nested not found")
	}
	app.reconcileCarouselFilePreview()
	app.commandsMu.RLock()
	openOnDir := app.model.CarouselFilePreview.Open
	app.commandsMu.RUnlock()
	if openOnDir {
		t.Fatal("CarouselFilePreview.Open = true on directory cursor, want closed")
	}

	// Move onto a file the way list navigation does (debounce coalesce armed).
	if !app.model.Primary.SelectVisibleEntry("scroll.txt") {
		t.Fatal("scroll.txt not found")
	}
	app.carouselPreviewNavSkipSnapshot.Store(true)
	app.reconcileCarouselFilePreview()

	// Opening from a directory must apply immediately (no debounce flush needed),
	// otherwise the child column blanks for the debounce interval (flicker).
	app.commandsMu.RLock()
	open := app.model.CarouselFilePreview.Open
	path := app.model.CarouselFilePreview.Path
	app.commandsMu.RUnlock()
	if !open || path != scroll {
		t.Fatalf("CarouselFilePreview after dir->file: open=%v path=%q, want open=true path=%q", open, path, scroll)
	}
	if app.carouselPreviewNavSkipSnapshot.Load() {
		t.Fatal("carouselPreviewNavSkipSnapshot still set; pending debounce was not cleared")
	}
}

func TestCarouselPreviewPageScrollWithCtrlJK(t *testing.T) {
	root := t.TempDir()
	scroll := filepath.Join(root, "scroll.txt")
	if err := os.WriteFile(scroll, []byte("river delta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 200, 30)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0
	app.model.HideInactivePanel = true
	app.model.Primary.CarouselMode = true
	if !app.model.Primary.SelectVisibleEntry("scroll.txt") {
		t.Fatal("scroll.txt not found")
	}

	app.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = strings.Repeat("line\n", 200)
		st.Scroll = 0
	})

	app.dispatch(keymap.ActionFileQuickViewPreviewPageDown)
	if app.model.CarouselFilePreview.Scroll < 1 {
		t.Fatalf("CarouselFilePreview.Scroll = %d, want > 0 after preview page down", app.model.CarouselFilePreview.Scroll)
	}
	scrollAfterDown := app.model.CarouselFilePreview.Scroll

	app.dispatch(keymap.ActionFileQuickViewPreviewPageUp)
	if app.model.CarouselFilePreview.Scroll >= scrollAfterDown {
		t.Fatalf("CarouselFilePreview.Scroll = %d, want < %d after preview page up", app.model.CarouselFilePreview.Scroll, scrollAfterDown)
	}
}

// Regression: with a fit-to-content split ("<33%") whose parent/center columns hold only short
// names, the carousel file preview's text width must reflect the actual measured (narrow) column
// widths, not the unmeasured 33%-cap worst case — otherwise the preview pre-wraps (markdown in
// particular) far narrower than the column it's actually painted into.
func TestCarouselChildPreviewLayoutMetricsUsesMeasuredFitWidth(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "doc.txt"), []byte("body\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 300, 30)
	app := newApp(t, screen, work)
	app.model.HideInactivePanel = true
	app.model.Primary.CarouselMode = true
	if !app.model.Primary.SelectVisibleEntry("doc.txt") {
		t.Fatal("doc.txt not found")
	}

	layout, err := panelcarousel.ParseLayout([]string{"<33%", "<33%", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.model.CarouselLayout = layout

	rect, ok := app.activePanelFileColumnRect()
	if !ok {
		t.Fatal("activePanelFileColumnRect: not ok")
	}
	worstCase := panelcarousel.ChildColumnWidth(rect, layout)

	tw, _, ok := app.carouselChildPreviewLayoutMetrics()
	if !ok {
		t.Fatal("carouselChildPreviewLayoutMetrics: not ok")
	}
	if tw+2 <= worstCase {
		t.Fatalf("measured child text width+2 = %d, want > unmeasured worst-case width %d "+
			"(short parent/center names should measure well under the 33%% cap)", tw+2, worstCase)
	}
}
