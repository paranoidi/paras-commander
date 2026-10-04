package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
)

func TestExtractDestinationShortcutsAndFooter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	activeDir := filepath.Join(dir, "active")
	inactiveDir := filepath.Join(dir, "inactive")
	for _, p := range []string{activeDir, inactiveDir} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	archive := filepath.Join(activeDir, "orchard.zip")
	if err := os.WriteFile(archive, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.activePanel().Load(activeDir); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	if err := app.inactivePanel().Load(inactiveDir); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	app.activePanel().SelectedPaths = map[string]bool{archive: true}
	app.dialogCtrl.OpenExtractDialog(app.activePanel())
	if !app.model.FileDialog.Open {
		t.Fatal("extract dialog should be open")
	}

	keys := app.activeFooterKeys()
	if !footerHasHint(keys, "Active path ◄", "S-left") || !footerHasHint(keys, "Inactive path ►", "S-right") {
		t.Fatalf("footer = %+v, want Active/Inactive path hints", keys)
	}
	dest := func() string { return app.model.FileDialog.Fields[0].Value }
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModShift))
	if want := dialogctrl.TransferPrefilledDestination(activeDir).Value; dest() != want {
		t.Fatalf("after Shift+Left destination = %q, want %q", dest(), want)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	if want := dialogctrl.TransferPrefilledDestination(inactiveDir).Value; dest() != want {
		t.Fatalf("after Shift+Right destination = %q, want %q", dest(), want)
	}
}
