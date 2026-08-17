package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestQuickViewDisablesSyncWithWarn(t *testing.T) {
	root := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	app.model.SyncFollowEnabled = true
	app.model.SyncFollowPanel = ui.PrimaryPanel

	app.dispatch(keymap.ActionFileQuickView)

	if !app.model.QuickViewEnabled {
		t.Fatal("quick view should be enabled")
	}
	if app.model.SyncFollowEnabled {
		t.Fatal("sync should be disabled when quick view is enabled")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyWarn {
		t.Fatalf("message urgency = %v, want warn", app.model.MessageUrgency)
	}
	if !strings.Contains(app.model.Message, "sync disabled") {
		t.Fatalf("message = %q, want sync disabled notice", app.model.Message)
	}
}

func TestSyncDisablesQuickViewWithWarn(t *testing.T) {
	root := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	app.model.QuickViewEnabled = true

	app.dispatch(keymap.ActionPanelToggleSync)

	if !app.model.SyncFollowEnabled {
		t.Fatal("sync should be enabled")
	}
	if app.model.QuickViewEnabled {
		t.Fatal("quick view should be disabled when sync is enabled")
	}
	if app.previewCtrl.FilePreviewOpen() {
		t.Fatal("file preview should be closed when sync displaces quick view")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyWarn {
		t.Fatalf("message urgency = %v, want warn", app.model.MessageUrgency)
	}
	if !strings.Contains(app.model.Message, "quick view disabled") {
		t.Fatalf("message = %q, want quick view disabled notice", app.model.Message)
	}
}

func TestQuickViewDirRecallsFromInactivePanelHistory(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(alpha, "a.txt"))
	writeFile(t, filepath.Join(alpha, "b.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	right := app.panelByID(ui.SecondaryPanel)
	selectPanelEntryByName(t, right, "alpha")
	if err := right.NavigateTo(alpha, "", app.panelViewportRows(ui.SecondaryPanel)); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, entering alpha
	selectPanelEntryByName(t, right, "b.txt")
	if err := right.Parent(app.panelViewportRows(ui.SecondaryPanel)); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, Parent back to root

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()

	entry, ok := app.model.QuickViewDirOverlay.CurrentEntry()
	if !ok {
		t.Fatal("overlay has no current entry")
	}
	if entry.Name != "b.txt" {
		t.Fatalf("overlay cursor entry = %q, want b.txt from inactive panel history", entry.Name)
	}
}

func TestQuickViewDirMirrorsInactivePanelWhenAlreadyInDirectory(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(alpha, "a.txt"))
	writeFile(t, filepath.Join(alpha, "b.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	right := app.panelByID(ui.SecondaryPanel)
	selectPanelEntryByName(t, right, "alpha")
	if err := right.NavigateTo(alpha, "", app.panelViewportRows(ui.SecondaryPanel)); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, entering alpha
	selectPanelEntryByName(t, right, "b.txt")

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()

	entry, ok := app.model.QuickViewDirOverlay.CurrentEntry()
	if !ok {
		t.Fatal("overlay has no current entry")
	}
	if entry.Name != "b.txt" {
		t.Fatalf("overlay cursor entry = %q, want b.txt from live inactive listing", entry.Name)
	}
	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != filepath.Clean(alpha) {
		t.Fatalf("inactive panel path = %q, want unchanged %q", got, alpha)
	}
}

func TestQuickViewDirRecallsLastSelectedEntry(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(alpha, "a.txt"))
	writeFile(t, filepath.Join(alpha, "b.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	app.model.ActivePanel = ui.PrimaryPanel
	left := app.panelByID(ui.PrimaryPanel)
	selectPanelEntryByName(t, left, "alpha")
	if err := left.NavigateTo(alpha, "", app.panelViewportRows(ui.PrimaryPanel)); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, entering alpha
	selectPanelEntryByName(t, left, "b.txt")
	if err := left.Parent(app.panelViewportRows(ui.PrimaryPanel)); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, Parent back to root
	selectPanelEntryByName(t, left, "alpha")
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()

	entry, ok := app.model.QuickViewDirOverlay.CurrentEntry()
	if !ok {
		t.Fatal("overlay has no current entry")
	}
	if entry.Name != "b.txt" {
		t.Fatalf("overlay cursor entry = %q, want b.txt (last selected in alpha)", entry.Name)
	}
}

func TestQuickViewTabPreservesLatchedDirectoryPreview(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	child := filepath.Join(root, "child")
	for _, p := range []string{alpha, child} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(alpha, "inside.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	if err := app.panelByID(ui.SecondaryPanel).Load(child); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, Secondary enters child
	inactiveBefore := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String())

	app.model.ActivePanel = ui.PrimaryPanel
	left := app.panelByID(ui.PrimaryPanel)
	selectPanelEntryByName(t, left, "alpha")
	if err := left.NavigateTo(alpha, "", app.panelViewportRows(ui.PrimaryPanel)); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, entering alpha
	selectPanelEntryByName(t, left, "inside.txt")
	if err := left.Parent(app.panelViewportRows(ui.PrimaryPanel)); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, Parent back to root
	selectPanelEntryByName(t, left, "alpha")
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()
	if got, want := filepath.Clean(app.model.QuickViewDirOverlay.Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("overlay path = %q, want %q", got, want)
	}
	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != inactiveBefore {
		t.Fatalf("inactive path before Tab = %q, want unchanged %q", got, inactiveBefore)
	}

	app.dispatch(keymap.ActionPanelSwitch)

	if !app.model.QuickViewEnabled || app.model.QuickViewPanel != ui.PrimaryPanel {
		t.Fatalf("quick view should stay latched on left driver (enabled=%v panel=%d)", app.model.QuickViewEnabled, app.model.QuickViewPanel)
	}
	if app.model.ActivePanel != ui.SecondaryPanel {
		t.Fatalf("ActivePanel = %d, want right panel", app.model.ActivePanel)
	}
	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != inactiveBefore {
		t.Fatalf("inactive path after Tab = %q, want unchanged %q", got, inactiveBefore)
	}
	if app.model.QuickViewDirOverlayActive {
		t.Fatal("dir overlay should be hidden while away from driver panel")
	}
	if app.model.ActiveSubFocus != ui.SubFocusFileList {
		t.Fatalf("ActiveSubFocus = %d, want file list", app.model.ActiveSubFocus)
	}

	app.dispatch(keymap.ActionPanelSwitch)

	if app.model.ActivePanel != ui.PrimaryPanel {
		t.Fatalf("ActivePanel = %d, want left panel after second Tab", app.model.ActivePanel)
	}
	if got, want := filepath.Clean(app.model.QuickViewDirOverlay.Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("overlay path after return = %q, want %q", got, want)
	}
	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != inactiveBefore {
		t.Fatalf("inactive path after return = %q, want unchanged %q", got, inactiveBefore)
	}
}

func TestQuickViewPreviewPageScrollWithCtrlJK(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "notes.txt"))
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "notes.txt")
	// Set quick view state directly (not via dispatch) so no real async preview load races
	// the manual FilePreview patch below.
	app.model.QuickViewEnabled = true
	app.model.QuickViewPanel = ui.PrimaryPanel

	app.commandsMu.Lock()
	app.model.FilePreview.Open = true
	app.model.FilePreview.Phase = ui.FilePreviewPhaseDone
	app.model.FilePreview.CombinedText = strings.Repeat("line\n", 200)
	app.model.FilePreview.Scroll = 0
	app.commandsMu.Unlock()

	app.dispatch(keymap.ActionFileQuickViewPreviewPageDown) // default Ctrl+J (vi j = down)
	if app.model.FilePreview.Scroll < 1 {
		t.Fatalf("FilePreview.Scroll = %d, want > 0 after preview page down", app.model.FilePreview.Scroll)
	}
	scrollAfterDown := app.model.FilePreview.Scroll

	app.dispatch(keymap.ActionFileQuickViewPreviewPageUp) // default Ctrl+K (vi k = up)
	if app.model.FilePreview.Scroll >= scrollAfterDown {
		t.Fatalf("FilePreview.Scroll = %d, want < %d after preview page up", app.model.FilePreview.Scroll, scrollAfterDown)
	}
}

func TestQuickViewPersistsAcrossPanelSwitch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "notes.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "notes.txt")
	app.dispatch(keymap.ActionFileQuickView)
	if !app.model.QuickViewEnabled {
		t.Fatal("quick view should be enabled")
	}
	if !app.previewCtrl.FilePreviewOpen() {
		t.Fatal("file preview should open for highlighted file with quick view on")
	}

	app.dispatch(keymap.ActionPanelSwitch)

	if !app.model.QuickViewEnabled || app.model.QuickViewPanel != ui.PrimaryPanel {
		t.Fatalf("quick view should stay latched on left (enabled=%v panel=%d)", app.model.QuickViewEnabled, app.model.QuickViewPanel)
	}
	if app.model.ActivePanel != ui.SecondaryPanel {
		t.Fatalf("ActivePanel = %d, want right panel", app.model.ActivePanel)
	}
	if app.previewCtrl.FilePreviewOpen() {
		t.Fatal("file preview should be hidden while away from quick-view driver panel")
	}

	app.dispatch(keymap.ActionPanelSwitch)

	if app.model.ActivePanel != ui.PrimaryPanel {
		t.Fatalf("ActivePanel = %d, want left panel after return", app.model.ActivePanel)
	}
	if !app.previewCtrl.FilePreviewOpen() {
		t.Fatal("file preview should reopen after returning to quick-view driver panel")
	}
}

func TestToggleSyncEnablesAndImmediatelyMirrorsHighlightedFolder(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")

	app.dispatch(keymap.ActionPanelToggleSync)
	applyNextInterruptEvent(t, app, screen) // async load, immediate sync-follow mirror

	if !app.model.SyncFollowEnabled || app.model.SyncFollowPanel != ui.PrimaryPanel {
		t.Fatalf("Sync state after enable = (enabled=%v panel=%d), want (true, PrimaryPanel)", app.model.SyncFollowEnabled, app.model.SyncFollowPanel)
	}
	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("right panel path after enable = %q, want %q", got, want)
	}
	if !strings.Contains(app.model.Message, "Sync") {
		t.Fatalf("transient message = %q, want Sync notice", app.model.Message)
	}
}

