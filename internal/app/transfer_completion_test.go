package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestTransferDialogPrefillDestinationTrailingSlash(t *testing.T) {
	root := t.TempDir()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: config.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	if err := app.inactivePanel().Load(root); err != nil {
		t.Fatal(err)
	}

	app.openCopyDialog()
	dest := app.model.TransferDialog.Destination.Value
	if dest == "" || dest[len(dest)-1] != '/' {
		t.Fatalf("destination prefill = %q, want trailing %q", dest, filepath.Separator)
	}
}

func TestTransferDialogTabAcceptsFilesystemCompletion(t *testing.T) {
	root := t.TempDir()
	fooDir := filepath.Join(root, "foo")
	if err := os.MkdirAll(fooDir, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	cfg := config.Default()
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	for _, open := range []func(){app.openCopyDialog, app.openMoveDialog} {
		open()
		d := &app.model.TransferDialog
		if !d.Open {
			t.Fatal("transfer dialog should be open")
		}
		prefix := filepath.Join(root, "f")
		d.Destination = ui.FileDialogField{
			Value:  prefix,
			Cursor: len([]rune(prefix)),
		}
		d.DestSubFocus = ui.TransferDestSubFocusText
		d.FocusField = 0
		app.syncPathFieldCompletion(&d.Destination, app.transferDestinationTextWidth())
		if d.Destination.CompletionSuffix != "oo" {
			t.Fatalf("suffix = %q want oo", d.Destination.CompletionSuffix)
		}

		app.handleTransferDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
		want := filepath.Join(root, "foo") + "/"
		if d.Destination.Value != want {
			t.Fatalf("Value = %q want %q", d.Destination.Value, want)
		}
		if d.Destination.Cursor != len([]rune(want)) {
			t.Fatalf("Cursor = %d want %d", d.Destination.Cursor, len([]rune(want)))
		}
		app.closeTransferDialog()
	}
}
