package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestFilePreviewFocusScrollAndTabReturnsToActivePanelFileList(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.patchFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = strings.Repeat("line\n", 40)
		st.Scroll = 0
	})
	app.model.ActiveSubFocus = ui.SubFocusInactivePreview

	app.dispatch(keymap.ActionNavDown)
	if app.model.FilePreview.Scroll != 1 {
		t.Fatalf("FilePreview.Scroll = %d, want 1", app.model.FilePreview.Scroll)
	}

	prevActive := app.model.ActivePanel
	app.dispatch(keymap.ActionPanelSwitch)
	if app.model.ActivePanel != prevActive {
		t.Fatalf("ActivePanel = %d, want unchanged %d after Tab from preview", app.model.ActivePanel, prevActive)
	}
	if app.model.ActiveSubFocus != ui.SubFocusFileList {
		t.Fatalf("ActiveSubFocus = %v, want SubFocusFileList", app.model.ActiveSubFocus)
	}
}

func TestMenuShortcutActivatesFullscreenFileView(t *testing.T) {
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

	app.dispatch(keymap.ActionFileView)
	if app.model.ViewMode != ui.ViewFilePreview {
		t.Fatalf("ViewMode = %v, want ViewFilePreview after file.view", app.model.ViewMode)
	}
	app.commandsMu.RLock()
	open := app.model.FullscreenFilePreview.Open
	app.commandsMu.RUnlock()
	if !open {
		t.Fatal("FullscreenFilePreview.Open = false, want true after file.view")
	}
}

func TestFullscreenFilePreviewArrowDownScrollsWithoutNavigatingList(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		writeFile(t, filepath.Join(dir, name))
	}

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

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = strings.Repeat("x\n", 200)
		st.Scroll = 0
	})

	cursorBefore := app.activePanel().Cursor
	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if got := app.activePanel().Cursor; got != cursorBefore {
		t.Fatalf("list cursor moved %d -> %d; Down must scroll preview, not nav.down", cursorBefore, got)
	}
	app.commandsMu.RLock()
	scroll := app.model.FullscreenFilePreview.Scroll
	app.commandsMu.RUnlock()
	if scroll != 1 {
		t.Fatalf("FullscreenFilePreview.Scroll = %d, want 1 after first Down", scroll)
	}
}

func TestFullscreenFilePreviewLeftBackspaceDoNotChangePanelPath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	writeFile(t, filepath.Join(sub, "a.txt"))

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
	if err := app.activePanel().Load(sub); err != nil {
		t.Fatalf("Load(sub): %v", err)
	}
	pathBefore := app.activePanel().Path

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = "x\n"
		st.Scroll = 0
	})

	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if got := app.activePanel().Path; !got.Equal(pathBefore) {
		t.Fatalf("KeyLeft changed path %q -> %q", pathBefore, got)
	}
	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if got := app.activePanel().Path; !got.Equal(pathBefore) {
		t.Fatalf("Backspace changed path %q -> %q", pathBefore, got)
	}
}

func TestFullscreenFilePreviewRightDoesNotMoveListCursor(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		writeFile(t, filepath.Join(dir, name))
	}

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

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = strings.Repeat("x\n", 200)
		st.Scroll = 0
	})

	cursorBefore := app.activePanel().Cursor
	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if got := app.activePanel().Cursor; got != cursorBefore {
		t.Fatalf("list cursor moved %d -> %d; Right must not nav.open", cursorBefore, got)
	}
}

