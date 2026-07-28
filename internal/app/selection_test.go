package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func TestFilePanelPlusMinusStarSelectionShortcuts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	t.Run("minus opens unselect dialog without quick filter", func(t *testing.T) {
		if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, '-', tcell.ModNone)); quit {
			t.Fatal("handleKey('-') quit = true")
		}
		if !app.model.GroupSelect.Open || app.model.GroupSelect.Mode != "unselect" {
			t.Fatalf("want unselect group dialog, got %+v", app.model.GroupSelect)
		}
		f := app.activePanel().Filter
		if f.Active || f.Editing || f.Query != "" {
			t.Fatalf("quick filter should stay off, got %+v", f)
		}
		app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
		if app.model.GroupSelect.Open {
			t.Fatal("dialog should close on Esc")
		}
	})

	t.Run("plus opens select dialog with or without shift", func(t *testing.T) {
		for _, ev := range []*tcell.EventKey{
			tcell.NewEventKey(tcell.KeyRune, '+', tcell.ModNone),
			tcell.NewEventKey(tcell.KeyRune, '+', tcell.ModShift),
		} {
			if quit, _ := app.handleKey(ev); quit {
				t.Fatalf("handleKey(%+v) quit", ev)
			}
			if !app.model.GroupSelect.Open || app.model.GroupSelect.Mode != "select" {
				t.Fatalf("ev %+v: want select dialog, got %+v", ev, app.model.GroupSelect)
			}
			app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
		}
	})

	t.Run("star inverts selection", func(t *testing.T) {
		if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, '*', tcell.ModShift)); quit {
			t.Fatal("handleKey('*') quit = true")
		}
		if !strings.Contains(app.model.Message, "Selection inverted") {
			t.Fatalf("status message = %q, want selection inverted", app.model.Message)
		}
		f := app.activePanel().Filter
		if f.Active || f.Editing {
			t.Fatalf("invert must not open quick filter, got %+v", f)
		}
	})
}

func TestGroupSelectPatternCtrlLAndWordNav(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openGroupSelect("select", "panel")

	for _, r := range "ab cd" {
		app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	gs := &app.model.GroupSelect
	if gs.Text != "ab cd" || gs.TextCursor != 5 {
		t.Fatalf("after type: text=%q cursor=%d", gs.Text, gs.TextCursor)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModAlt))
	if gs.TextCursor != 3 {
		t.Fatalf("after Alt+b: cursor=%d want 3", gs.TextCursor)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone))
	if gs.Text != "" || gs.TextCursor != 0 || gs.TextScroll != 0 {
		t.Fatalf("after Ctrl+L: text=%q cursor=%d scroll=%d", gs.Text, gs.TextCursor, gs.TextScroll)
	}
}

func TestGroupSelectEnterOnPatternInputConfirms(t *testing.T) {
	dir := t.TempDir()
	foo := filepath.Join(dir, "foo.txt")
	bar := filepath.Join(dir, "bar.txt")
	writeFile(t, foo)
	writeFile(t, bar)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openGroupSelect("select", "panel")
	for _, r := range "*.txt" {
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if app.model.GroupSelect.Focus != dialog.GroupSelectFocusPattern {
		t.Fatalf("focus = %d, want %d (pattern input)", app.model.GroupSelect.Focus, dialog.GroupSelectFocusPattern)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.GroupSelect.Open {
		t.Fatal("Enter on pattern input should close dialog")
	}
	p := app.activePanel()
	if p.SelectedPaths == nil || !p.SelectedPaths[foo] || !p.SelectedPaths[bar] {
		t.Fatalf("selection after Enter = %v, want foo.txt and bar.txt", p.SelectedPaths)
	}
}

func TestGroupSelectPlainTypingDoesNotTriggerShortcuts(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openGroupSelect("select", "panel")

	for _, r := range "focus" {
		app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if got := app.model.GroupSelect.Text; got != "focus" {
		t.Fatalf("pattern = %q, want focus", got)
	}
	if app.model.GroupSelect.FilesOnly || app.model.GroupSelect.CaseSensitive || app.model.GroupSelect.PatternMode != panel.GroupPatternShell {
		t.Fatalf("checkbox/mode state changed unexpectedly: %+v", app.model.GroupSelect)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModShift))
	if got := app.model.GroupSelect.Text; got != "focusF" {
		t.Fatalf("pattern after shifted letter = %q, want focusF", got)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt))
	if !app.model.GroupSelect.FilesOnly || app.model.GroupSelect.Focus != dialog.GroupSelectFocusFilesOnly {
		t.Fatalf("Alt+F should toggle Files only and focus row; got FilesOnly=%v focus=%d",
			app.model.GroupSelect.FilesOnly, app.model.GroupSelect.Focus)
	}
}

func TestGroupSelectModeShortcutKeepsPatternFocus(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openGroupSelect("select", "panel")

	for _, r := range "foo" {
		app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	gs := &app.model.GroupSelect
	if gs.Focus != dialog.GroupSelectFocusPattern {
		t.Fatalf("focus = %d, want pattern", gs.Focus)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	if gs.PatternMode != panel.GroupPatternRegex {
		t.Fatalf("mode = %v, want regex", gs.PatternMode)
	}
	if gs.Text != "foo" || gs.Focus != dialog.GroupSelectFocusPattern {
		t.Fatalf("after Alt+R: text=%q focus=%d", gs.Text, gs.Focus)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModAlt))
	if gs.PatternMode != panel.GroupPatternSimple {
		t.Fatalf("mode = %v, want simple", gs.PatternMode)
	}
}

func TestGroupSelectInvalidRegexBlocksOK(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openGroupSelect("select", "panel")

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	for _, r := range "[" {
		app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.model.GroupSelect.Open {
		t.Fatal("invalid regex should keep dialog open")
	}
}

func TestGroupSelectCheckboxFocusNavigation(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openGroupSelect("select", "panel")

	gs := &app.model.GroupSelect
	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) // pattern -> files
	if gs.Focus != dialog.GroupSelectFocusFilesOnly {
		t.Fatalf("after Down from pattern: focus = %d, want files only", gs.Focus)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) // files -> case
	if gs.Focus != dialog.GroupSelectFocusCase {
		t.Fatalf("after Down from files: focus = %d, want case sensitive", gs.Focus)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)) // case -> files
	if gs.Focus != dialog.GroupSelectFocusFilesOnly {
		t.Fatalf("after Up from case: focus = %d, want files only", gs.Focus)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)) // files -> dirs
	if gs.Focus != dialog.GroupSelectFocusDirsOnly {
		t.Fatalf("after Right from files: focus = %d, want directories only", gs.Focus)
	}
}

