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
	if err := app.focusedPanel().Load(activeDir); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	if err := app.otherBrowserPanel().Load(inactiveDir); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	app.focusedPanel().SelectedPaths = map[string]bool{archive: true}
	app.dialogCtrl.OpenExtractDialog(app.focusedPanel())
	if !app.model.FileDialog.Open {
		t.Fatal("extract dialog should be open")
	}

	keys := app.activeFooterKeys()
	if !footerHasHint(keys, "Active path ◄", "S-←") || !footerHasHint(keys, "Inactive path ►", "S-→") {
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

// A destination change must repaint the panel frames, not just the dialog.
func TestExtractDestinationShortcutRepaintsTargetFrames(t *testing.T) {
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
	if err := app.focusedPanel().Load(activeDir); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	if err := app.otherBrowserPanel().Load(inactiveDir); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)
	app.focusedPanel().SelectedPaths = map[string]bool{archive: true}
	app.dialogCtrl.OpenExtractDialog(app.focusedPanel())
	app.render()

	layout := app.layoutForTerminalSize(app.screen.Size())
	targetFG, _, _ := app.styles.PanelTargetFrame.Decompose()
	check := func(when string, wantPrimary, wantSecondary bool) {
		t.Helper()
		for _, c := range []struct {
			name string
			x, y int
			want bool
		}{
			{"Primary", layout.Primary.X, layout.Primary.Y, wantPrimary},
			{"Secondary", layout.Secondary.X, layout.Secondary.Y, wantSecondary},
		} {
			_, style, _ := screen.Get(c.x, c.y)
			if fg, _, _ := style.Decompose(); (fg == targetFG) != c.want {
				t.Fatalf("%s: %s target frame = %v, want %v", when, c.name, fg == targetFG, c.want)
			}
		}
	}
	check("open", false, true)
	app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModShift))
	check("Shift+Left", true, false)
	app.handleKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	check("Shift+Right", false, true)
}