func TestFullscreenFilePreviewDoesNotOpenMenuFromDispatch(t *testing.T) {
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

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = "x\n"
		st.Scroll = 0
	})

	app.dispatch(keymap.ActionAppOpenMenu)
	if app.model.Menu.Open {
		t.Fatal("ActionAppOpenMenu must not open menu during fullscreen file preview")
	}
	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))
	if app.model.Menu.Open {
		t.Fatal("F9 must not open menu during fullscreen file preview")
	}
	if !app.model.FilePreviewThemePicker.Open {
		t.Fatal("F9 must open inline theme picker during fullscreen file preview")
	}
	var f9Hint string
	var hasEscClose bool
	var hasEnterSave bool
	for _, fk := range app.activeFooterKeys() {
		if fk.Key == tcell.KeyF9 {
			f9Hint = fk.Hint
		}
		if fk.Hint == "Close" && fk.Key == tcell.KeyEsc {
			hasEscClose = true
		}
		if fk.Key == tcell.KeyEnter && fk.KeyLabel == "Enter" && fk.Hint == "Save" {
			hasEnterSave = true
		}
	}
	if f9Hint != "" {
		t.Fatalf("footer must not show F9 while theme picker is open, got hint %q", f9Hint)
	}
	if !hasEscClose {
		t.Fatal("footer must show Esc Close while theme picker is open")
	}
	if !hasEnterSave {
		t.Fatal("footer must show Enter Save while theme picker is open")
	}
}

func TestFullscreenFilePreviewIgnoresBrowserOnlyShortcuts(t *testing.T) {
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

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = "x\n"
		st.Scroll = 0
	})
	pathBefore := app.activePanel().PathString()

	for _, tc := range []struct {
		name string
		key  *tcell.EventKey
	}{
		{"user menu", tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone)},
		{"edit user menu", tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModShift)},
		{"refresh panel", tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt|tcell.ModCtrl)},
		{"add bookmark", tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModCtrl)},
		{"open bookmarks", tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModCtrl)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app.handleFilePreviewViewKey(tc.key)
			if app.model.QuickAction.Open {
				t.Fatal("user menu must stay closed")
			}
			if app.model.PathPicker.Open {
				t.Fatal("bookmark picker must stay closed")
			}
			if got := app.activePanel().PathString(); got != pathBefore {
				t.Fatalf("panel path changed %q -> %q", pathBefore, got)
			}
		})
	}
}

func TestFilePreviewRunGenStaleSkipsRunningPatch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.patchFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhasePending
		st.Path = path
	})
	staleGen := app.filePreviewRunGen.Add(1)
	app.filePreviewRunGen.Add(1)

	app.runPreview(context.Background(), app.previewRequest(path, 80, root, false, nil, previewTargetInactive), previewTargetInactive, staleGen)

	app.commandsMu.RLock()
	ph := app.model.FilePreview.Phase
	app.commandsMu.RUnlock()
	if ph != ui.FilePreviewPhasePending {
		t.Fatalf("Phase = %v, want Pending when run gen is stale at start", ph)
	}
}

func TestRunPreviewInternalSetsHighlightedCells(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.Preview.Mode = config.PreviewModeInternal
	app.config.Preview.LineNumbers = true

	app.patchFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhasePending
		st.Path = path
	})
	gen := app.filePreviewRunGen.Add(1)
	app.runPreview(context.Background(), app.previewRequest(path, 80, root, false, nil, previewTargetInactive), previewTargetInactive, gen)

	app.commandsMu.RLock()
	st := app.model.FilePreview
	app.commandsMu.RUnlock()
	if st.Phase != ui.FilePreviewPhaseDone {
		t.Fatalf("Phase = %v, want Done", st.Phase)
	}
	if st.Source != ui.PreviewSourceInternalHighlighted {
		t.Fatalf("Source = %v, want internal highlighted", st.Source)
	}
	if len(st.HighlightedCells) == 0 {
		t.Fatal("HighlightedCells empty, want Chroma output")
	}
}

