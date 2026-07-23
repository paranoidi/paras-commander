package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func flattenDialogTestSetup(t *testing.T) (app *App, activeDir, inactiveDir string) {
	t.Helper()
	dir := t.TempDir()
	activeDir = filepath.Join(dir, "active")
	inactiveDir = filepath.Join(dir, "inactive")
	root := filepath.Join(dir, "source")
	for _, p := range []string{activeDir, inactiveDir, root} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app = newApp(t, screen, dir)
	if err := app.activePanel().Load(activeDir); err != nil {
		t.Fatal(err)
	}
	if err := app.inactivePanel().Load(inactiveDir); err != nil {
		t.Fatal(err)
	}
	app.activePanel().SelectedPaths = map[string]bool{filepath.Clean(root): true}
	app.openFlattenDialog()
	if !app.model.FlattenDialog.Open {
		t.Fatal("flatten dialog should be open")
	}
	return app, activeDir, inactiveDir
}

func footerHasHint(keys []menu.FunctionKey, hint, keyLabel string) bool {
	for _, fk := range keys {
		if fk.Hint == hint && fk.KeyLabel == keyLabel {
			return true
		}
	}
	return false
}

func TestFlattenDestinationTargetPanel(t *testing.T) {
	app, _, inactiveDir := flattenDialogTestSetup(t)

	// Default destination is the active (Primary) panel's path.
	app.applyFlattenDestinationPathValidation()
	if !app.model.DestinationTargetPrimary {
		t.Fatal("expected Primary panel marked as destination target")
	}
	if app.model.DestinationTargetSecondary {
		t.Fatal("Secondary panel should not be marked as destination target")
	}

	// Retyping the destination to the inactive (Secondary) panel's path flips the target.
	app.model.FlattenDialog.Destination.Value = inactiveDir
	app.applyFlattenDestinationPathValidation()
	if !app.model.DestinationTargetSecondary {
		t.Fatal("expected Secondary panel marked as destination target")
	}
	if app.model.DestinationTargetPrimary {
		t.Fatal("Primary panel should no longer be marked as destination target")
	}

	app.closeFlattenDialog()
	if app.model.DestinationTargetPrimary || app.model.DestinationTargetSecondary {
		t.Fatal("closing the dialog should clear destination target panels")
	}
}

func TestFlattenMixedSelectionShowsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "alpha.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(dir, "bravo")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{
		filepath.Clean(filePath): true,
		filepath.Clean(subDir):   true,
	}

	app.openFlattenDialog()
	if app.model.FlattenDialog.Open {
		t.Fatal("flatten dialog should not open for mixed selection")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyError {
		t.Fatalf("urgency = %v, want error", app.model.MessageUrgency)
	}
}

func TestFileMenuFlattenOpensDialog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sub := filepath.Join(dir, "charlie")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "delta")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.activePanel().Load(dir); err != nil {
		t.Fatal(err)
	}
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatal(err)
	}
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{filepath.Clean(sub): true}

	activateFileMenuItem(t, app, 'F')
	if !app.model.FlattenDialog.Open {
		t.Fatal("flatten dialog should be open")
	}
	if app.model.FlattenDialog.Destination.Value != transferPrefilledDestination(dir).Value {
		t.Fatalf("destination = %q, want %q", app.model.FlattenDialog.Destination.Value, transferPrefilledDestination(dir).Value)
	}
	if app.model.FlattenDialog.Recursive {
		t.Fatal("recursive should default false")
	}
	if !app.model.FlattenDialog.RemoveEmpty {
		t.Fatal("remove empty should default true")
	}
	if app.model.FlattenDialog.FocusField != 0 {
		t.Fatalf("FocusField = %d, want destination row 0", app.model.FlattenDialog.FocusField)
	}
}

func TestFlattenConfirmQueuesJob(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "echo")
	dst := filepath.Join(dir, "foxtrot")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "golf.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.stopWorker()
	if err := app.activePanel().Load(dir); err != nil {
		t.Fatal(err)
	}
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatal(err)
	}
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{filepath.Clean(root): true}

	app.openFlattenDialog()
	app.handleFlattenDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FlattenDialog.Open {
		t.Fatal("dialog should close after Enter on OK")
	}
	all := app.jobState.AllJobs()
	if len(all) != 1 {
		t.Fatalf("jobs = %d, want 1", len(all))
	}
	if all[0].Type != jobs.TypeFlatten {
		t.Fatalf("job type = %v, want flatten", all[0].Type)
	}
	if len(all[0].Sources) != 1 {
		t.Fatalf("sources = %d, want 1 child", len(all[0].Sources))
	}
}

