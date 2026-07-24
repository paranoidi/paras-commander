package preview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func writeFileForPreviewWarm(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOverlayQuickViewInactiveDrawTitleTracksCursor(t *testing.T) {
	h, fh := newTestHandler(t, 80, 24)
	root := t.TempDir()

	readme := filepath.Join(root, "readme.md")
	writeFileForPreviewWarm(t, readme)
	notes := filepath.Join(root, "notes.txt")
	writeFileForPreviewWarm(t, notes)

	h.model.Primary = panel.State{Path: pathloc.MustParse(root)}
	if err := h.model.Primary.Load(root); err != nil {
		t.Fatal(err)
	}
	notesIdx := -1
	for i, e := range h.model.Primary.Entries {
		if e.Name == "notes.txt" {
			notesIdx = i
			break
		}
	}
	if notesIdx < 0 {
		t.Fatal("notes.txt not in listing")
	}
	h.model.Primary.Cursor = notesIdx
	h.model.ActivePanel = ui.PrimaryPanel
	h.model.QuickViewEnabled = true
	h.model.QuickViewPanel = ui.PrimaryPanel
	fh.inactive = ui.SecondaryPanel

	h.mu.Lock()
	h.model.FilePreview = ui.FilePreviewState{
		Open:         true,
		Phase:        ui.FilePreviewPhaseDone,
		Path:         readme,
		TitleBase:    "readme.md",
		CombinedText: "old body",
	}
	h.mu.Unlock()

	h.SnapshotPreviewDrawStates()

	draw := h.model.FilePreviewDraw
	if draw.TitleBase != "notes.txt" {
		t.Fatalf("FilePreviewDraw.TitleBase = %q, want notes.txt from cursor", draw.TitleBase)
	}
	if draw.CombinedText != "old body" {
		t.Fatalf("FilePreviewDraw.CombinedText = %q, want held body until debounced reload", draw.CombinedText)
	}
}

func TestOverlayQuickViewInactiveDrawTitleSkipsDirectoryOverlay(t *testing.T) {
	h, _ := newTestHandler(t, 80, 24)

	h.model.ActivePanel = ui.PrimaryPanel
	h.model.QuickViewEnabled = true
	h.model.QuickViewPanel = ui.PrimaryPanel
	h.model.QuickViewDirOverlayActive = true
	h.model.QuickViewDirOverlayPanelID = ui.SecondaryPanel

	h.mu.Lock()
	h.model.FilePreview = ui.FilePreviewState{
		Open:      true,
		TitleBase: "stale.txt",
	}
	h.mu.Unlock()

	h.SnapshotPreviewDrawStates()

	if h.model.FilePreviewDraw.TitleBase != "stale.txt" {
		t.Fatalf("TitleBase = %q, want stale draw title when directory overlay is active", h.model.FilePreviewDraw.TitleBase)
	}
}

func TestOverlayQuickViewInactiveDrawTitleFromFileEntry(t *testing.T) {
	h, fh := newTestHandler(t, 80, 24)
	root := t.TempDir()

	readme := filepath.Join(root, "readme.md")
	writeFileForPreviewWarm(t, readme)

	h.model.Primary = panel.State{Path: pathloc.MustParse(root)}
	if err := h.model.Primary.Load(root); err != nil {
		t.Fatal(err)
	}
	entry, ok := h.model.Primary.CurrentEntry()
	if !ok || entry.Type != localfs.EntryFile {
		t.Fatalf("current entry = %+v, want file", entry)
	}

	h.model.ActivePanel = ui.PrimaryPanel
	h.model.QuickViewEnabled = true
	h.model.QuickViewPanel = ui.PrimaryPanel
	fh.inactive = ui.SecondaryPanel

	h.mu.Lock()
	h.model.FilePreview = ui.FilePreviewState{Open: false}
	h.mu.Unlock()

	h.SnapshotPreviewDrawStates()

	if !h.model.FilePreviewDraw.Open {
		t.Fatal("FilePreviewDraw.Open = false, want true for quick-view file selection")
	}
	if h.model.FilePreviewDraw.TitleBase != "readme.md" {
		t.Fatalf("FilePreviewDraw.TitleBase = %q, want readme.md", h.model.FilePreviewDraw.TitleBase)
	}
}
