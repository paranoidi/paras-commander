package dialog

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestBookmarksPickerMissingScanAppliesAndIgnoresStaleGen(t *testing.T) {
	root := t.TempDir()
	// Isolate from the real user's GTK bookmarks file (bookmarks.LoadAll merges it in).
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	existing := filepath.Join(root, "hedgehog", "lantern")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "wandering", "turnip")

	marksPath := filepath.Join(root, "marks")
	lines := fmt.Sprintf("herone : %s\nhertwo : %s\n", existing, missing)
	if err := os.WriteFile(marksPath, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(120, 40)

	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	fh := &fakeMassRenamePatternHost{cfg: cfg}
	h := &Handler{
		host:   fh,
		screen: screen,
		model:  &ui.Model{},
	}

	h.OpenBookmarkDialog()
	if !h.model.PathPicker.Open {
		t.Fatal("expected path picker open")
	}
	if len(h.model.PathPicker.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(h.model.PathPicker.Items))
	}
	for _, it := range h.model.PathPicker.Items {
		if it.PathMissing {
			t.Fatalf("item %+v: PathMissing true right after open, want false (scan is async)", it)
		}
	}

	currentGen := h.pathPickerMissingGen

	// Stale gen: must be ignored even though it would flip the existing path missing.
	h.ApplyPathPickerMissing(PathsMissingPayload{
		Target: "picker",
		Gen:    currentGen - 1,
		Missing: map[string]bool{
			filepath.Clean(existing): true,
		},
	})
	for _, it := range h.model.PathPicker.Items {
		if it.Path == filepath.Clean(existing) && it.PathMissing {
			t.Fatal("stale-gen payload was applied, want ignored")
		}
	}

	// Matching gen: the nonexistent path flips to missing, the existing one stays not-missing.
	h.ApplyPathPickerMissing(PathsMissingPayload{
		Target: "picker",
		Gen:    currentGen,
		Missing: map[string]bool{
			filepath.Clean(existing): false,
			filepath.Clean(missing):  true,
		},
	})
	for _, it := range h.model.PathPicker.Items {
		want := it.Path == filepath.Clean(missing)
		if it.PathMissing != want {
			t.Errorf("item %q: PathMissing = %v, want %v", it.Path, it.PathMissing, want)
		}
	}
}