func TestFilePreviewOverlayMapsF5ToToggleRaw(t *testing.T) {
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	id, ok := bundle.FilePreview.Lookup(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if !ok || id != keymap.ActionFileViewToggleRaw {
		t.Fatalf("FilePreview.Lookup(F5) = %q %v, want %s", id, ok, keymap.ActionFileViewToggleRaw)
	}
}

func TestToggleFilePreviewRawMarkdownFlipsAndResetsScrollForMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Path = path
		st.Phase = ui.FilePreviewPhaseDone
		st.Scroll = 5
	})

	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))

	if !app.model.FullscreenFilePreviewRawMarkdown {
		t.Fatal("FullscreenFilePreviewRawMarkdown = false, want true after F5 on a markdown file")
	}
	app.commandsMu.RLock()
	scroll := app.model.FullscreenFilePreview.Scroll
	app.commandsMu.RUnlock()
	if scroll != 0 {
		t.Fatalf("Scroll = %d, want 0 after toggling raw/rendered", scroll)
	}
}

func TestToggleFilePreviewRawMarkdownNoOpForNonMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Path = path
		st.Phase = ui.FilePreviewPhaseDone
		st.Scroll = 5
	})

	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))

	if app.model.FullscreenFilePreviewRawMarkdown {
		t.Fatal("FullscreenFilePreviewRawMarkdown = true, want false for a non-markdown file")
	}
	app.commandsMu.RLock()
	scroll := app.model.FullscreenFilePreview.Scroll
	app.commandsMu.RUnlock()
	if scroll != 5 {
		t.Fatalf("Scroll = %d, want unchanged 5 (no-op)", scroll)
	}
}

func TestToggleFilePreviewRawMarkdownNoOpForDiff(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Path = path
		st.Phase = ui.FilePreviewPhaseDone
		st.IsDiff = true
		st.Scroll = 5
	})

	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))

	if app.model.FullscreenFilePreviewRawMarkdown {
		t.Fatal("FullscreenFilePreviewRawMarkdown = true, want false while showing a diff")
	}
	app.commandsMu.RLock()
	scroll := app.model.FullscreenFilePreview.Scroll
	app.commandsMu.RUnlock()
	if scroll != 5 {
		t.Fatalf("Scroll = %d, want unchanged 5 (no-op)", scroll)
	}
}

func TestOpenFilePreviewFullscreenResetsRawMarkdownFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.model.FullscreenFilePreviewRawMarkdown = true

	app.openFilePreviewFullscreen()

	if app.model.FullscreenFilePreviewRawMarkdown {
		t.Fatal("FullscreenFilePreviewRawMarkdown = true, want reset to false on a fresh fullscreen preview")
	}
}

func TestF8DeletesOnlyPreviewedFileKeepingPanelSelection(t *testing.T) {
	dir := t.TempDir()
	previewedPath := filepath.Join(dir, "walrus.txt")
	otherPath := filepath.Join(dir, "lighthouse.txt")
	writeFile(t, previewedPath)
	writeFile(t, otherPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{previewedPath: true, otherPath: true}

	app.model.ViewMode = ui.ViewFilePreview
	app.model.FullscreenFilePreview.Path = previewedPath
	app.model.FullscreenFilePreview.TitleBase = filepath.Base(previewedPath)

	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogDelete {
		t.Fatal("F8 should open the delete dialog for the previewed file")
	}

	// Default focus is No; move to Yes then Enter confirms delete.
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after confirm")
	}
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser after delete", app.model.ViewMode)
	}
	if !p.SelectedPaths[previewedPath] || !p.SelectedPaths[otherPath] {
		t.Fatal("panel selection must stay untouched by the previewed-file delete")
	}

	flushBackgroundJobs(t, app)
	if _, err := os.Stat(previewedPath); !os.IsNotExist(err) {
		t.Fatal("previewed file should be deleted")
	}
	if _, err := os.Stat(otherPath); err != nil {
		t.Fatal("other selected file must not be deleted")
	}
}