func setupSelectionsStripFocusTest(t *testing.T) (*App, *panel.State) {
	t.Helper()
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
	if left.SelectionsStripCount() == 0 {
		t.Fatal("expected selections strip to list beta while cwd is alpha")
	}
	app.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
	return app, left
}

func TestActiveFooterKeysSelectionsStripFocused(t *testing.T) {
	app, _ := setupSelectionsStripFocusTest(t)
	want := menu.FunctionKeysSelectionsStripView(app.keys.Global.MenuBindingLabel(keymap.ActionPanelClearSelection))
	got := app.activeFooterKeys()
	if len(got) != len(want) {
		t.Fatalf("footer len = %d, want %d: got %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Key != want[i].Key || got[i].KeyLabel != want[i].KeyLabel || got[i].Hint != want[i].Hint {
			t.Fatalf("footer key %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	var sawView, sawEdit bool
	for _, fk := range got {
		if fk.Key == tcell.KeyEsc || fk.Key == tcell.KeyF9 {
			t.Fatalf("strip footer must not list Esc/F9, got %+v", got)
		}
		if fk.Key == tcell.KeyF3 && fk.Hint == "View" {
			sawView = true
		}
		if fk.Key == tcell.KeyF4 && fk.Hint == "Edit" {
			sawEdit = true
		}
	}
	if !sawView || !sawEdit {
		t.Fatalf("strip footer must list F3 View and F4 Edit, got %+v", got)
	}
}

func TestSelectionsStripClearSelectionViaBinding(t *testing.T) {
	app, left := setupSelectionsStripFocusTest(t)
	app.dispatch(keymap.ActionPanelClearSelection)
	if len(left.SelectedPaths) != 0 {
		t.Fatalf("SelectedPaths len = %d, want 0", len(left.SelectedPaths))
	}
	if left.SelectionsStripCount() != 0 {
		t.Fatalf("SelectionsStripCount = %d, want 0", left.SelectionsStripCount())
	}
	if app.model.ActiveSubFocus != ui.SubFocusFileList {
		t.Fatalf("ActiveSubFocus = %d, want file list", app.model.ActiveSubFocus)
	}
	if !strings.Contains(app.model.Message, "Selection cleared") {
		t.Fatalf("message = %q, want Selection cleared", app.model.Message)
	}
}

// setupSelectionsStripFileFocusTest marks a file in one directory, then navigates elsewhere
// so the selections strip lists that file and keyboard focus is on the strip.
func setupSelectionsStripFileFocusTest(t *testing.T) (*App, string) {
	t.Helper()
	root := t.TempDir()
	harbor := filepath.Join(root, "harbor")
	meadow := filepath.Join(root, "meadow")
	if err := os.Mkdir(harbor, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(meadow, 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(harbor, "willow.txt")
	writeFile(t, filePath)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.model.ActivePanel = ui.PrimaryPanel
	app.model.ActiveSubFocus = ui.SubFocusFileList
	left := app.panelByID(ui.PrimaryPanel)
	if err := left.NavigateTo(harbor, "", 20); err != nil {
		t.Fatalf("NavigateTo harbor: %v", err)
	}
	selectPanelEntryByName(t, left, "willow.txt")
	if selected, _ := left.ToggleSelection(); !selected {
		t.Fatal("toggle selection on willow.txt")
	}
	if err := left.NavigateTo(meadow, "", 20); err != nil {
		t.Fatalf("NavigateTo meadow: %v", err)
	}
	if left.SelectionsStripCount() == 0 {
		t.Fatal("expected selections strip to list willow.txt while cwd is meadow")
	}
	app.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
	left.SelectionsStripCursor = 0
	return app, filePath
}

func TestSelectionsStripFileEditViaF4(t *testing.T) {
	app, filePath := setupSelectionsStripFileFocusTest(t)
	var edited string
	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		edited = path
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.dispatch(keymap.ActionFileEdit)
	if edited != filePath {
		t.Fatalf("edited = %q, want strip file %q", edited, filePath)
	}
	if app.model.ActiveSubFocus != ui.SubFocusSelectionsStrip {
		t.Fatalf("ActiveSubFocus = %d, want selections strip", app.model.ActiveSubFocus)
	}
}

func TestSelectionsStripFileViewViaF3(t *testing.T) {
	app, filePath := setupSelectionsStripFileFocusTest(t)
	app.dispatch(keymap.ActionFileView)
	if app.model.ViewMode != ui.ViewFilePreview {
		t.Fatalf("ViewMode = %v, want ViewFilePreview", app.model.ViewMode)
	}
	if !app.model.FullscreenFilePreview.Open {
		t.Fatal("fullscreen preview not open")
	}
	if app.model.FullscreenFilePreview.Path != filePath {
		t.Fatalf("preview path = %q, want strip file %q", app.model.FullscreenFilePreview.Path, filePath)
	}
}
