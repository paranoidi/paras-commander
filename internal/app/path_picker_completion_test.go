package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	app.dialogCtrl.OpenBookmarkDialog()
	st := &app.model.PathPicker

	long := "/very/long/path/with/many/segments/that/exceeds/the/visible/picker/input/width/value"
	longRunes := []rune(long)
	st.Query = long
	st.QueryCursor = len(longRunes)
	partial := "value"
	start := len(longRunes) - len([]rune(partial))
	st.Completion.Set(long, start, []pathpick.Candidate{{Name: partial + "EXTRA"}})
	st.QueryScroll = 0

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	wantCursor := len([]rune(long + "EXTRA"))
	if st.QueryCursor != wantCursor {
		t.Fatalf("cursor = %d want %d", st.QueryCursor, wantCursor)
	}
	width := app.dialogCtrl.PathPickerQueryWidth()
	_, wantScroll := dialog.EnsureScrollInputVisible(wantCursor, wantCursor, 0, width)
	if st.QueryScroll <= 0 {
		t.Fatalf("QueryScroll = %d want > 0 for long accepted path", st.QueryScroll)
	}
	if st.QueryScroll > wantScroll {
		t.Fatalf("QueryScroll = %d want <= %d for tail visibility", st.QueryScroll, wantScroll)
	}

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if st.QueryCursor != wantCursor {
		t.Fatalf("after End cursor = %d want %d", st.QueryCursor, wantCursor)
	}
	_, wantScroll = dialog.EnsureScrollInputVisible(wantCursor, wantCursor, st.QueryScroll, width)
	if st.QueryScroll != wantScroll {
		t.Fatalf("after End scroll = %d want %d", st.QueryScroll, wantScroll)
	}
}

// TestPathPickerItemsHistoryIncludesMissingPaths verifies that a missing history path is kept
// (not filtered out) with PathMissing starting false — the async scan started separately by
// startPathPickerMissingScan is what later flags it, so building the item list itself must
// never stat anything.
func TestPathPickerItemsHistoryIncludesMissingPaths(t *testing.T) {
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
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	app.model.Primary.History = []string{gone, exists}
	app.model.Secondary.History = nil
	items, err := app.dialogCtrl.PathPickerItemsHistory()
	if err != nil {
		t.Fatal(err)
	}
	cleanExists := filepath.Clean(exists)
	cleanGone := filepath.Clean(gone)
	foundExists, foundGone := false, false
	for _, it := range items {
		if it.PathMissing {
			t.Fatalf("item %+v: PathMissing true, want false (filled in later by the async scan)", it)
		}
		if it.Path == cleanGone {
			foundGone = true
		}
		if it.Path == cleanExists {
			foundExists = true
			if it.Name != "" {
				t.Fatalf("history item Name = %q, want empty", it.Name)
			}
		}
	}
	if !foundExists {
		t.Fatalf("expected %q in picker items, got %d items", exists, len(items))
	}
	if !foundGone {
		t.Fatalf("expected missing path %q to still be present in picker items, got %d items", gone, len(items))
	}
}

func TestPathPickerCreateDirRow(t *testing.T) {
	root := t.TempDir()
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("m : "+root+"\n"), 0o644); err != nil {
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
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	app.dialogCtrl.OpenBookmarkDialog()
	st := &app.model.PathPicker

	target := filepath.Join(root, "copper", "kettle")
	st.Query = target + "/"
	st.QueryCursor = len([]rune(st.Query))
	app.dialogCtrl.SyncPathPickerCompletion()
	if !st.Completion.Open || len(st.Completion.Items) != 1 || !st.Completion.Items[0].Create {
		t.Fatalf("want create row, got open=%v items=%+v", st.Completion.Open, st.Completion.Items)
	}

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if fi, err := os.Stat(target); err != nil || !fi.IsDir() {
		t.Fatalf("directory not created: %v", err)
	}
	if !st.Open {
		t.Fatal("picker should stay open after creating")
	}
	if st.Query != target+"/" {
		t.Fatalf("query changed to %q", st.Query)
	}
	if len(st.Completion.Items) != 0 {
		t.Fatalf("create row should be gone, got %+v", st.Completion.Items)
	}
	if !app.dialogCtrl.PathPickerValidateArmed() {
		t.Fatal("validation should be re-armed")
	}
}