// TestRefreshPreviewTargetAfterResizeReRunsOnlyOnWidthChange covers the decision logic used by
// the *tcell.EventResize handler: an open preview target is re-run when its currently computed
// text width differs from the width its content was last requested at, and left alone otherwise.
func TestRefreshPreviewTargetAfterResizeReRunsOnlyOnWidthChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	writeFile(t, path)
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, dir)

	app.patchFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.Path = path
	})
	tw, _, ok := app.inactivePanelPreviewLayoutMetrics(true)
	if !ok {
		t.Fatal("inactivePanelPreviewLayoutMetrics() ok = false, want true")
	}

	// Same width as last request: no re-run.
	app.previewLastWidth[previewTargetInactive] = tw
	genBefore := app.filePreviewRunGen.Load()
	app.refreshPreviewTargetAfterResize(previewTargetInactive)
	if got := app.filePreviewRunGen.Load(); got != genBefore {
		t.Fatalf("filePreviewRunGen = %d, want unchanged %d when width did not change", got, genBefore)
	}

	// Different width from last request: re-run triggered.
	app.previewLastWidth[previewTargetInactive] = tw + 1
	app.refreshPreviewTargetAfterResize(previewTargetInactive)
	if got := app.filePreviewRunGen.Load(); got != genBefore+1 {
		t.Fatalf("filePreviewRunGen = %d, want %d after width change triggers a re-run", got, genBefore+1)
	}
	if app.previewLastWidth[previewTargetInactive] != tw {
		t.Fatalf("previewLastWidth[inactive] = %d, want %d recorded from the new request", app.previewLastWidth[previewTargetInactive], tw)
	}
}

// TestFullscreenFilePreviewSearchStartTypeEnterNavigateEsc drives the "/" incremental
// search flow end to end: start search, type a query, accept with Enter, step to the next
// match with "n", then cancel the search with Esc (leaving the fullscreen preview open).
func TestFullscreenFilePreviewSearchStartTypeEnterNavigateEsc(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	// Matches are spaced far apart (well beyond one screen height of filler lines) so
	// navigating from the first to the second match must actually scroll the viewport.
	var content strings.Builder
	for i := 0; i < 3; i++ {
		content.WriteString("foo bar\n")
		content.WriteString(strings.Repeat("baz qux\n", 30))
	}

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Path = path
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = content.String()
		st.Scroll = 0
	})

	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.commandsMu.RLock()
	search := app.model.FullscreenFilePreview.Search
	app.commandsMu.RUnlock()
	if !search.Active || !search.Editing {
		t.Fatalf("after '/', Search = %+v, want Active=true Editing=true", search)
	}

	for _, r := range "foo" {
		app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.commandsMu.RLock()
	search = app.model.FullscreenFilePreview.Search
	app.commandsMu.RUnlock()
	if search.Query != "foo" || len(search.Matches) != 3 || search.Current != 0 {
		t.Fatalf("after typing \"foo\", Search = %+v, want Query=foo len(Matches)=3 Current=0", search)
	}

	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	app.commandsMu.RLock()
	search = app.model.FullscreenFilePreview.Search
	app.commandsMu.RUnlock()
	if search.Editing || !search.Active {
		t.Fatalf("after Enter, Search = %+v, want Editing=false Active=true", search)
	}

	// "n" must now navigate (not be captured as query text, since Editing is false).
	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	app.commandsMu.RLock()
	current := app.model.FullscreenFilePreview.Search.Current
	scroll := app.model.FullscreenFilePreview.Scroll
	app.commandsMu.RUnlock()
	if current != 1 {
		t.Fatalf("after 'n', Search.Current = %d, want 1", current)
	}
	if scroll == 0 {
		t.Fatal("after 'n', Scroll = 0, want the view to have scrolled to reveal match 1")
	}

	// First Esc clears the search but leaves the fullscreen preview open.
	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	app.commandsMu.RLock()
	search = app.model.FullscreenFilePreview.Search
	open := app.model.FullscreenFilePreview.Open
	app.commandsMu.RUnlock()
	if search.Active {
		t.Fatalf("after first Esc, Search.Active = true, want false")
	}
	if !open {
		t.Fatal("after first Esc, fullscreen preview should still be open")
	}

	// Second Esc closes the preview (no active search left to clear).
	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser after second Esc", app.model.ViewMode)
	}
}