// Regression: key handling calls render() before the Run loop's trailing reconcileAfterEvent(),
// so latched sync must run inside render(); otherwise the follower updates one tick late and
// the UI tracks the previous directory highlight.
func TestSyncFollowAppliesBeforeRenderAfterNav(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	applyNextInterruptEvent(t, app, screen) // async load, immediate sync-follow mirror

	app.dispatch(keymap.ActionNavDown)
	app.render()
	applyNextInterruptEvent(t, app, screen) // async load, sync-follow mirrors beta
	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(filepath.Join(root, "beta")); got != want {
		t.Fatalf("after down+render follower path = %q, want %q", got, want)
	}

	app.dispatch(keymap.ActionNavDown)
	app.render()
	applyNextInterruptEvent(t, app, screen) // async load, sync-follow mirrors gamma
	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(filepath.Join(root, "gamma")); got != want {
		t.Fatalf("after second down+render follower path = %q, want %q", got, want)
	}
}

func TestPanelSyncFollowNavDebounceDefersFollowerUntilCleared(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	applyNextInterruptEvent(t, app, screen) // async load, immediate sync-follow mirror
	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(filepath.Join(root, "alpha")); got != want {
		t.Fatalf("right after sync enable = %q, want %q", got, want)
	}
	app.config.UI.KeyRepeatDebounceMS = 500
	app.dispatch(keymap.ActionNavDown)
	app.reconcileAfterEvent()
	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(filepath.Join(root, "alpha")); got != want {
		t.Fatalf("follower path after debounced nav+reconcile = %q, want %q (still coalescing)", got, want)
	}
	app.clearPanelSyncFollowNavCoalesce()
	app.reconcileAfterEvent()
	applyNextInterruptEvent(t, app, screen) // async load, sync-follow mirrors beta after coalesce clears
	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(filepath.Join(root, "beta")); got != want {
		t.Fatalf("follower path after clear+reconcile = %q, want %q", got, want)
	}
}

