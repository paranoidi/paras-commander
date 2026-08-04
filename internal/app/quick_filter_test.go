package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestQuickFilterFunctionKeyClosesFuzzyAndRunsFullscreenFileView(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

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

	app.activePanel().OpenFilter(app.activeViewportRows())
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))

	// F3 runs file.view (full-screen file view) after the bound action clears the filter.
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Primary.Filter.Editing || app.model.Primary.Filter.Active || app.model.Primary.Filter.Query != "" {
		t.Fatalf("filter should be cleared, got editing=%v active=%v query=%q",
			app.model.Primary.Filter.Editing, app.model.Primary.Filter.Active, app.model.Primary.Filter.Query)
	}
	if app.model.ViewMode != ui.ViewFilePreview {
		t.Fatalf("ViewMode = %v, want ViewFilePreview after F3 from quick filter", app.model.ViewMode)
	}
	app.commandsMu.RLock()
	open := app.model.FullscreenFilePreview.Open
	app.commandsMu.RUnlock()
	if !open {
		t.Fatal("FullscreenFilePreview.Open = false, want true after F3 from quick filter")
	}
}

func TestQuickFilterF9ClosesFuzzyAndOpensMenu(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

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

	app.activePanel().OpenFilter(app.activeViewportRows())
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))
	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Primary.Filter.Editing || app.model.Primary.Filter.Active {
		t.Fatal("filter should be cleared after F9")
	}
	if !app.model.Menu.Open {
		t.Fatal("menu open = false, want true after F9 from quick filter")
	}
}

func TestQuickFilterF10Quits(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

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

	app.activePanel().OpenFilter(app.activeViewportRows())
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	if !quit {
		t.Fatal("handleKey() quit = false, want true for F10 from quick filter")
	}
	if app.model.Primary.Filter.Editing || app.model.Primary.Filter.Active {
		t.Fatal("filter should be cleared before quit")
	}
}

func TestQuickFilterEmptyOverlayThenTypingEnterOnFileClearsFuzzy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.txt"))
	writeFile(t, filepath.Join(dir, "beta.txt"))

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

	app.activePanel().OpenFilter(app.activeViewportRows())
	if !app.model.Primary.Filter.Editing {
		t.Fatal("filter editing = false, want true after OpenFilter")
	}
	for _, r := range "beta" {
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.Primary.Filter.Editing || app.model.Primary.Filter.Active || app.model.Primary.Filter.Query != "" {
		t.Fatalf("filter editing=%v active=%v query=%q, want fuzzy cleared after Enter on a file",
			app.model.Primary.Filter.Editing, app.model.Primary.Filter.Active, app.model.Primary.Filter.Query)
	}
	if app.model.Primary.VisibleEntryCount() != 2 {
		t.Fatalf("visible=%d, want both files visible", app.model.Primary.VisibleEntryCount())
	}
	entry, ok := app.model.Primary.CurrentEntry()
	if !ok || entry.Name != "beta.txt" {
		t.Fatalf("CurrentEntry() = %q ok=%v, want beta.txt", entry.Name, ok)
	}
}

func TestPlainTypingStartsQuickFilterAndMovesToFirstVisibleMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "notes.txt"))
	writeFile(t, filepath.Join(dir, "src"))

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

	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))

	if !app.model.Primary.Filter.Editing || app.model.Primary.Filter.Query != "s" {
		t.Fatalf("filter editing=%v query=%q, want typing to start query s", app.model.Primary.Filter.Editing, app.model.Primary.Filter.Query)
	}
	entry, ok := app.model.Primary.CurrentEntry()
	if !ok || entry.Name != "notes.txt" {
		t.Fatalf("CurrentEntry() = %q ok=%v, want first visible match notes.txt", entry.Name, ok)
	}
}

