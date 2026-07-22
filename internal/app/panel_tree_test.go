package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// TestPanelToggleTreeExpandsDirectoryUnderCursor exercises panel.toggle-tree (bound to Space):
// on first use it should both enable tree layout and expand the directory under the cursor.
func TestPanelToggleTreeExpandsDirectoryUnderCursor(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "orchard"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "orchard", "ember.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	if app.model.Primary.ListLayout == panel.ListLayoutTree {
		t.Fatal("panel should start in flat layout")
	}
	// Directories sort first, so the cursor already sits on "orchard".
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleTree)
	if app.model.Primary.ListLayout != panel.ListLayoutTree {
		t.Fatal("Space should enable tree layout on first use")
	}
	// Child loading is async (Phase 2): the row starts loading, and expansion completes once the
	// scheduler's goroutine posts its result back to the screen.
	applyNextInterruptEvent(t, app, screen)
	if got := app.model.Primary.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after Space = %d, want 2 (orchard expanded to show ember.txt)", got)
	}
}

// TestPanelToggleTreeNoOpOnFileRow confirms Space still enables tree layout when the cursor is
// on a file row, but performs no expansion (nothing to expand).
func TestPanelToggleTreeNoOpOnFileRow(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lantern.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleTree)
	if app.model.Primary.ListLayout != panel.ListLayoutTree {
		t.Fatal("Space should enable tree layout even when the cursor is on a file row")
	}
	if got := app.model.Primary.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount = %d, want 1 (no expansion possible on a file row)", got)
	}
}

// TestPanelTreeExpandActionEnablesTreeAndExpands exercises panel.tree-expand (Alt+Right)
// end-to-end through action dispatch: it should auto-enable tree layout (like Space) and expand
// the directory under the cursor.
func TestPanelTreeExpandActionEnablesTreeAndExpands(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "orchard"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "orchard", "ember.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	if app.model.Primary.ListLayout == panel.ListLayoutTree {
		t.Fatal("panel should start in flat layout")
	}
	// Directories sort first, so the cursor already sits on "orchard".
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelTreeExpand)
	if app.model.Primary.ListLayout != panel.ListLayoutTree {
		t.Fatal("panel.tree-expand should enable tree layout on first use")
	}
	// Child loading is async (Phase 2): wait for the scheduler's goroutine to post its result.
	applyNextInterruptEvent(t, app, screen)
	if got := app.model.Primary.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after panel.tree-expand = %d, want 2 (orchard expanded to show ember.txt)", got)
	}

	// panel.tree-collapse-all should clear the expansion back to depth 0 without disabling
	// tree mode.
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelTreeCollapseAll)
	if app.model.Primary.ListLayout != panel.ListLayoutTree {
		t.Fatal("panel.tree-collapse-all should not disable tree layout")
	}
	if got := app.model.Primary.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount after panel.tree-collapse-all = %d, want 1 (collapsed back to depth 0)", got)
	}
}

// TestSyncFollowsCursorOntoTreeChildRow is Phase 4 item 3 (plan doc): confirm
// syncFollowFromActive already resolves the driver's cursor via CurrentEntry, which is
// tree-mode-aware, so latching sync while the cursor sits on an expanded child row loads
// the follower to that child's path rather than the driver's top-level cwd.
func TestSyncFollowsCursorOntoTreeChildRow(t *testing.T) {
	dir := t.TempDir()
	orchard := filepath.Join(dir, "orchard")
	grove := filepath.Join(orchard, "grove")
	if err := os.MkdirAll(grove, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.ActivePanel = ui.PrimaryPanel
	// Directories sort first, so the cursor already sits on "orchard".
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleTree)
	if app.model.Primary.ListLayout != panel.ListLayoutTree {
		t.Fatal("Space should enable tree layout")
	}
	// Child loading is async (Phase 2): wait for the scheduler's goroutine to post its result.
	applyNextInterruptEvent(t, app, screen)
	if got := app.model.Primary.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after expand = %d, want 2 (orchard expanded to show grove)", got)
	}

	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "grove")
	app.dispatch(keymap.ActionPanelToggleSync)

	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).PathString()), filepath.Clean(grove); got != want {
		t.Fatalf("follower path after sync latch = %q, want %q (cursor was on tree child row)", got, want)
	}
}
