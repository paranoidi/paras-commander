package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func TestDispatchMovesOnlyActivePanel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	writeFile(t, filepath.Join(dir, "b.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.dispatch(keymap.ActionNavDown)
	if app.model.Primary.Cursor != 1 {
		t.Fatalf("left cursor = %d, want 1", app.model.Primary.Cursor)
	}
	if app.model.Secondary.Cursor != 0 {
		t.Fatalf("right cursor = %d, want 0", app.model.Secondary.Cursor)
	}

	app.dispatch(keymap.ActionPanelSwitch)
	app.dispatch(keymap.ActionNavDown)
	if app.model.ActivePanel != ui.SecondaryPanel {
		t.Fatalf("active panel = %d, want right panel", app.model.ActivePanel)
	}
	if app.model.Primary.Cursor != 1 {
		t.Fatalf("left cursor = %d, want unchanged 1", app.model.Primary.Cursor)
	}
	if app.model.Secondary.Cursor != 1 {
		t.Fatalf("right cursor = %d, want 1", app.model.Secondary.Cursor)
	}
}

func TestHideInactivePanelToggleAndTabShow(t *testing.T) {
	dir := t.TempDir()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.model.SyncFollowEnabled = true
	app.model.SyncFollowPanel = ui.PrimaryPanel
	app.model.QuickViewEnabled = true

	app.dispatch(keymap.ActionPanelToggleHideInactive)
	if !app.model.HideInactivePanel {
		t.Fatal("HideInactivePanel = false, want true")
	}
	if app.model.SyncFollowEnabled {
		t.Fatal("sync still enabled after hiding inactive panel")
	}
	if app.model.QuickViewEnabled {
		t.Fatal("quick view still enabled after hiding inactive panel")
	}

	app.dispatch(keymap.ActionPanelSwitch)
	if app.model.HideInactivePanel {
		t.Fatal("HideInactivePanel = true after Tab, want shown")
	}
	if app.model.ActivePanel != ui.SecondaryPanel {
		t.Fatalf("ActivePanel = %d, want right", app.model.ActivePanel)
	}
}

func TestDispatchTogglesSelectionOnlyInActivePanel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	writeFile(t, filepath.Join(dir, "b.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	leftEntry, ok := app.model.Primary.CurrentEntry()
	if !ok {
		t.Fatal("left CurrentEntry() ok = false, want true")
	}
	rightEntry, ok := app.model.Secondary.CurrentEntry()
	if !ok {
		t.Fatal("right CurrentEntry() ok = false, want true")
	}

	app.dispatch(keymap.ActionPanelSelectToggle)
	if !app.model.Primary.IsSelected(leftEntry) {
		t.Fatal("left active entry is not selected")
	}
	if app.model.Primary.Cursor != 1 {
		t.Fatalf("left cursor = %d, want 1 after selection advances", app.model.Primary.Cursor)
	}
	if app.model.Secondary.IsSelected(rightEntry) {
		t.Fatal("right entry is selected, want inactive panel unchanged")
	}

	app.dispatch(keymap.ActionPanelSwitch)
	app.dispatch(keymap.ActionPanelSelectToggle)
	if !app.model.Secondary.IsSelected(rightEntry) {
		t.Fatal("right active entry is not selected after switching panels")
	}
}

func TestMenuInputUsesMenuStateInsteadOfPanelNavigation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	writeFile(t, filepath.Join(dir, "b.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	if !app.model.Menu.Open {
		t.Fatal("menu open = false, want true")
	}
	if app.model.Menu.ActiveMenu != menu.DefaultIndex() {
		t.Fatalf("active menu = %d, want file menu", app.model.Menu.ActiveMenu)
	}

	// F9 now opens menu bar only (no pulldown). Press Down to open pulldown.
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.Primary.Cursor != 0 {
		t.Fatalf("left cursor = %d, want unchanged 0 while menu is open", app.model.Primary.Cursor)
	}
	if !app.model.Menu.PulldownOpen {
		t.Fatalf("pulldown open = false after Down")
	}
	// First selectable menu item (View at index 0) should be selected.
	if app.model.Menu.SelectedItem != 0 {
		t.Fatalf("selected menu item = %d, want 0", app.model.Menu.SelectedItem)
	}
	// Press Down again to move to second item.
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.Menu.SelectedItem != 1 {
		t.Fatalf("selected menu item = %d, want 1", app.model.Menu.SelectedItem)
	}

	// Esc when pulldown open: closes pulldown, keeps menu bar active.
	app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.Menu.PulldownOpen {
		t.Fatal("pulldown open = true, want false after Esc")
	}
	if !app.model.Menu.Open {
		t.Fatal("menu open = false, want true after Esc (menu bar stays active)")
	}
	// Second Esc closes the menu bar entirely.
	app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want false after second Esc")
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	app.handleKey(tcell.NewEventKey(tcell.KeyCtrlLeftSq, 0, tcell.ModNone))
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want false after Ctrl-[ (Esc alias)")
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, '\x1b', tcell.ModNone))
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want false after escape rune")
	}
}