func TestQuickFilterSpaceAppendsToQueryInsteadOfTogglingTree(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "foo bar.txt"))
	writeFile(t, filepath.Join(dir, "other.txt"))

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

	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone))
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone))
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone))
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))

	if !app.model.Primary.Filter.Editing || app.model.Primary.Filter.Query != "foo " {
		t.Fatalf("filter editing=%v query=%q, want space appended to query while typing", app.model.Primary.Filter.Editing, app.model.Primary.Filter.Query)
	}
	if app.model.Primary.ListLayout == panel.ListLayoutTree {
		t.Fatal("space during quick filter must not toggle tree view")
	}
}

func TestPlainTypingMultiLetterSelectsBestRankedMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "abzzc.txt"))
	writeFile(t, filepath.Join(dir, "abc.txt"))

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

	for _, r := range "abc" {
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	entry, ok := app.model.Primary.CurrentEntry()
	if !ok || entry.Name != "abc.txt" {
		t.Fatalf("CurrentEntry() = %q ok=%v, want best ranked abc.txt", entry.Name, ok)
	}
}

func TestQuickFilterEnterOpensDirectoryAndClearsQuery(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("Mkdir(sub): %v", err)
	}
	writeFile(t, filepath.Join(dir, "other.txt"))

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

	for _, r := range "sub" {
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	wantPath := filepath.Clean(sub)
	if got := filepath.Clean(app.model.Primary.Path.String()); got != wantPath {
		t.Fatalf("left path=%q after Enter want %q", got, wantPath)
	}
	if app.model.Primary.Filter.Active || app.model.Primary.Filter.Query != "" || app.model.Primary.Filter.Editing {
		t.Fatalf("filter cleared: active=%v query=%q editing=%v want all off",
			app.model.Primary.Filter.Active, app.model.Primary.Filter.Query, app.model.Primary.Filter.Editing)
	}
}

func TestQuickFilterInsertSelectsAndAdvancesCursor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.txt"))
	writeFile(t, filepath.Join(dir, "alpine.txt"))

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

	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))
	if app.model.Primary.Filter.Query != "a" {
		t.Fatalf("query=%q want a after typing", app.model.Primary.Filter.Query)
	}

	entryPath := filepath.Join(dir, "alpha.txt")
	if app.model.Primary.SelectedPaths[entryPath] {
		t.Fatal("alpha.txt selected before Insert, want not selected")
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if !app.model.Primary.Filter.Active {
		t.Fatal("filter closed after Insert, want open for multi-select")
	}
	if !app.model.Primary.SelectedPaths[entryPath] {
		t.Fatal("alpha.txt not selected after Insert, want selected")
	}
	if app.model.Primary.Cursor != 1 {
		t.Fatalf("cursor=%d after Insert, want 1 (moved down past filtered entry)", app.model.Primary.Cursor)
	}
}

func TestQuickFilterEmptyQueryEnterExitsEditing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.txt"))

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

	app.activePanel().OpenFilter(app.activeViewportRows())
	if !app.model.Primary.Filter.Editing {
		t.Fatal("want editing after OpenFilter")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.Primary.Filter.Editing {
		t.Fatal("want editing=false after Enter with empty query")
	}
	if app.model.Primary.Filter.Active || app.model.Primary.Filter.Query != "" {
		t.Fatalf("want no active query, got active=%v query=%q", app.model.Primary.Filter.Active, app.model.Primary.Filter.Query)
	}
}

func TestFilterModeEscCancelsInsteadOfQuitting(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.txt"))

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

	app.activePanel().OpenFilter(app.activeViewportRows())
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if quit {
		t.Fatal("handleKey(Esc) quit = true, want filter cancel")
	}
	if app.model.Primary.Filter.Editing || app.model.Primary.Filter.Active {
		t.Fatalf("filter editing=%v active=%v, want canceled", app.model.Primary.Filter.Editing, app.model.Primary.Filter.Active)
	}
}

