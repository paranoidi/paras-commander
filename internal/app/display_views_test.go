package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func writeCompareTree(t *testing.T, dir string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("copper-%02d.bin", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAuxiliaryScreensOpenFromJobsView(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.jobsCtrl.OpenJobsView()
	if !app.tryDispatchAuxiliaryScreens(keymap.ActionCommandsOpen) {
		t.Fatal("commands.open should be consumed from jobs view")
	}
	if app.model.ViewMode != ui.ViewCommands {
		t.Fatalf("ViewMode = %v, want ViewCommands", app.model.ViewMode)
	}
}

func TestAuxiliaryScreensOpenFromCommandsViewViaKey(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.commandsCtrl.OpenView()
	ev := tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModAlt)
	if app.actionFromKeyEvent(ev) != keymap.ActionJobsOpen {
		t.Fatalf("Alt+J in commands view = %q, want jobs.open", app.actionFromKeyEvent(ev))
	}
	_, rendered := app.handleKey(ev)
	if !rendered {
		t.Fatal("handleKey should render")
	}
	if app.model.ViewMode != ui.ViewJobs {
		t.Fatalf("ViewMode = %v, want ViewJobs", app.model.ViewMode)
	}
}

func TestJobsViewMenuIncludesDisplay(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.jobsCtrl.OpenJobsView()
	var hasDisplay bool
	for _, def := range menu.ActiveDefinitions(app.model.MenuDefinitions) {
		if def.ID == menu.TopDisplay {
			hasDisplay = true
			break
		}
	}
	if !hasDisplay {
		t.Fatal("jobs view menu bar missing Display")
	}
}

func TestAuxiliaryScreensSwitchTearsDownCompare(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	left := t.TempDir()
	right := t.TempDir()
	writeCompareTree(t, left, 40)
	writeCompareTree(t, right, 40)
	app.model.Primary.Path = pathloc.MustParse(left)
	app.model.Secondary.Path = pathloc.MustParse(right)

	app.openComparePanels()
	if app.model.ViewMode != ui.ViewCompare {
		t.Fatal("compare view did not open")
	}

	consumed := false
	done := make(chan struct{})
	go func() {
		consumed = app.tryDispatchAuxiliaryScreens(keymap.ActionJobsOpen)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("switching from compare to jobs did not return")
	}
	if !consumed {
		t.Fatal("jobs.open should be consumed from compare view")
	}
	if app.model.ViewMode != ui.ViewJobs {
		t.Fatalf("ViewMode = %v, want ViewJobs", app.model.ViewMode)
	}
	if !app.model.CompareSnapshot.PrimaryRoot.IsZero() || app.model.CompareSnapshot.Rows != nil {
		t.Fatalf("compare snapshot still set after leaving view: phase=%v rows=%d", app.model.CompareSnapshot.Phase, len(app.model.CompareSnapshot.Rows))
	}
	if app.model.CompareView != (ui.CompareViewState{}) {
		t.Fatalf("compare view state still set: %+v", app.model.CompareView)
	}
}

func TestAuxiliaryScreensSwitchFromCompareDropsDedupReturnHook(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "maple.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "cedar.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	snap := waitDedupDone(t, app)
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v, want ViewDedup", app.model.ViewMode)
	}
	if snap.Phase != comparepkg.DedupDone {
		t.Fatalf("dedup phase = %v (%s)", snap.Phase, snap.Err)
	}

	other := t.TempDir()
	rescans := 0
	if !app.compareCtrl.OpenPaths(pathloc.MustParse(dir), pathloc.MustParse(other), false, func() {
		rescans++
		app.dedupCtrl.ReopenPreservingState()
	}) {
		t.Fatal("OpenPaths failed")
	}
	if app.model.ViewMode != ui.ViewCompare {
		t.Fatalf("ViewMode = %v, want ViewCompare", app.model.ViewMode)
	}

	if !app.tryDispatchAuxiliaryScreens(keymap.ActionMessagesOpen) {
		t.Fatal("messages.open should be consumed from compare view")
	}
	if rescans != 0 {
		t.Fatalf("dedup rescan count = %d, want 0", rescans)
	}
	if app.model.ViewMode != ui.ViewMessages {
		t.Fatalf("ViewMode = %v, want ViewMessages", app.model.ViewMode)
	}
	if !app.model.CompareSnapshot.PrimaryRoot.IsZero() {
		t.Fatal("compare session still live after leaving for messages")
	}
	if app.model.DedupProgressDialog.Open {
		t.Fatal("dedup progress dialog opened; detour should not rescan")
	}
}

func TestAuxiliaryScreensSwitchKeepsDedupResults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "maple.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cedar.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	if !app.tryDispatchAuxiliaryScreens(keymap.ActionCommandsOpen) {
		t.Fatal("commands.open should be consumed from dedup view")
	}
	if app.model.ViewMode != ui.ViewCommands {
		t.Fatalf("ViewMode = %v, want ViewCommands", app.model.ViewMode)
	}
	if !app.dedupCtrl.HasResults() {
		t.Fatal("dedup results should be kept after switching views")
	}
	app.dispatch(keymap.ActionDedupOpen)
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v, want ViewDedup after dedup.open", app.model.ViewMode)
	}

	// dedup.open again toggles the view off, keeping results.
	if !app.tryDispatchAuxiliaryScreens(keymap.ActionDedupOpen) {
		t.Fatal("dedup.open should be consumed from dedup view")
	}
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser after toggling dedup off", app.model.ViewMode)
	}
	if !app.dedupCtrl.HasResults() {
		t.Fatal("dedup results should be kept after toggling the view off")
	}
}
