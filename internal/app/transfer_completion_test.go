package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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

	app.dialogCtrl.OpenCopyDialog()
	dest := app.model.TransferDialog.Destination.Value
	if dest == "" || dest[len(dest)-1] != '/' {
		t.Fatalf("destination prefill = %q, want trailing %q", dest, filepath.Separator)
	}
}

func TestTransferDialogCopyPreserveAltAndFocusedShortcuts(t *testing.T) {
	root := t.TempDir()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	cfg := config.Default()
	cfg.Operations.PreservePermissions = true
	cfg.Operations.PreserveTimestamps = true
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}

	app.dialogCtrl.OpenCopyDialog()
	d := &app.model.TransferDialog
	if !d.Open || d.Kind != dialog.TransferKindCopy {
		t.Fatal("copy dialog should be open")
	}
	if !d.PreservePermissions || !d.PreserveTimestamps {
		t.Fatal("expected preserve options from config defaults")
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	if d.PreservePermissions {
		t.Fatal("Alt+R should toggle preserve permissions off")
	}
	if !d.PreserveTimestamps {
		t.Fatal("Alt+T should not affect timestamps yet")
	}

	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModAlt))
	if d.PreserveTimestamps {
		t.Fatal("Alt+T should toggle preserve timestamps off")
	}

	d.FocusField = 1
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))
	if !d.PreservePermissions {
		t.Fatal("r on focused permissions row should toggle on")
	}

	d.FocusField = 2
	app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
	if !d.PreserveTimestamps {
		t.Fatal("t on focused timestamps row should toggle on")
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

	for _, open := range []func(){app.dialogCtrl.OpenCopyDialog, app.dialogCtrl.OpenMoveDialog} {
		open()
		d := &app.model.TransferDialog
		if !d.Open {
			t.Fatal("transfer dialog should be open")
		}
		prefix := filepath.Join(root, "f")
		d.Destination = dialog.FileDialogField{
			Value:  prefix,
			Cursor: len([]rune(prefix)),
		}
		d.DestSubFocus = dialog.TransferDestSubFocusText
		d.FocusField = 0
		app.dialogCtrl.SyncPathFieldCompletion(&d.Destination, app.dialogCtrl.TransferDestinationTextWidth())
		if d.Destination.CompletionSuffix != "oo" {
			t.Fatalf("suffix = %q want oo", d.Destination.CompletionSuffix)
		}

		app.dialogCtrl.HandleTransferDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
		want := filepath.Join(root, "foo") + "/"
		if d.Destination.Value != want {
			t.Fatalf("Value = %q want %q", d.Destination.Value, want)
		}
		if d.Destination.Cursor != len([]rune(want)) {
			t.Fatalf("Cursor = %d want %d", d.Destination.Cursor, len([]rune(want)))
		}
		app.dialogCtrl.CloseTransferDialog()
	}
}