func TestQuickFilterKeymapActionClosesFilterAndOpensDirInOtherPanel(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	writeFile(t, filepath.Join(dir, "other.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	ev, ok := app.keys.Global.FirstEventKeyForAction(keymap.ActionPanelOpenDirInOther)
	if !ok {
		t.Fatal("no key bound to ActionPanelOpenDirInOther")
	}

	app.activePanel().OpenFilter(app.activeViewportRows())
	for _, r := range "sub" {
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if app.model.Primary.Filter.Query != "sub" {
		t.Fatalf("query=%q want sub", app.model.Primary.Filter.Query)
	}
	entry, okEntry := app.model.Primary.CurrentEntry()
	if !okEntry || entry.Name != "subdir" {
		t.Fatalf("CurrentEntry() = %q ok=%v, want subdir under cursor before shortcut", entry.Name, okEntry)
	}

	quit, _ := app.handleKey(ev)
	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Primary.Filter.Editing || app.model.Primary.Filter.Active || app.model.Primary.Filter.Query != "" {
		t.Fatalf("filter should be cleared, got editing=%v active=%v query=%q",
			app.model.Primary.Filter.Editing, app.model.Primary.Filter.Active, app.model.Primary.Filter.Query)
	}
	want := filepath.Clean(sub)
	if got := filepath.Clean(app.model.Secondary.Path.String()); got != want {
		t.Fatalf("right panel path=%q want %q", got, want)
	}
}

func TestQuickFilterUpDownCyclesMatches(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "assets"))
	writeFile(t, filepath.Join(dir, "notes.txt"))
	writeFile(t, filepath.Join(dir, "src"))

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

	app.activePanel().OpenFilter(app.activeViewportRows())
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))
	if app.model.Primary.Filter.Query != "s" {
		t.Fatalf("query=%q want s", app.model.Primary.Filter.Query)
	}
	if !app.model.Primary.Filter.Editing || !app.model.Primary.Filter.Active {
		t.Fatalf("want filter editing and active, got editing=%v active=%v",
			app.model.Primary.Filter.Editing, app.model.Primary.Filter.Active)
	}

	first, ok := app.model.Primary.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false")
	}
	if first.Name != "assets" {
		t.Fatalf("CurrentEntry() = %q, want first visible match assets", first.Name)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	second, ok := app.model.Primary.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after Down")
	}
	if second.Name != "notes.txt" {
		t.Fatalf("after Down want next visible match notes.txt, got %q", second.Name)
	}
	if app.model.Primary.Filter.Query != "s" {
		t.Fatalf("query should stay set, got %q", app.model.Primary.Filter.Query)
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	third, ok := app.model.Primary.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after second Down")
	}
	if third.Name != "src" {
		t.Fatalf("after second Down want next visible match src, got %q", third.Name)
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	wrapped, ok := app.model.Primary.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after third Down")
	}
	if wrapped.Name != first.Name {
		t.Fatalf("after Down from last match want wrap to %q, got %q", first.Name, wrapped.Name)
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	backToLast, ok := app.model.Primary.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after Up from first")
	}
	if backToLast.Name != "src" {
		t.Fatalf("after Up from first match want wrap to src, got %q", backToLast.Name)
	}
}

func TestQuickFilterCtrlBackspaceClearsQuery(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.txt"))
	writeFile(t, filepath.Join(dir, "beta.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	// Type something into filter
	app.activePanel().OpenFilter(app.activeViewportRows())
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone))

	if app.model.Primary.Filter.Query != "al" {
		t.Fatalf("query=%q want al", app.model.Primary.Filter.Query)
	}
	if !app.model.Primary.Filter.Editing {
		t.Fatal("want filter editing")
	}

	// Ctrl+Backspace should clear query but keep editing
	app.handleKey(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModCtrl))

	if app.model.Primary.Filter.Query != "" {
		t.Fatalf("query=%q want empty after Ctrl+Backspace", app.model.Primary.Filter.Query)
	}
	if !app.model.Primary.Filter.Editing {
		t.Fatal("Ctrl+Backspace should keep filter in editing mode")
	}
	if app.model.Primary.Filter.Active {
		t.Fatal("Ctrl+Backspace should deactivate filter")
	}
}
