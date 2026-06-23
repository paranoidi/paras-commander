package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

func TestChooserModeEnablesQuickViewByDefault(t *testing.T) {
	root := t.TempDir()
	screen := uitest.Screen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:         func() (string, error) { return root, nil },
		Config:      config.Default(),
		ChooserFile: filepath.Join(t.TempDir(), "out"),
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	if app.model.HideInactivePanel {
		t.Fatal("HideInactivePanel = true, want false in chooser mode")
	}
	if !app.model.QuickViewEnabled {
		t.Fatal("QuickViewEnabled = false, want true in chooser mode")
	}
	if app.model.QuickViewPanel != app.model.ActivePanel {
		t.Fatalf("QuickViewPanel = %d, want active panel %d", app.model.QuickViewPanel, app.model.ActivePanel)
	}
}

func TestChooserModeEnablesCarouselByDefault(t *testing.T) {
	root := t.TempDir()
	screen := uitest.Screen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:         func() (string, error) { return root, nil },
		Config:      config.Default(),
		ChooserFile: filepath.Join(t.TempDir(), "out"),
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	if !app.model.Primary.CarouselMode {
		t.Fatal("Left.CarouselMode = false, want true by default in chooser mode")
	}
}

func TestChooserNoCarouselDisablesPrimaryPanelCarousel(t *testing.T) {
	root := t.TempDir()
	screen := uitest.Screen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:               func() (string, error) { return root, nil },
		Config:            config.Default(),
		ChooserFile:       filepath.Join(t.TempDir(), "out"),
		ChooserNoCarousel: true,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	if app.model.Primary.CarouselMode {
		t.Fatal("Left.CarouselMode = true, want false with ChooserNoCarousel")
	}
}

func TestChooserSelectHighlightsFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "buffer.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	screen := uitest.Screen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:           func() (string, error) { return root, nil },
		Config:        config.Default(),
		ChooserFile:   filepath.Join(t.TempDir(), "out"),
		ChooserSelect: file,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	entry, ok := app.model.Primary.CurrentEntry()
	if !ok {
		t.Fatal("no current entry")
	}
	if entry.Name != "buffer.go" {
		t.Fatalf("current entry = %q, want buffer.go", entry.Name)
	}
	if app.model.Primary.PathString() != root {
		t.Fatalf("panel path = %q, want %q", app.model.Primary.PathString(), root)
	}
}

func TestChooserSelectMissingFileOpensParent(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "scratch.go")
	screen := uitest.Screen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:           func() (string, error) { return root, nil },
		Config:        config.Default(),
		ChooserFile:   filepath.Join(t.TempDir(), "out"),
		ChooserSelect: missing,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	if app.model.Primary.PathString() != root {
		t.Fatalf("panel path = %q, want %q", app.model.Primary.PathString(), root)
	}
}

func TestChooserEnterWritesAndQuits(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "picked.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	chooserOut := filepath.Join(t.TempDir(), "chooser-out")
	screen := uitest.Screen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:         func() (string, error) { return root, nil },
		Config:      config.Default(),
		ChooserFile: chooserOut,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	if !app.model.Primary.SelectVisibleEntry("picked.txt") {
		t.Fatal("SelectVisibleEntry(picked.txt) = false")
	}
	if quit := app.handleNavOpen(&app.model.Primary, 10); !quit {
		t.Fatal("handleNavOpen = false, want quit")
	}
	data, err := os.ReadFile(chooserOut)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want, err := filepath.Abs(file)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if strings.TrimSpace(string(data)) != want {
		t.Fatalf("chooser file = %q, want %q", strings.TrimSpace(string(data)), want)
	}
}

func TestChooserEnterDirectoryDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "nested")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	chooserOut := filepath.Join(t.TempDir(), "chooser-out")
	screen := uitest.Screen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:         func() (string, error) { return root, nil },
		Config:      config.Default(),
		ChooserFile: chooserOut,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	if !app.model.Primary.SelectVisibleEntry("nested") {
		t.Fatal("SelectVisibleEntry(nested) = false")
	}
	if quit := app.handleNavOpen(&app.model.Primary, 10); quit {
		t.Fatal("handleNavOpen quit on directory")
	}
	if app.model.Primary.PathString() != sub {
		t.Fatalf("panel path = %q, want %q", app.model.Primary.PathString(), sub)
	}
	if _, err := os.Stat(chooserOut); err == nil {
		t.Fatal("chooser file created on directory enter")
	}
}

func TestChooserDispatchEnterQuits(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "open.me")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	chooserOut := filepath.Join(t.TempDir(), "chooser-out")
	screen := uitest.Screen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:         func() (string, error) { return root, nil },
		Config:      config.Default(),
		ChooserFile: chooserOut,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	if !app.model.Primary.SelectVisibleEntry("open.me") {
		t.Fatal("SelectVisibleEntry(open.me) = false")
	}
	quit, _ := app.finishResolvedKeyboardAction(keymap.ActionNavOpen)
	if !quit {
		t.Fatal("finishResolvedKeyboardAction quit = false, want true")
	}
}

func TestChooserQuitSkipsConfirm(t *testing.T) {
	screen := uitest.Screen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:         func() (string, error) { return t.TempDir(), nil },
		Config:      config.Default(),
		ChooserFile: filepath.Join(t.TempDir(), "out"),
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	if !app.handleQuit() {
		t.Fatal("handleQuit = false, want immediate quit in chooser mode")
	}
}

func TestWriteChooserSelection(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "nested", "choice")
	file := filepath.Join(t.TempDir(), "src.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := writeChooserSelection(out, file); err != nil {
		t.Fatalf("writeChooserSelection: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want, err := filepath.Abs(file)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if strings.TrimSpace(string(data)) != want {
		t.Fatalf("got %q, want %q", strings.TrimSpace(string(data)), want)
	}
}
