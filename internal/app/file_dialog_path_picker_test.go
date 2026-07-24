package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestSymlinkDialogRightAtEndFocusesPathPickerGlyph(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"))

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
	p := app.activePanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == "a.txt" {
			p.Cursor = i
			break
		}
	}

	app.dispatch(keymap.ActionFileSymlink)
	f := app.dialogCtrl.FocusedField()
	if f == nil || !f.PathPicker {
		t.Fatal("symlink target field should have PathPicker enabled")
	}
	if f.PickerFocused {
		t.Fatal("picker glyph should not be focused initially")
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if !f.PickerFocused {
		t.Fatal("Right at end should focus path-picker glyph")
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if f.PickerFocused {
		t.Fatal("Left from glyph should return focus to text")
	}
}

func TestSymlinkDialogTabAcceptsFilesystemCompletion(t *testing.T) {
	root := t.TempDir()
	fooDir := filepath.Join(root, "foo")
	if err := os.MkdirAll(fooDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.txt"))

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
	p := app.activePanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == "a.txt" {
			p.Cursor = i
			break
		}
	}

	app.dispatch(keymap.ActionFileSymlink)
	f := app.dialogCtrl.FocusedField()
	if f == nil {
		t.Fatal("symlink target field missing")
	}

	prefix := filepath.Join(root, "f")
	f.Value = prefix
	f.Cursor = len([]rune(prefix))
	app.syncPathFieldCompletion(f, app.transferDestinationTextWidth())
	if f.CompletionSuffix != "oo" {
		t.Fatalf("suffix = %q want oo", f.CompletionSuffix)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	want := filepath.Join(root, "foo") + "/"
	if f.Value != want {
		t.Fatalf("Value = %q want %q", f.Value, want)
	}
}

func TestSymlinkDialogOpensPathPickerFromGlyph(t *testing.T) {
	root := t.TempDir()
	dst := filepath.Join(root, "dst")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.txt"))
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("m : "+dst+"\n"), 0o644); err != nil {
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
	p := app.activePanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == "a.txt" {
			p.Cursor = i
			break
		}
	}

	app.dispatch(keymap.ActionFileSymlink)
	f := app.dialogCtrl.FocusedField()
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if !f.PickerFocused {
		t.Fatal("picker glyph should be focused")
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.model.PathPicker.Open || app.model.PathPicker.Purpose != dialog.PathPickerPurposeApplyFileDialogField {
		t.Fatalf("path picker = open %v purpose %v, want ApplyFileDialogField",
			app.model.PathPicker.Open, app.model.PathPicker.Purpose)
	}
}