func TestFlattenDestinationFooterShowsActiveAndInactive(t *testing.T) {
	t.Parallel()
	app, _, _ := flattenDialogTestSetup(t)
	if app.model.FlattenDialog.FocusField != 0 {
		t.Fatalf("FocusField = %d, want 0 (destination)", app.model.FlattenDialog.FocusField)
	}
	keys := app.activeFooterKeys()
	if !footerHasHint(keys, "Active path ◄", "S-left") {
		t.Fatalf("footer = %+v, want Active S-left hint", keys)
	}
	if !footerHasHint(keys, "Inactive path ►", "S-right") {
		t.Fatalf("footer = %+v, want Inactive S-right hint", keys)
	}
}

func TestFlattenDestinationShortcutShiftLeftSetsActivePath(t *testing.T) {
	t.Parallel()
	app, activeDir, inactiveDir := flattenDialogTestSetup(t)
	wantActive := transferPrefilledDestination(activeDir).Value
	if app.model.FlattenDialog.Destination.Value != wantActive {
		t.Fatalf("initial destination = %q, want %q", app.model.FlattenDialog.Destination.Value, wantActive)
	}
	app.handleFlattenDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	wantInactive := transferPrefilledDestination(inactiveDir).Value
	if app.model.FlattenDialog.Destination.Value != wantInactive {
		t.Fatalf("after Shift+Right destination = %q, want %q", app.model.FlattenDialog.Destination.Value, wantInactive)
	}
	app.handleFlattenDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModShift))
	if app.model.FlattenDialog.Destination.Value != wantActive {
		t.Fatalf("after Shift+Left destination = %q, want %q", app.model.FlattenDialog.Destination.Value, wantActive)
	}
}

func TestFlattenDestinationShortcutShiftRightSetsInactivePath(t *testing.T) {
	t.Parallel()
	app, _, inactiveDir := flattenDialogTestSetup(t)
	wantInactive := transferPrefilledDestination(inactiveDir).Value
	app.handleFlattenDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	if app.model.FlattenDialog.Destination.Value != wantInactive {
		t.Fatalf("destination = %q, want %q", app.model.FlattenDialog.Destination.Value, wantInactive)
	}
}

func TestFlattenDestinationShortcutsNoOpWhenUnfocused(t *testing.T) {
	t.Parallel()
	app, _, _ := flattenDialogTestSetup(t)
	want := app.model.FlattenDialog.Destination.Value
	app.handleFlattenDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.FlattenDialog.FocusField != 1 {
		t.Fatalf("FocusField = %d, want 1 (recursive)", app.model.FlattenDialog.FocusField)
	}
	keys := app.activeFooterKeys()
	if footerHasHint(keys, "Active path ◄", "S-left") || footerHasHint(keys, "Inactive path ►", "S-right") {
		t.Fatalf("footer = %+v, must not show Active/Inactive when destination unfocused", keys)
	}
	app.handleFlattenDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModShift))
	if app.model.FlattenDialog.Destination.Value != want {
		t.Fatalf("Shift+Left destination = %q, want unchanged %q", app.model.FlattenDialog.Destination.Value, want)
	}
	app.handleFlattenDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	if app.model.FlattenDialog.Destination.Value != want {
		t.Fatalf("Shift+Right destination = %q, want unchanged %q", app.model.FlattenDialog.Destination.Value, want)
	}
}

func TestFlattenInactivePanelIsSourceUsesActive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	activeDir := filepath.Join(dir, "active")
	sourceDir := filepath.Join(dir, "source")
	for _, p := range []string{activeDir, sourceDir} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.activePanel().Load(activeDir); err != nil {
		t.Fatal(err)
	}
	// inactive panel is navigated into the directory being flattened
	if err := app.inactivePanel().Load(sourceDir); err != nil {
		t.Fatal(err)
	}
	app.activePanel().SelectedPaths = map[string]bool{filepath.Clean(sourceDir): true}
	app.openFlattenDialog()
	if !app.model.FlattenDialog.Open {
		t.Fatal("flatten dialog should be open")
	}
	want := transferPrefilledDestination(activeDir).Value
	if app.model.FlattenDialog.Destination.Value != want {
		t.Fatalf("destination = %q, want active panel %q", app.model.FlattenDialog.Destination.Value, want)
	}
}

func TestFlattenDefaultLocationActivePrefill(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	activeDir := filepath.Join(dir, "active")
	inactiveDir := filepath.Join(dir, "inactive")
	root := filepath.Join(dir, "source")
	for _, p := range []string{activeDir, inactiveDir, root} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.config.Operations.FlattenDefaultLocation = "active"
	if err := app.activePanel().Load(activeDir); err != nil {
		t.Fatal(err)
	}
	if err := app.inactivePanel().Load(inactiveDir); err != nil {
		t.Fatal(err)
	}
	app.activePanel().SelectedPaths = map[string]bool{filepath.Clean(root): true}
	app.openFlattenDialog()
	want := transferPrefilledDestination(activeDir).Value
	if app.model.FlattenDialog.Destination.Value != want {
		t.Fatalf("destination = %q, want active panel %q", app.model.FlattenDialog.Destination.Value, want)
	}
}