// Regression: with selections-strip focus, sync must follow the strip row (what the user
// is steering), not the file-list cursor — otherwise the other panel shows a stale directory.
func TestSyncFollowUsesSelectionsStripWhenStripFocused(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(beta, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	app.model.ActiveSubFocus = ui.SubFocusFileList
	left := app.panelByID(ui.PrimaryPanel)
	selectPanelEntryByName(t, left, "beta")
	if selected, _ := left.ToggleSelection(); !selected {
		t.Fatal("toggle selection on beta")
	}
	selectPanelEntryByName(t, left, "alpha")
	if err := left.NavigateTo(alpha, "", 20); err != nil {
		t.Fatalf("NavigateTo alpha: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, entering alpha
	if left.SelectionsStripCount() == 0 {
		t.Fatal("expected selections strip to list beta while cwd is alpha")
	}
	app.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
	left.SelectionsStripCursor = 0

	app.dispatch(keymap.ActionPanelToggleSync)
	applyNextInterruptEvent(t, app, screen) // async load, immediate sync-follow mirror

	want := filepath.Clean(beta)
	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != want {
		t.Fatalf("follower path = %q want %q (strip row should drive sync, not file-list cursor)", got, want)
	}
}

func TestToggleSyncDisablesWhenAlreadyDriving(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")

	app.dispatch(keymap.ActionPanelToggleSync)
	if !app.model.SyncFollowEnabled || app.model.SyncFollowPanel != ui.PrimaryPanel {
		t.Fatalf("Sync state after enable = (enabled=%v panel=%d), want (true, PrimaryPanel)", app.model.SyncFollowEnabled, app.model.SyncFollowPanel)
	}

	app.dispatch(keymap.ActionPanelToggleSync)
	if app.model.SyncFollowEnabled {
		t.Fatalf("SyncFollowEnabled after disable = true, want false")
	}
	if !strings.Contains(app.model.Message, "Sync disabled") {
		t.Fatalf("transient message = %q, want Sync disabled", app.model.Message)
	}
}

func TestToggleSyncFromOtherPanelClearsPreviousDriverFirst(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	gamma := filepath.Join(alpha, "gamma")
	if err := os.MkdirAll(gamma, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	applyNextInterruptEvent(t, app, screen) // async load, immediate sync-follow mirror
	if !app.model.SyncFollowEnabled || app.model.SyncFollowPanel != ui.PrimaryPanel {
		t.Fatalf("Sync state after left enable = (enabled=%v panel=%d), want (true, PrimaryPanel)", app.model.SyncFollowEnabled, app.model.SyncFollowPanel)
	}
	// Right panel should now be inside /alpha (synced from left).
	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("right panel after left enable = %q, want %q", got, want)
	}

	// Switch focus to right and toggle sync there: should clear left's sync, then enable right.
	app.model.ActivePanel = ui.SecondaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.SecondaryPanel), "gamma")
	app.dispatch(keymap.ActionPanelToggleSync)
	applyNextInterruptEvent(t, app, screen) // async load, immediate sync-follow mirror (now right-driven)

	if !app.model.SyncFollowEnabled || app.model.SyncFollowPanel != ui.SecondaryPanel {
		t.Fatalf("Sync state after right toggle = (enabled=%v panel=%d), want (true, SecondaryPanel)", app.model.SyncFollowEnabled, app.model.SyncFollowPanel)
	}
	if got, want := filepath.Clean(app.panelByID(ui.PrimaryPanel).Path.String()), filepath.Clean(gamma); got != want {
		t.Fatalf("left panel path after right takes over = %q, want %q", got, want)
	}
}

func TestSyncFollowsCursorMovementOverDirectory(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	applyNextInterruptEvent(t, app, screen) // async load, immediate sync-follow mirror
	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("right panel after enable = %q, want %q", got, want)
	}

	// Move cursor onto beta and verify the right panel mirrors it.
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "beta")
	app.reconcileAfterEvent()
	applyNextInterruptEvent(t, app, screen) // async load, sync-follow mirrors beta

	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(beta); got != want {
		t.Fatalf("right panel after move = %q, want %q", got, want)
	}
}

