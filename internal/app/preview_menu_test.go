package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func openFullscreenPreviewForMenuTest(t *testing.T, app *App, path, content string) {
	t.Helper()
	app.model.ViewMode = ui.ViewFilePreview
	app.commandsMu.Lock()
	app.model.FullscreenFilePreview = ui.FilePreviewState{
		Open:         true,
		Path:         path,
		Phase:        ui.FilePreviewPhaseDone,
		CombinedText: content,
	}
	app.commandsMu.Unlock()
}

func TestColonOpensPreviewMenuOnlyInFilePreview(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	// Browser view: ':' resolves to the normal function menu, unaffected.
	if id := app.actionFromKeyEvent(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone)); id != keymap.ActionAppLeaderMenu {
		t.Fatalf("browser ':' resolves to %q, want %q", id, keymap.ActionAppLeaderMenu)
	}
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionAppLeaderMenu)
	if !app.model.LeaderMenu.Open || app.model.LeaderMenu.PreviewMenu || app.model.LeaderMenu.CopyMenu {
		t.Fatalf("LeaderMenu = %+v, want built-in function menu open", app.model.LeaderMenu)
	}
	app.closeLeaderMenu()

	// Fullscreen preview: ':' resolves to the preview menu instead.
	openFullscreenPreviewForMenuTest(t, app, path, "hello\n")
	if id := app.actionFromKeyEvent(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone)); id != keymap.ActionFileViewMenu {
		t.Fatalf("preview ':' resolves to %q, want %q", id, keymap.ActionFileViewMenu)
	}
	app.previewCtrl.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	if !app.model.LeaderMenu.Open || !app.model.LeaderMenu.PreviewMenu {
		t.Fatalf("LeaderMenu = %+v, want preview menu open", app.model.LeaderMenu)
	}
	if len(app.model.LeaderMenu.Items) != 9 {
		t.Fatalf("preview menu items = %d, want 9", len(app.model.LeaderMenu.Items))
	}
}

func TestPreviewMenuColonToggles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	openFullscreenPreviewForMenuTest(t, app, path, "hello\n")

	app.previewCtrl.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	if !app.model.LeaderMenu.Open || !app.model.LeaderMenu.PreviewMenu {
		t.Fatalf("first ':' should open preview menu, got LeaderMenu=%+v", app.model.LeaderMenu)
	}

	// Second ':' while the menu is open routes through InputModeLeaderMenu, not the preview handler.
	_, rendered := app.handleKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	if !rendered {
		t.Fatal("second ':' should render after closing the preview menu")
	}
	if app.model.LeaderMenu.Open {
		t.Fatal("second ':' should close the preview menu")
	}
	if app.model.ViewMode != ui.ViewFilePreview || !app.model.FullscreenFilePreview.Open {
		t.Fatal("closing the preview menu must not exit fullscreen preview")
	}
}

func TestPreviewMenuEscClosesMenuNotPreview(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	openFullscreenPreviewForMenuTest(t, app, path, "hello\n")

	app.previewCtrl.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	if !app.model.LeaderMenu.Open {
		t.Fatal("expected preview menu open")
	}
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.LeaderMenu.Open {
		t.Fatal("Esc should close the preview menu")
	}
	if app.model.ViewMode != ui.ViewFilePreview || !app.model.FullscreenFilePreview.Open {
		t.Fatal("Esc on the menu must not close the fullscreen preview underneath")
	}
}

func TestPreviewMenuFooterShowsOnlyEscAndF10(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	openFullscreenPreviewForMenuTest(t, app, path, "hello\n")
	app.previewCtrl.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))

	keys := app.activeFooterKeys()
	if len(keys) != 2 {
		t.Fatalf("footer keys = %+v, want exactly [Esc Close, F10 Quit]", keys)
	}
	if keys[0].Key != tcell.KeyEsc || keys[1].Key != tcell.KeyF10 {
		t.Fatalf("footer keys = %+v, want Esc then F10", keys)
	}
}

func TestPreviewMenuThemePickerLetter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	openFullscreenPreviewForMenuTest(t, app, path, "hello\n")

	app.previewCtrl.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
	if app.model.LeaderMenu.Open {
		t.Fatal("menu should close after activating an entry")
	}
	if !app.model.FilePreviewThemePicker.Open {
		t.Fatal("'t' should open the theme picker")
	}
}

func TestPreviewMenuToggleRawLetter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	openFullscreenPreviewForMenuTest(t, app, path, "# heading\n")

	app.previewCtrl.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))
	if !app.model.FullscreenFilePreviewRawMarkdown {
		t.Fatal("'r' should toggle raw markdown on")
	}
}

func TestPreviewMenuSearchStartLetter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	openFullscreenPreviewForMenuTest(t, app, path, "foo bar\nbaz\n")

	app.previewCtrl.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))
	app.commandsMu.RLock()
	search := app.model.FullscreenFilePreview.Search
	app.commandsMu.RUnlock()
	if !search.Active || !search.Editing {
		t.Fatalf("'s' should start preview search, got Search=%+v", search)
	}
}

