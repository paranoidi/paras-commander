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

	// panel.tree-collapse-all collapses one expand-all level (or clears leftover manual
	// expansions when the deepen counter is already 0) without disabling tree mode.
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
	applyNextInterruptEvent(t, app, screen) // async load, Secondary follows onto grove

	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).PathString()), filepath.Clean(grove); got != want {
		t.Fatalf("follower path after sync latch = %q, want %q (cursor was on tree child row)", got, want)
	}
}

// TestToggleCarouselFromTreeModeRevertsToFlatLayout is a regression test: switching to carousel
// view while a panel is in tree mode with a directory expanded left ListLayout stuck on Tree and
// the cursor at a treeRows index, while carousel rendering (internal/panelcarousel) reads the
// flat Entries slice directly — a mismatch that showed up as a wrong/missing cursor highlight in
// carousel view. Toggling carousel on must force the panel back to flat layout with the cursor
// remapped onto the same file.
func TestToggleCarouselFromTreeModeRevertsToFlatLayout(t *testing.T) {
	dir := t.TempDir()
	orchard := filepath.Join(dir, "orchard")
	if err := os.Mkdir(orchard, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orchard, "ember.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "zephyr.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	// Directories sort first, so the cursor already sits on "orchard".
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleTree)
	applyNextInterruptEvent(t, app, screen) // async child load
	if got := app.model.Primary.VisibleEntryCount(); got != 3 {
		t.Fatalf("VisibleEntryCount after expand = %d, want 3 (orchard, ember.txt, zephyr.txt)", got)
	}
	// treeRows index 2 ("zephyr.txt") sits one past its flat Entries index (1) once ember.txt is
	// counted as an extra row — the exact mismatch that used to leave Cursor out of range.
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "zephyr.txt")

	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleCarousel)

	if app.model.Primary.ListLayout != panel.ListLayoutFlat {
		t.Fatal("toggling carousel on should revert the panel to flat layout")
	}
	entry, ok := app.model.Primary.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry not ok after toggling carousel from tree mode (cursor left out of range)")
	}
	if entry.Name != "zephyr.txt" {
		t.Fatalf("CurrentEntry().Name = %q, want %q (cursor should follow the same file)", entry.Name, "zephyr.txt")
	}
}

// TestToggleCarouselFromExpandedChildRowLandsOnRootAncestor: when the cursor sits on a row nested
// inside an expanded directory (a row with no counterpart in the flat listing at all), leaving
// tree mode must follow the same node-collapse rule as CollapseAllTree/CollapseTreeCursorRow — the
// cursor lands on the row's depth-0 ancestor — rather than being left on whatever numeric index
// the nested row's treeRows position happened to clamp to in the flat Entries slice.
func TestToggleCarouselFromExpandedChildRowLandsOnRootAncestor(t *testing.T) {
	dir := t.TempDir()
	orchard := filepath.Join(dir, "orchard")
	if err := os.Mkdir(orchard, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orchard, "ember.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	// Directories sort first, so the cursor already sits on "orchard".
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleTree)
	applyNextInterruptEvent(t, app, screen) // async child load
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "ember.txt")

	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleCarousel)

	if app.model.Primary.ListLayout != panel.ListLayoutFlat {
		t.Fatal("toggling carousel on should revert the panel to flat layout")
	}
	entry, ok := app.model.Primary.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry not ok after toggling carousel from a nested tree row")
	}
	if entry.Name != "orchard" {
		t.Fatalf("CurrentEntry().Name = %q, want %q (cursor should land on the depth-0 ancestor, ember.txt has no flat-list counterpart)", entry.Name, "orchard")
	}
}

// TestPanelTreeExpandWorksAfterCarouselRoundTrip is a regression test: leaving tree mode used to
// wipe TreeRoots (so cached Children are gone) without also resetting TreeExpanded, leaving stale
// "expanded" flags behind. Re-entering tree mode later (here via toggling carousel back off, then
// toggling tree on again) reseeded fresh TreeRoots with Children == nil but reused the stale
// TreeExpanded map, so the first expand press on a directory that was expanded in the earlier
// session actually toggled it *closed* (since ToggleTreeExpand inverts the already-true stale
// flag) instead of expanding it — expand appeared to silently do nothing.
func TestPanelTreeExpandWorksAfterCarouselRoundTrip(t *testing.T) {
	dir := t.TempDir()
	orchard := filepath.Join(dir, "orchard")
	if err := os.Mkdir(orchard, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orchard, "ember.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	// Expand "orchard" once, then bounce through carousel mode and back to flat.
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleTree)
	applyNextInterruptEvent(t, app, screen)
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleCarousel)
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleCarousel)
	if app.model.Primary.ListLayout != panel.ListLayoutFlat {
		t.Fatal("panel should be back in flat layout after leaving carousel")
	}

	// Re-enter tree mode and expand "orchard" again — this must actually expand, not silently
	// collapse an already-invisible stale expand flag.
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleTree)
	applyNextInterruptEvent(t, app, screen)
	if got := app.model.Primary.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after re-expand = %d, want 2 (orchard, ember.txt) — expand was a no-op due to stale TreeExpanded state", got)
	}
}

// TestPanelTreeExpandAllShallowDepthLimitToast covers the sixth Ctrl+Alt+Right: after five
// deepen presses the action shows an info toast instead of expanding further.
func TestPanelTreeExpandAllShallowDepthLimitToast(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "orchard"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	for i := 0; i < 5; i++ {
		app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelTreeExpandAllShallow)
	}
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelTreeExpandAllShallow)
	if got, want := app.model.Message, "Expand all is limited to depth 5"; got != want {
		t.Fatalf("Message = %q, want %q", got, want)
	}
	if app.model.MessageUrgency != ui.MessageUrgencyInfo {
		t.Fatalf("MessageUrgency = %v, want info", app.model.MessageUrgency)
	}
}