func TestLeftMenuToggleHiddenIsGlobal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".hidden"))
	writeFile(t, filepath.Join(dir, "visible.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	app.model.ActivePanel = ui.SecondaryPanel

	app.dispatch(keymap.ActionAppOpenMenu)
	app.moveMenu(-1)
	// Open pulldown for Left menu.
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.moveMenuItem(2)
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want closed")
	}
	if !app.model.Primary.ShowHidden {
		t.Fatal("left ShowHidden = false, want true")
	}
	if !app.model.Secondary.ShowHidden {
		t.Fatal("right ShowHidden = false, want true (toggle is global, not panel-scoped)")
	}
	applyNextInterruptEvent(t, app, screen) // async reload, Primary
	applyNextInterruptEvent(t, app, screen) // async reload, Secondary
	if len(app.model.Primary.Entries) != 2 {
		t.Fatalf("left len(Entries) = %d, want hidden and visible entries", len(app.model.Primary.Entries))
	}
	if app.model.Message != "Hidden and ignored files shown" {
		t.Fatalf("Message = %q, want global hidden visibility message", app.model.Message)
	}
}

// TestToggleHiddenConvergesDivergedPanels guards the set-to-value semantics: even when
// the panels somehow diverge, one toggle brings both to the same state (flip-each
// semantics would keep them diverged forever).
func TestToggleHiddenConvergesDivergedPanels(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "visible.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.model.Secondary.ShowHidden = true
	app.model.ActivePanel = ui.PrimaryPanel

	app.dispatch(keymap.ActionPanelToggleHidden)
	if !app.model.Primary.ShowHidden || !app.model.Secondary.ShowHidden {
		t.Fatalf("ShowHidden = (%v, %v), want both true after toggle from diverged state",
			app.model.Primary.ShowHidden, app.model.Secondary.ShowHidden)
	}

	app.dispatch(keymap.ActionPanelToggleHidden)
	if app.model.Primary.ShowHidden || app.model.Secondary.ShowHidden {
		t.Fatalf("ShowHidden = (%v, %v), want both false after second toggle",
			app.model.Primary.ShowHidden, app.model.Secondary.ShowHidden)
	}
}

