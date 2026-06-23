package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestPathPickerAcceptLongCompletionScrollsToEnd(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "exists")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("m : "+real+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	app.openBookmarkDialog()
	st := &app.model.PathPicker

	long := "/very/long/path/with/many/segments/that/exceeds/the/visible/picker/input/width/value"
	st.Query = long
	st.QueryCursor = len([]rune(long))
	st.QueryCompletionSuffix = "EXTRA"
	st.QueryCompletionIsDir = false
	st.QueryScroll = 0

	app.acceptPathPickerCompletion()
	wantCursor := len([]rune(long + "EXTRA"))
	if st.QueryCursor != wantCursor {
		t.Fatalf("cursor = %d want %d", st.QueryCursor, wantCursor)
	}
	width := app.pathPickerQueryWidth()
	_, wantScroll := ui.EnsureScrollInputVisible(wantCursor, wantCursor, 0, width)
	if st.QueryScroll <= 0 {
		t.Fatalf("QueryScroll = %d want > 0 for long accepted path", st.QueryScroll)
	}
	if st.QueryScroll > wantScroll {
		t.Fatalf("QueryScroll = %d want <= %d for tail visibility", st.QueryScroll, wantScroll)
	}

	app.handlePathPickerKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if st.QueryCursor != wantCursor {
		t.Fatalf("after End cursor = %d want %d", st.QueryCursor, wantCursor)
	}
	_, wantScroll = ui.EnsureScrollInputVisible(wantCursor, wantCursor, st.QueryScroll, width)
	if st.QueryScroll != wantScroll {
		t.Fatalf("after End scroll = %d want %d", st.QueryScroll, wantScroll)
	}
}

func TestPathPickerItemsSkipMissingHistoryPaths(t *testing.T) {
	root := t.TempDir()
	exists := filepath.Join(root, "exists")
	if err := os.MkdirAll(exists, 0o755); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(root, "gone")
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("m : "+exists+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	app.model.Primary.History = []string{gone, exists}
	app.model.Secondary.History = nil
	items, err := app.pathPickerItemsHistoryAndBookmarks()
	if err != nil {
		t.Fatal(err)
	}
	cleanExists := filepath.Clean(exists)
	cleanGone := filepath.Clean(gone)
	foundExists := false
	for _, it := range items {
		if it.Path == cleanGone {
			t.Fatalf("missing path %q should be filtered from picker items", gone)
		}
		if it.Path == cleanExists {
			foundExists = true
			if it.Source != "history" {
				t.Fatalf("history item Source = %q, want history", it.Source)
			}
			if it.Name != "" {
				t.Fatalf("history item Name = %q, want empty", it.Name)
			}
		}
	}
	if !foundExists {
		t.Fatalf("expected %q in picker items, got %d items", exists, len(items))
	}
}
