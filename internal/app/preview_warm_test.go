package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestOverlayQuickViewInactiveDrawTitleTracksCursor(t *testing.T) {
	screen := newScreen(t, 80, 24)
	root := t.TempDir()
	app := newApp(t, screen, root)

	readme := filepath.Join(root, "readme.md")
	if err := os.WriteFile(readme, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	notes := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(notes, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	app.model.Primary = panel.State{Path: pathloc.MustParse(root)}
	if err := app.model.Primary.Load(root); err != nil {
		t.Fatal(err)
	}
	notesIdx := -1
	for i, e := range app.model.Primary.Entries {
		if e.Name == "notes.txt" {
			notesIdx = i
			break
		}
	}
	if notesIdx < 0 {
		t.Fatal("notes.txt not in listing")
	}
	app.model.Primary.Cursor = notesIdx
	app.model.ActivePanel = ui.PrimaryPanel
	app.model.QuickViewEnabled = true
	app.model.QuickViewPanel = ui.PrimaryPanel

	app.commandsMu.Lock()
	app.model.FilePreview = ui.FilePreviewState{
		Open:         true,
		Phase:        ui.FilePreviewPhaseDone,
		Path:         readme,
		TitleBase:    "readme.md",
		CombinedText: "old body",
	}
	app.commandsMu.Unlock()

	app.snapshotPreviewDrawStates()

	draw := app.model.FilePreviewDraw
	if draw.TitleBase != "notes.txt" {
		t.Fatalf("FilePreviewDraw.TitleBase = %q, want notes.txt from cursor", draw.TitleBase)
	}
	if draw.CombinedText != "old body" {
		t.Fatalf("FilePreviewDraw.CombinedText = %q, want held body until debounced reload", draw.CombinedText)
	}
}

func TestOverlayQuickViewInactiveDrawTitleSkipsDirectoryOverlay(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())

	app.model.ActivePanel = ui.PrimaryPanel
	app.model.QuickViewEnabled = true
	app.model.QuickViewPanel = ui.PrimaryPanel
	app.model.QuickViewDirOverlayActive = true
	app.model.QuickViewDirOverlayPanelID = ui.SecondaryPanel

	app.commandsMu.Lock()
	app.model.FilePreview = ui.FilePreviewState{
		Open:      true,
		TitleBase: "stale.txt",
	}
	app.commandsMu.Unlock()

	app.snapshotPreviewDrawStates()

	if app.model.FilePreviewDraw.TitleBase != "stale.txt" {
		t.Fatalf("TitleBase = %q, want stale draw title when directory overlay is active", app.model.FilePreviewDraw.TitleBase)
	}
}

func writeFileForPreviewWarm(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOverlayQuickViewInactiveDrawTitleFromFileEntry(t *testing.T) {
	screen := newScreen(t, 80, 24)
	root := t.TempDir()
	app := newApp(t, screen, root)

	readme := filepath.Join(root, "readme.md")
	writeFileForPreviewWarm(t, readme)

	app.model.Primary = panel.State{Path: pathloc.MustParse(root)}
	if err := app.model.Primary.Load(root); err != nil {
		t.Fatal(err)
	}
	entry, ok := app.model.Primary.CurrentEntry()
	if !ok || entry.Type != localfs.EntryFile {
		t.Fatalf("current entry = %+v, want file", entry)
	}
	_ = entry

	app.model.ActivePanel = ui.PrimaryPanel
	app.model.QuickViewEnabled = true
	app.model.QuickViewPanel = ui.PrimaryPanel

	app.commandsMu.Lock()
	app.model.FilePreview = ui.FilePreviewState{Open: false}
	app.commandsMu.Unlock()

	app.snapshotPreviewDrawStates()

	if !app.model.FilePreviewDraw.Open {
		t.Fatal("FilePreviewDraw.Open = false, want true for quick-view file selection")
	}
	if app.model.FilePreviewDraw.TitleBase != "readme.md" {
		t.Fatalf("FilePreviewDraw.TitleBase = %q, want readme.md", app.model.FilePreviewDraw.TitleBase)
	}
}