func TestSyncSkipsCursorMovementOverFile(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "notes.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	applyNextInterruptEvent(t, app, screen) // async load, immediate sync-follow mirror
	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("right panel after enable = %q, want %q", got, want)
	}

	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "notes.txt")
	app.reconcileAfterEvent()

	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("right panel after non-dir hover = %q, want unchanged %q", got, want)
	}
}

func TestQuickViewFollowsDirectoryHighlight(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	inactiveBefore := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String())
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()

	if got, want := filepath.Clean(app.model.QuickViewDirOverlay.Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("overlay path = %q, want %q", got, want)
	}
	if !app.model.QuickViewDirOverlayActive {
		t.Fatal("quick view dir overlay should be active")
	}
	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != inactiveBefore {
		t.Fatalf("inactive panel path = %q, want unchanged %q", got, inactiveBefore)
	}
	if app.previewCtrl.FilePreviewOpen() {
		t.Fatal("file preview should be closed while quick view shows directory listing")
	}
}

func TestQuickViewFollowsCursorBetweenSubdirectories(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	inactiveBefore := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String())
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()
	if got, want := filepath.Clean(app.model.QuickViewDirOverlay.Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("overlay after alpha = %q, want %q", got, want)
	}
	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != inactiveBefore {
		t.Fatalf("inactive after alpha = %q, want unchanged %q", got, inactiveBefore)
	}

	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "beta")
	app.reconcileAfterEvent()
	if got, want := filepath.Clean(app.model.QuickViewDirOverlay.Path.String()), filepath.Clean(beta); got != want {
		t.Fatalf("overlay after beta = %q, want %q", got, want)
	}
	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != inactiveBefore {
		t.Fatalf("inactive after beta = %q, want unchanged %q", got, inactiveBefore)
	}
}

