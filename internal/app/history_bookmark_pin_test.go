package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
)

func TestAltInsertPinsFromHistoryAndBookmarkDialogs(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "harbor", "lantern")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte(fmt.Sprintf("lantern : %s\n", target)), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	altIns := tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModAlt)

	app.dialogCtrl.OpenBookmarkDialog()
	app.handleKey(altIns)
	if !app.model.PathPicker.Open {
		t.Fatal("bookmark dialog closed on pin")
	}
	if len(app.model.PinnedItems) != 1 || app.model.PinnedItems[0].Path != filepath.Clean(target) {
		t.Fatalf("PinnedItems after bookmark pin = %+v", app.model.PinnedItems)
	}
	app.dialogCtrl.ClosePathPicker()

	// The current directory is hidden from History: leave root so it is listed.
	navigatePanelIntoDir(t, app, screen, app.model.ActivePanel, "harbor")
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModAlt))
	if !app.model.HistoryDialog.Open {
		t.Fatal("expected history dialog open")
	}
	app.handleKey(altIns)
	if !app.model.HistoryDialog.Open {
		t.Fatal("history dialog closed on pin")
	}
	if len(app.model.PinnedItems) != 2 {
		t.Fatalf("PinnedItems after history pin = %+v", app.model.PinnedItems)
	}
}