func TestPreviewMenuEditLetter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	openFullscreenPreviewForMenuTest(t, app, path, "hello\n")

	var edited string
	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, p string) error {
		edited = p
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.previewCtrl.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))
	if edited != path {
		t.Fatalf("edited = %q, want %q", edited, path)
	}
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser after 'e' edits and closes the preview", app.model.ViewMode)
	}
}

func TestPreviewMenuDeleteLetterOpensDeleteDialogForPreviewedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	openFullscreenPreviewForMenuTest(t, app, path, "hello\n")
	app.model.FullscreenFilePreview.TitleBase = filepath.Base(path)

	app.previewCtrl.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogDelete {
		t.Fatal("'d' should open the delete dialog for the previewed file")
	}
}

func TestPreviewMenuReloadLetterRefetchesFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	openFullscreenPreviewForMenuTest(t, app, path, "original\n")

	if err := os.WriteFile(path, []byte("updated content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app.previewCtrl.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'R', tcell.ModNone))

	// Internal (Chroma) mode stores highlighted content as styled cells, not CombinedText,
	// for source-code extensions; check whichever the result populated.
	highlightedText := func(cells []previewpanel.AnsiCell) string {
		var b strings.Builder
		for _, c := range cells {
			b.WriteRune(c.R)
		}
		return b.String()
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		app.commandsMu.RLock()
		txt := app.model.FullscreenFilePreview.CombinedText
		hl := highlightedText(app.model.FullscreenFilePreview.HighlightedCells)
		app.commandsMu.RUnlock()
		if strings.Contains(txt, "updated content") || strings.Contains(hl, "updated content") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	app.commandsMu.RLock()
	finalPhase := app.model.FullscreenFilePreview.Phase
	finalErr := app.model.FullscreenFilePreview.ErrorMsg
	finalTxt := app.model.FullscreenFilePreview.CombinedText
	finalHL := highlightedText(app.model.FullscreenFilePreview.HighlightedCells)
	app.commandsMu.RUnlock()
	t.Fatalf("'R' should reload the previewed file from disk; final state: Phase=%v ErrorMsg=%q CombinedText=%q HighlightedCells=%q",
		finalPhase, finalErr, finalTxt, finalHL)
}

// diffHunkNavContent builds n short lines, long enough (with a 24-row screen) that the
// fullscreen preview cannot show it all at once and diff-hunk navigation actually has to
// scroll, rather than every target offset getting clamped straight back to 0.
func diffHunkNavContent(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line"
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestPreviewMenuDiffHunkNavTargetsFullscreenNotInactive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	content := diffHunkNavContent(60)

	app.model.ViewMode = ui.ViewFilePreview
	app.commandsMu.Lock()
	app.model.FullscreenFilePreview = ui.FilePreviewState{
		Open:          true,
		Path:          path,
		Phase:         ui.FilePreviewPhaseDone,
		CombinedText:  content,
		IsDiff:        true,
		DiffHunkLines: []int{50},
		Scroll:        0,
	}
	// The inactive-column quick view preview: TryDispatchFileView (the generic action
	// dispatcher) would wrongly target this for diff-hunk nav instead of the fullscreen
	// preview. Give it its own diff state so a misrouted call is detectable.
	app.model.FilePreview = ui.FilePreviewState{
		Open:          true,
		Path:          path,
		Phase:         ui.FilePreviewPhaseDone,
		CombinedText:  content,
		IsDiff:        true,
		DiffHunkLines: []int{50},
		Scroll:        0,
	}
	app.commandsMu.Unlock()

	app.previewCtrl.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	if !app.model.LeaderMenu.Open {
		t.Fatal("preview menu did not open")
	}
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if app.model.LeaderMenu.Open {
		t.Fatal("preview menu still open after 'n' — key did not activate an item")
	}

	app.commandsMu.RLock()
	fullscreenScroll := app.model.FullscreenFilePreview.Scroll
	inactiveScroll := app.model.FilePreview.Scroll
	app.commandsMu.RUnlock()

	if fullscreenScroll == 0 {
		t.Fatalf("'n' should scroll the fullscreen preview to the next diff hunk; message=%q", app.model.Message)
	}
	if inactiveScroll != 0 {
		t.Fatalf("'n' from the preview menu must not touch the inactive-column preview, got Scroll=%d", inactiveScroll)
	}
}

func TestPreviewMenuDiffPrevHunkLetter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	content := diffHunkNavContent(60)

	app.model.ViewMode = ui.ViewFilePreview
	app.commandsMu.Lock()
	app.model.FullscreenFilePreview = ui.FilePreviewState{
		Open:          true,
		Path:          path,
		Phase:         ui.FilePreviewPhaseDone,
		CombinedText:  content,
		IsDiff:        true,
		DiffHunkLines: []int{5},
		Scroll:        30,
	}
	app.commandsMu.Unlock()

	app.previewCtrl.HandleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModNone))

	app.commandsMu.RLock()
	scroll := app.model.FullscreenFilePreview.Scroll
	app.commandsMu.RUnlock()
	if scroll >= 30 {
		t.Fatalf("'p' should scroll back to the previous diff hunk, Scroll = %d, want < 30", scroll)
	}
}
