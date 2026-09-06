package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
)

func TestPathPickerBackspaceRevealWhenValueFitsWithGhostSuffix(t *testing.T) {
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

	query := "/synthetic/volume/catalog/branches/widget/12"
	runes := []rune(query)
	st.Query = query
	st.QueryCursor = len(runes)
	st.QueryScroll = len(runes) - 2
	st.QueryCompletionSuffix = "documentary.riverbank.mountain.catalog.release" // long ghost tail

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone)) // drop '2'
	if st.QueryScroll != 0 {
		t.Fatalf("QueryScroll = %d want 0; committed path fits field width", st.QueryScroll)
	}
	if st.Query != "/synthetic/volume/catalog/branches/widget/1" {
		t.Fatalf("after first backspace Query = %q", st.Query)
	}

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone)) // drop '1'
	if st.QueryScroll != 0 {
		t.Fatalf("QueryScroll = %d want 0", st.QueryScroll)
	}
	if st.Query != "/synthetic/volume/catalog/branches/widget/" {
		t.Fatalf("Query = %q", st.Query)
	}
}

func TestPathPickerBackspaceRevealScrollAtLeftOverflowMarker(t *testing.T) {
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

	query := "/very/long/path/with/many/segments/that/exceeds/the/visible/picker/input/width/~/synthetic/workspace/catalog/"
	for _, r := range query {
		app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.dialogCtrl.SyncPathPickerScroll()
	if st.QueryScroll == 0 {
		t.Fatal("expected scrolled query")
	}

	// Cursor just right of first visible rune (◀ marks hidden prefix to the left).
	st.QueryCursor = st.QueryScroll + 1
	scrollBefore := st.QueryScroll

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if st.QueryScroll >= scrollBefore {
		t.Fatalf("QueryScroll = %d want < %d after erasing first visible rune", st.QueryScroll, scrollBefore)
	}
}
