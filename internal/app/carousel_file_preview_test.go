package app

import (
	"os"
	"path/filepath"
	"testing"

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
