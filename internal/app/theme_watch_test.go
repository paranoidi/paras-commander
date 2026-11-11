package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// fakeThemeFiles is a themeWatcher whose highlighted theme and file revision the test controls.
type fakeThemeFiles struct {
	name    string
	paths   map[string]string
	sigs    map[string]themeFileSig
	changed []string
}

func (f *fakeThemeFiles) watcher() *themeWatcher {
	return &themeWatcher{
		name: func() string { return f.name },
		path: func(name string) string { return f.paths[name] },
		stat: func(path string) (themeFileSig, bool) {
			sig, ok := f.sigs[path]
			return sig, ok
		},
		changed: func(name string) { f.changed = append(f.changed, name) },
	}
}

func TestThemeWatcherReportsEachNewRevisionOnce(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0)
	f := &fakeThemeFiles{
		name:  "amber",
		paths: map[string]string{"amber": "/themes/amber.toml", "birch": "/themes/birch.toml"},
		sigs: map[string]themeFileSig{
			"/themes/amber.toml": {mod: base, size: 10},
			"/themes/birch.toml": {mod: base, size: 20},
		},
	}
	w := f.watcher()

	w.check() // first look only records the revision
	w.check()
	if len(f.changed) != 0 {
		t.Fatalf("changed without a new revision: %v", f.changed)
	}

	f.sigs["/themes/amber.toml"] = themeFileSig{mod: base.Add(time.Second), size: 10}
	w.check()
	w.check() // the same revision is not reported again
	if len(f.changed) != 1 || f.changed[0] != "amber" {
		t.Fatalf("changed = %v, want [amber] once", f.changed)
	}

	// A new highlight follows the new file: its first look records, a later change reports.
	f.name = "birch"
	w.check()
	f.sigs["/themes/amber.toml"] = themeFileSig{mod: base.Add(2 * time.Second), size: 11}
	w.check()
	if len(f.changed) != 1 {
		t.Fatalf("changed = %v, want the old theme's file ignored", f.changed)
	}
	f.sigs["/themes/birch.toml"] = themeFileSig{mod: base.Add(time.Second), size: 20}
	w.check()
	if len(f.changed) != 2 || f.changed[1] != "birch" {
		t.Fatalf("changed = %v, want [amber birch]", f.changed)
	}
}

func TestThemeWatcherSkipsEmbeddedAndUnreadable(t *testing.T) {
	t.Parallel()
	f := &fakeThemeFiles{name: "embedded", paths: map[string]string{"gone": "/themes/gone.toml"}}
	w := f.watcher()
	w.check() // no file: nothing to stat
	f.name = "gone"
	w.check() // the file cannot be read
	w.check()
	if len(f.changed) != 0 {
		t.Fatalf("changed = %v, want none", f.changed)
	}
}

func TestThemeWatcherRunStopsWhenAsked(t *testing.T) {
	t.Parallel()
	f := &fakeThemeFiles{}
	stop, done := make(chan struct{}), make(chan struct{})
	tick := make(chan time.Time)
	go f.watcher().run(stop, tick, done)
	tick <- time.Now() // the loop is up and takes ticks
	close(stop)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher still running after stop")
	}
}

func newThemeWatchTestApp(t *testing.T, themesDir string) *App {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 20)
	defaultTheme := theme.Default()
	return newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: config.Default(),
		Theme:  defaultTheme,
		Paths:  config.Paths{ThemesDir: themesDir}.WithResolvedLocations(),
		ThemeChoices: []theme.NamedTheme{
			{Name: defaultTheme.Name, Label: "Default", Theme: defaultTheme},
		},
	})
}

func TestThemeWatchRunsOnlyWhileDialogOpenWithAutoReload(t *testing.T) {
	app := newThemeWatchTestApp(t, t.TempDir())
	ended := func(w *themeWatch) bool {
		select {
		case <-w.done:
			return true
		case <-time.After(5 * time.Second):
			return false
		}
	}

	if app.themeWatch != nil {
		t.Fatal("watch running with the dialog closed")
	}
	app.openThemeDialog()
	first := app.themeWatch
	if first == nil || !app.model.ThemeDialog.AutoReload {
		t.Fatal("opening the dialog should start the watch (auto reload defaults on)")
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if app.themeWatch != nil || app.model.ThemeDialog.AutoReload || !ended(first) {
		t.Fatal("auto reload off should stop the watch")
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	second := app.themeWatch
	if second == nil || second == first {
		t.Fatal("auto reload on should start a new watch")
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.ThemeDialog.Open || app.themeWatch != nil || !ended(second) {
		t.Fatal("closing the dialog should stop the watch")
	}

	app.openThemeDialog() // each opening starts with auto reload on again
	if app.themeWatch == nil {
		t.Fatal("reopened dialog should watch")
	}
	app.closeThemeDialog()
}

func TestThemeFileChangedPayloadReloadsOnlyTheHighlightedTheme(t *testing.T) {
	app := newThemeWatchTestApp(t, t.TempDir())
	app.openThemeDialog()
	defer app.closeThemeDialog()
	before := app.styles

	// A change of some other theme's file does nothing.
	if app.applyThemeFileChanged(themeFileChangedPayload{Name: "not-highlighted"}) {
		t.Fatal("payload for another theme was applied")
	}
	// The highlighted one re-previews (here from the embedded theme: same styles back).
	if !app.applyThemeFileChanged(themeFileChangedPayload{Name: app.themeDialogSelectedName()}) {
		t.Fatal("payload for the highlighted theme was ignored")
	}
	if app.styles.Name != before.Name {
		t.Fatalf("styles after reload = %q, want %q", app.styles.Name, before.Name)
	}

	app.closeThemeDialog()
	if app.applyThemeFileChanged(themeFileChangedPayload{Name: before.Name}) {
		t.Fatal("payload applied with the dialog closed")
	}
}

func TestThemeDialogPreviewPageCyclesBackwards(t *testing.T) {
	app := newThemeWatchTestApp(t, t.TempDir())
	app.openThemeDialog()
	defer app.closeThemeDialog()
	n := dialog.ThemePreviewPageCount()
	shiftF6 := tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModShift)
	app.handleKey(shiftF6)
	if app.model.ThemeDialog.Page != n-1 {
		t.Fatalf("Shift+F6 on page 1 gave page %d, want the last (%d)", app.model.ThemeDialog.Page, n-1)
	}
	for want := n - 2; want >= 0; want-- {
		app.handleKey(shiftF6)
		if app.model.ThemeDialog.Page != want {
			t.Fatalf("Shift+F6 gave page %d, want %d", app.model.ThemeDialog.Page, want)
		}
	}
}

func TestThemeDialogPreviewPageCycles(t *testing.T) {
	app := newThemeWatchTestApp(t, t.TempDir())
	app.openThemeDialog()
	defer app.closeThemeDialog()
	if app.model.ThemeDialog.Page != 0 {
		t.Fatal("the picker should open on preview page 1")
	}
	for i := 1; i <= dialog.ThemePreviewPageCount(); i++ {
		app.handleKey(tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone))
		if want := i % dialog.ThemePreviewPageCount(); app.model.ThemeDialog.Page != want {
			t.Fatalf("after %d F6 presses page = %d, want %d (wraps after the last page)", i, app.model.ThemeDialog.Page, want)
		}
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone))
	app.closeThemeDialog()
	app.openThemeDialog()
	if app.model.ThemeDialog.Page != 0 {
		t.Fatal("the picker should open on page 1 every time")
	}
}