func TestOpenSelectedDirectoryInInactivePanel(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	gamma := filepath.Join(alpha, "gamma")
	if err := os.MkdirAll(gamma, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	for i := 0; i < left.VisibleEntryCount(); i++ {
		entry, _, ok := left.VisibleEntry(i)
		if ok && entry.Name == "alpha" {
			left.Cursor = i
			break
		}
	}
	app.model.ActivePanel = ui.PrimaryPanel
	app.dispatch(keymap.ActionPanelOpenDirInOther)
	applyNextInterruptEvent(t, app, screen) // async load, Secondary opens alpha

	wantRoot := filepath.Clean(root)
	wantAlpha := filepath.Clean(alpha)
	if got := filepath.Clean(app.panelByID(ui.PrimaryPanel).Path.String()); got != wantRoot {
		t.Fatalf("left panel path = %q want %q", got, wantRoot)
	}
	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != wantAlpha {
		t.Fatalf("right panel path = %q want %q", got, wantAlpha)
	}

	right := app.panelByID(ui.SecondaryPanel)
	for i := 0; i < right.VisibleEntryCount(); i++ {
		entry, _, ok := right.VisibleEntry(i)
		if ok && entry.Name == "gamma" {
			right.Cursor = i
			break
		}
	}
	app.model.ActivePanel = ui.SecondaryPanel
	app.dispatch(keymap.ActionPanelOpenDirInOther)
	applyNextInterruptEvent(t, app, screen) // async load, Primary opens gamma

	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != wantAlpha {
		t.Fatalf("right panel path = %q want %q after second open", got, wantAlpha)
	}
	wantGamma := filepath.Clean(gamma)
	if got := filepath.Clean(app.panelByID(ui.PrimaryPanel).Path.String()); got != wantGamma {
		t.Fatalf("left panel path = %q want %q", got, wantGamma)
	}
}

func TestOpenActivePathInInactivePanel(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	gamma := filepath.Join(alpha, "gamma")
	if err := os.MkdirAll(gamma, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	for i := 0; i < left.VisibleEntryCount(); i++ {
		entry, _, ok := left.VisibleEntry(i)
		if ok && entry.Name == "alpha" {
			left.Cursor = i
			break
		}
	}
	app.model.ActivePanel = ui.PrimaryPanel
	app.dispatch(keymap.ActionNavOpen)
	applyNextInterruptEvent(t, app, screen) // async load, Primary enters alpha

	for i := 0; i < left.VisibleEntryCount(); i++ {
		entry, _, ok := left.VisibleEntry(i)
		if ok && entry.Name == "gamma" {
			left.Cursor = i
			break
		}
	}
	wantAlpha := filepath.Clean(alpha)
	if got := filepath.Clean(left.PathString()); got != wantAlpha {
		t.Fatalf("left cwd = %q want %q", got, wantAlpha)
	}

	app.dispatch(keymap.ActionPanelOpenActivePathInOther)
	applyNextInterruptEvent(t, app, screen) // async load, Secondary mirrors active path

	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != wantAlpha {
		t.Fatalf("right panel path = %q want active cwd %q", got, wantAlpha)
	}
}

func TestOpenInOtherPanelDisablesSync(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	gamma := filepath.Join(alpha, "gamma")
	if err := os.MkdirAll(gamma, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	selectPanelEntryByName(t, left, "alpha")
	app.model.ActivePanel = ui.PrimaryPanel
	app.model.SyncFollowEnabled = true
	app.model.SyncFollowPanel = ui.PrimaryPanel

	app.dispatch(keymap.ActionPanelOpenDirInOther)

	if app.model.SyncFollowEnabled {
		t.Fatal("sync should be disabled after open-dir-in-other")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyWarn {
		t.Fatalf("message urgency = %v, want warn", app.model.MessageUrgency)
	}
	if !strings.Contains(app.model.Message, "sync disabled") {
		t.Fatalf("message = %q, want sync disabled notice", app.model.Message)
	}

	app.model.SyncFollowEnabled = true
	app.model.SyncFollowPanel = ui.PrimaryPanel
	if _, err := left.Enter(app.activeViewportRows()); err != nil {
		t.Fatal(err)
	}
	app.dispatch(keymap.ActionPanelOpenActivePathInOther)

	if app.model.SyncFollowEnabled {
		t.Fatal("sync should be disabled after open-active-path-in-other")
	}
	if !strings.Contains(app.model.Message, "sync disabled") {
		t.Fatalf("message after open-active-path = %q, want sync disabled notice", app.model.Message)
	}
}

func navigatePanelIntoDir(t *testing.T, app *App, screen tcell.SimulationScreen, panelID int, name string) {
	t.Helper()
	p := app.panelByID(panelID)
	selectPanelEntryByName(t, p, name)
	app.model.ActivePanel = panelID
	app.dispatch(keymap.ActionNavOpen)
	applyNextInterruptEvent(t, app, screen) // async load triggered by entering the directory
}

// Regression: Parent must center the exited directory in the file list on first paint (same as rename recall).
func TestParentNavigationCentersInAppViewport(t *testing.T) {
	root := t.TempDir()
	bar := filepath.Join(root, "bar")
	asdf := filepath.Join(bar, "asdf")
	if err := os.MkdirAll(asdf, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("%02d_dir", i)
		if err := os.Mkdir(filepath.Join(bar, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, bar)
	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "asdf")
	if _, err := app.activePanel().Enter(app.activeViewportRows()); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, entering asdf
	app.dispatch(keymap.ActionNavParent)
	applyNextInterruptEvent(t, app, screen) // async load, Parent back to bar
	app.render()
	p := app.activePanel()
	vr := app.activeViewportRows()
	if vr < 1 {
		t.Fatalf("viewportRows = %d", vr)
	}
	entry, ok := p.CurrentEntry()
	if !ok || entry.Name != "asdf" {
		t.Fatalf("highlight = %q ok=%v, want asdf", entry.Name, ok)
	}
	row := p.Cursor - p.ScrollOffset
	mid := vr / 2
	if row != mid && row != vr-1 {
		t.Fatalf("after Parent: viewport row = %d, want %d (centered) or %d (tail); cursor=%d scroll=%d vr=%d",
			row, mid, vr-1, p.Cursor, p.ScrollOffset, vr)
	}
	app.reconcileAfterEvent()
	row = p.Cursor - p.ScrollOffset
	if row != mid && row != vr-1 {
		t.Fatalf("after second reconcile: viewport row = %d, want %d or %d; cursor=%d scroll=%d",
			row, mid, vr-1, p.Cursor, p.ScrollOffset)
	}
}

// Regression: Parent must re-resolve viewport rows after chdir when the selections strip layout changes.
func TestParentCentersWhenSelectionsStripShrinksAfterChdir(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("%02d_dir", i)
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	other := filepath.Join(root, "walnut.txt")
	writeFile(t, other)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	left := app.panelByID(ui.PrimaryPanel)
	if left.SelectedPaths == nil {
		left.SelectedPaths = make(map[string]bool)
	}
	for i := 0; i < 4; i++ {
		p := filepath.Join(root, fmt.Sprintf("peer%d.txt", i))
		writeFile(t, p)
		left.SelectedPaths[p] = true
	}
	left.SelectedPaths[other] = true

	if ui.SelectionsStripLayoutItemCount(left, ui.PrimaryPanel, ui.PrimaryPanel, false) != 0 {
		t.Fatal("strip should be hidden while cross-dir selections are in the current directory")
	}
	selectPanelEntryByName(t, left, "sub")
	if _, err := left.Enter(app.activeViewportRows()); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, entering sub
	if ui.SelectionsStripLayoutItemCount(left, ui.PrimaryPanel, ui.PrimaryPanel, false) == 0 {
		t.Fatal("strip should be visible after entering sub with cross-dir selection")
	}

	if left.FileListViewportRows == nil {
		t.Fatal("FileListViewportRows callback not wired")
	}
	staleVR := app.panelViewportRows(ui.PrimaryPanel) // still in sub: includes selections strip
	origViewport := left.FileListViewportRows
	var scrollPath string
	var scrollVR int
	left.FileListViewportRows = func() int {
		scrollPath = left.PathString()
		scrollVR = origViewport()
		return scrollVR
	}
	if err := left.Parent(staleVR); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen) // async load, Parent back to root
	vr := app.activeViewportRows()
	if staleVR >= vr {
		t.Fatalf("staleVR = %d, want smaller than post-parent %d (strip must shrink file list)", staleVR, vr)
	}
	if ui.SelectionsStripLayoutItemCount(left, ui.PrimaryPanel, ui.PrimaryPanel, false) != 0 {
		t.Fatal("strip should be hidden in parent after chdir")
	}
	if vr != ui.FileListViewportRows(
		app.layoutForTerminalSize(80, 24).Primary,
		left,
		ui.PrimaryPanel,
		ui.PrimaryPanel,
		false,
		app.selectionsStripSplitParams(ui.PrimaryPanel, 0),
	) {
		t.Fatalf("viewportRows = %d, want post-parent file list rows %d", vr,
			ui.FileListViewportRows(app.layoutForTerminalSize(80, 24).Primary, left, ui.PrimaryPanel, ui.PrimaryPanel, false, app.selectionsStripSplitParams(ui.PrimaryPanel, 0)))
	}
	entry, ok := left.CurrentEntry()
	if !ok || entry.Name != "sub" {
		t.Fatalf("highlight = %q ok=%v, want sub", entry.Name, ok)
	}
	if scrollPath != left.PathString() {
		t.Fatalf("scrollPath = %q, want parent %q", scrollPath, left.PathString())
	}
	if scrollVR != vr {
		t.Fatalf("scrollVR = %d, want live post-parent %d", scrollVR, vr)
	}
	// Regression: first list navigation must not re-scroll using the pre-Parent strip viewport.
	beforeScroll := left.ScrollOffset
	left.EnsureCursorInViewport(staleVR)
	if left.ScrollOffset != beforeScroll {
		t.Fatalf("stale viewport %d changed ScrollOffset %d -> %d; cursor=%d vr=%d",
			staleVR, beforeScroll, left.ScrollOffset, left.Cursor, vr)
	}
}
