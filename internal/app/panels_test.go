package app

import (
	"os"
	"path/filepath"
	"testing"

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

	app.refreshBothPanels()

	want := filepath.Clean(parent)
	if got := app.inactivePanel().PathString(); got != want {
		t.Fatalf("inactive path = %q, want %q", got, want)
	}
	if got := app.activePanel().PathString(); got != want {
		t.Fatalf("active path = %q, want unchanged %q", got, want)
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

	app.model.ActivePanel = ui.LeftPanel
	left := app.panelByID(ui.LeftPanel)
	selectPanelEntryByName(t, left, "alpha")
	app.model.QuickViewEnabled = true
	app.model.QuickViewPanel = ui.LeftPanel
	app.applyQuickViewPreviewImmediately()
	if got := filepath.Clean(app.model.QuickViewDirOverlay.Path.String()); got != filepath.Clean(alpha) {
		t.Fatalf("overlay path = %q, want %q", got, alpha)
	}

	if err := os.RemoveAll(alpha); err != nil {
		t.Fatal(err)
	}
	app.refreshBothPanels()

	if !app.model.QuickViewDirOverlayActive {
		t.Fatal("quick view overlay should stay active after delete refresh")
	}
	if got := filepath.Clean(app.model.QuickViewDirOverlay.Path.String()); got != filepath.Clean(beta) {
		t.Fatalf("overlay path = %q, want next highlight %q", got, beta)
	}
}
