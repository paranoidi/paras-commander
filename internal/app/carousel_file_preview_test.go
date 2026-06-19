package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
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
	app.model.Left.CarouselMode = true
	if !app.model.Left.SelectVisibleEntry("scroll.txt") {
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
	app.model.Left.CarouselMode = true
	app.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Path = filepath.Join(root, "stale.txt")
	})
	app.carouselFilePreviewLastFingerprint = "f:" + filepath.Join(root, "stale.txt")

	if !app.model.Left.SelectVisibleEntry("nested") {
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
	app.model.Left.CarouselMode = true
	if !app.model.Left.SelectVisibleEntry("scroll.txt") {
		t.Fatal("scroll.txt not found")
	}
	app.reconcileCarouselFilePreview()
	app.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.Path = scroll
		st.CombinedText = "river delta\n"
	})

	app.model.Left.OpenFilter(app.activeViewportRows())
	app.model.Left.AppendFilterRune('n', app.activeViewportRows()) // jumps to nested dir match
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
	app.model.Left.CarouselMode = true
	if !app.model.Left.SelectVisibleEntry("alpha.txt") {
		t.Fatal("alpha.txt not found")
	}
	app.reconcileCarouselFilePreview()

	// Fuzzy-filter to beta.txt; the inline preview should follow the matched file.
	app.model.Left.OpenFilter(app.activeViewportRows())
	for _, r := range "beta" {
		app.model.Left.AppendFilterRune(r, app.activeViewportRows())
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
	app.model.ActivePanel = ui.LeftPanel
	app.model.QuickViewEnabled = true
	app.model.QuickViewPanel = ui.LeftPanel
	if !app.model.Left.SelectVisibleEntry("alpha.txt") {
		t.Fatal("alpha.txt not found")
	}
	app.reconcileQuickViewPreview()

	// Fuzzy-filter to beta.txt; the quick-view preview should follow the matched file.
	app.model.Left.OpenFilter(app.activeViewportRows())
	for _, r := range "beta" {
		app.model.Left.AppendFilterRune(r, app.activeViewportRows())
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
	app.model.Left.CarouselMode = true

	// Cursor on the directory: no file preview.
	if !app.model.Left.SelectVisibleEntry("nested") {
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
	if !app.model.Left.SelectVisibleEntry("scroll.txt") {
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
	app.model.Left.CarouselMode = true
	if !app.model.Left.SelectVisibleEntry("scroll.txt") {
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