func TestQuickViewShowsPreviewOnFileHighlight(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "notes.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "notes.txt")
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()

	if !app.previewCtrl.FilePreviewOpen() {
		t.Fatal("file preview should open for a highlighted file with quick view on")
	}
}

func TestQuickViewPreviewNavDebounceDefersPreviewUntilFlush(t *testing.T) {
	root := t.TempDir()
	notes := filepath.Join(root, "notes.txt")
	readme := filepath.Join(root, "readme.txt")
	writeFile(t, notes)
	writeFile(t, readme)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 500

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "notes.txt")
	app.model.QuickViewEnabled = true
	app.previewCtrl.ApplyQuickViewPreviewImmediately()

	app.commandsMu.RLock()
	firstPath := app.model.FilePreview.Path
	app.commandsMu.RUnlock()
	if firstPath != notes {
		t.Fatalf("preview path = %q, want %q", firstPath, notes)
	}

	app.dispatch(keymap.ActionNavDown)
	app.reconcileAfterEvent()

	app.commandsMu.RLock()
	stillPath := app.model.FilePreview.Path
	app.commandsMu.RUnlock()
	if stillPath != notes {
		t.Fatalf("preview path after debounced nav+reconcile = %q, want %q (still coalescing)", stillPath, notes)
	}

	app.previewCtrl.ClearQuickViewNavCoalesce()
	app.reconcileAfterEvent()
	if !app.previewCtrl.FlushQuickViewPreviewNow() {
		t.Fatal("FlushQuickViewPreviewNow should apply deferred preview")
	}

	app.commandsMu.RLock()
	gotPath := app.model.FilePreview.Path
	app.commandsMu.RUnlock()
	if gotPath != readme {
		t.Fatalf("preview path after flush = %q, want %q", gotPath, readme)
	}
}

// TestQuickViewDirToFileDebounceShowsPendingChromeImmediately verifies that navigating from a
// directory entry to a text file, once the debounce flushes, immediately drops the stale
// dir-overlay listing and shows pending file-preview chrome (loading state) rather than
// mirroring the entered directory's listing until the async preview finishes.
func TestQuickViewDirToFileDebounceShowsPendingChromeImmediately(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "bravo")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	article := filepath.Join(root, "article.txt")
	writeFile(t, article)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 500

	// Start with cursor on the directory and quick view open.
	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "bravo")
	app.model.QuickViewEnabled = true
	app.previewCtrl.ApplyQuickViewPreviewImmediately()

	if !app.model.QuickViewDirOverlayActive {
		t.Fatal("dir overlay should be active when cursor is on a directory")
	}

	// Navigate down to the text file; debounce coalesces the preview update.
	app.dispatch(keymap.ActionNavDown)
	app.reconcileAfterEvent()

	// While debounce is active the dir overlay stays (reconcile is skipped).
	if !app.model.QuickViewDirOverlayActive {
		t.Fatal("dir overlay should still be active while debounce is coalescing")
	}

	flushed := app.previewCtrl.FlushQuickViewPreviewNow()
	if !flushed {
		t.Fatal("FlushQuickViewPreviewNow should have applied")
	}

	// After flush: dir overlay must be cleared and the inactive column shows file-preview
	// chrome (pending/loading), never the stale directory listing.
	if app.model.QuickViewDirOverlayActive {
		t.Fatal("dir overlay should be cleared after debounce flush")
	}
	if !app.model.InactiveColumnShowsFilePreview(app.inactivePanelID()) {
		t.Fatal("inactive column should show file-preview chrome immediately after flush")
	}

	app.commandsMu.RLock()
	phase := app.model.FilePreview.Phase
	imagePayload := app.model.FilePreview.ImagePayload
	combinedText := app.model.FilePreview.CombinedText
	app.commandsMu.RUnlock()
	if phase != ui.FilePreviewPhasePending {
		t.Fatalf("file preview phase = %v, want FilePreviewPhasePending", phase)
	}
	if imagePayload != "" {
		t.Fatalf("file preview ImagePayload = %q, want empty", imagePayload)
	}
	if combinedText != "" {
		t.Fatalf("file preview CombinedText = %q, want empty", combinedText)
	}
}

func TestQuickViewOffRestoresInactivePanelState(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	child := filepath.Join(root, "child")
	for _, p := range []string{alpha, child} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	if err := app.panelByID(ui.SecondaryPanel).Load(child); err != nil {
		t.Fatal(err)
	}
	rightBefore := app.panelByID(ui.SecondaryPanel)
	inactivePathBefore := filepath.Clean(rightBefore.Path.String())
	inactiveCursorBefore := rightBefore.Cursor

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()
	if got, want := filepath.Clean(app.model.QuickViewDirOverlay.Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("overlay during quick view = %q, want %q", got, want)
	}

	app.dispatch(keymap.ActionFileQuickView)
	if app.model.QuickViewEnabled {
		t.Fatal("quick view should be disabled after toggle")
	}
	if app.previewCtrl.FilePreviewOpen() {
		t.Fatal("file preview should be closed after quick view off")
	}
	if app.model.QuickViewDirOverlayActive {
		t.Fatal("dir overlay should be cleared after quick view off")
	}
	rightAfter := app.panelByID(ui.SecondaryPanel)
	if got, want := filepath.Clean(rightAfter.Path.String()), inactivePathBefore; got != want {
		t.Fatalf("inactive path after quick view off = %q, want pre-quick-view %q", got, want)
	}
	if rightAfter.Cursor != inactiveCursorBefore {
		t.Fatalf("inactive cursor after quick view off = %d, want %d", rightAfter.Cursor, inactiveCursorBefore)
	}
}

func TestQuickViewDirDoesNotMoveOpenInOtherPanelIndicator(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	child := filepath.Join(root, "child")
	for _, p := range []string{alpha, child} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	if err := app.panelByID(ui.SecondaryPanel).Load(child); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, Secondary enters child
	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()
	app.render()

	openGlyph := app.styles.FolderIconGlyph(theme.FolderIconOpen)
	w, _ := screen.Size()
	leftHalf := w / 2
	var childRowHasOpen, alphaRowHasOpen bool
	for y := 1; y < 20; y++ {
		row := screenLine(screen, y, leftHalf)
		if strings.Contains(row, "child") && strings.Contains(row, openGlyph) {
			childRowHasOpen = true
		}
		if strings.Contains(row, "alpha") && strings.Contains(row, openGlyph) {
			alphaRowHasOpen = true
		}
	}
	if !childRowHasOpen {
		t.Fatal("open-in-other-panel glyph should stay on child row matching real inactive path")
	}
	if alphaRowHasOpen {
		t.Fatal("open-in-other-panel glyph should not move to alpha row while quick view previews alpha on inactive column")
	}
}

func TestSyncDoesNotFollowFromNonDriverActivePanel(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	rightAfterEnable := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String())

	// Switch focus to the non-driver (right) panel and move its cursor.
	app.dispatch(keymap.ActionPanelSwitch)
	if app.model.ActivePanel != ui.SecondaryPanel {
		t.Fatalf("ActivePanel after switch = %d, want SecondaryPanel", app.model.ActivePanel)
	}
	if !app.model.SyncFollowEnabled || app.model.SyncFollowPanel != ui.PrimaryPanel {
		t.Fatalf("Tab should not change Sync state; got (enabled=%v panel=%d), want (true, PrimaryPanel)", app.model.SyncFollowEnabled, app.model.SyncFollowPanel)
	}
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "beta")
	app.reconcileAfterEvent()

	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != rightAfterEnable {
		t.Fatalf("right panel changed while non-driver was active: got %q, want unchanged %q", got, rightAfterEnable)
	}
}

// Regression: bookmark / history-picker navigation jumps the active panel via
// navigatePanelToDirectory (which loads, then would historically need its own sync
// trigger). With the post-event reconciler, the next reconcileAfterEvent re-mirrors
// the follower automatically — proving the chokepoint catches paths that bypass dispatch.
func TestSyncFollowsBookmarkLikeNavigationFromActivePanel(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	betaChild := filepath.Join(beta, "child")
	if err := os.MkdirAll(betaChild, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	applyNextInterruptEvent(t, app, screen) // async load, immediate sync-follow mirror
	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("right panel after enable = %q, want %q", got, want)
	}

	// Simulate a bookmark/history jump: navigate the active panel to /beta.
	if err := app.navigatePanelToDirectory(ui.PrimaryPanel, beta, ""); err != nil {
		t.Fatalf("navigatePanelToDirectory: %v", err)
	}
	// applyNextInterruptEvent itself now runs reconcileAfterEvent after applying each event
	// (mirroring Run()'s loop for every event, interrupts included — see its doc comment), and
	// each application can itself schedule a further async mirror (render() re-checks sync-follow,
	// same as the reconcile pass does) — so rather than guess an exact event count, drain
	// repeatedly until Secondary actually settles on its final target.
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		return filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()) == filepath.Clean(betaChild)
	})

	// Cursor in /beta lands on "child" (only entry); sync should mirror it.
	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(betaChild); got != want {
		t.Fatalf("right panel after bookmark-like jump = %q, want %q (sync should re-mirror)", got, want)
	}
}

// Regression guard: when the inactive (follower) panel changes directory by other means
// (e.g. an out-of-band Load), sync must NOT trigger from it because only the driver-while-active
// fires sync hops.
func TestSyncDoesNotFollowWhenInactivePanelChangesDirectory(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	leftBefore := filepath.Clean(app.panelByID(ui.PrimaryPanel).Path.String())

	// Mutate the follower (right) panel directly. The driver (left) is still active and
	// has not moved its cursor, so the left panel should stay put even after reconcile.
	if err := app.panelByID(ui.SecondaryPanel).Load(beta); err != nil {
		t.Fatalf("Load: %v", err)
	}
	app.reconcileAfterEvent()

	if got := filepath.Clean(app.panelByID(ui.PrimaryPanel).Path.String()); got != leftBefore {
		t.Fatalf("left panel changed because follower moved: got %q, want %q", got, leftBefore)
	}
}

// Regression-by-design: the Insert key (panel.select-toggle) calls
// ToggleSelectionAndAdvance, which moves the cursor down by one. With the previous
// per-call-site wiring this branch had no syncFollowFromActive() call, so sync
// would have silently gone stale. The post-event reconciler catches it for free.
func TestSyncFollowsAfterSelectToggleAdvance(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	applyNextInterruptEvent(t, app, screen) // async load, immediate sync-follow mirror
	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(filepath.Join(root, "alpha")); got != want {
		t.Fatalf("right panel after enable = %q, want %q", got, want)
	}

	// Insert: toggle selection on alpha and advance cursor to beta.
	app.dispatch(keymap.ActionPanelSelectToggle)
	app.reconcileAfterEvent()
	applyNextInterruptEvent(t, app, screen) // async load, sync-follow mirrors beta

	if got, want := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()), filepath.Clean(filepath.Join(root, "beta")); got != want {
		t.Fatalf("right panel after Insert advance = %q, want %q (reconciler should mirror new highlight)", got, want)
	}
}

func TestSyncFollowSkipsHistoryRecordingOnFollower(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.PrimaryPanel
	rightHistoryAtStart := append([]string(nil), app.panelByID(ui.SecondaryPanel).History...)

	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	applyNextInterruptEvent(t, app, screen) // async load, immediate sync-follow mirror
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "beta")
	app.reconcileAfterEvent()
	applyNextInterruptEvent(t, app, screen) // async load, sync-follow mirrors beta

	right := app.panelByID(ui.SecondaryPanel)
	if got, want := filepath.Clean(right.PathString()), filepath.Clean(beta); got != want {
		t.Fatalf("right panel path = %q, want %q", got, want)
	}
	// Sync hops use Load (not NavigateTo), so the follower's directory history must remain untouched
	// beyond whatever it already had at startup.
	if len(right.History) != len(rightHistoryAtStart) {
		t.Fatalf("right history length = %d, want %d (sync should not record history)", len(right.History), len(rightHistoryAtStart))
	}
}
