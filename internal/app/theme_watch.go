package app

import (
	"os"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// themeWatchInterval is how often the theme picker stats the highlighted theme's file.
const themeWatchInterval = 500 * time.Millisecond

// themeFileChangedPayload wakes the event loop when the highlighted theme's file changed on disk.
type themeFileChangedPayload struct{ Name string }

// themeFileSig identifies one revision of a theme file.
type themeFileSig struct {
	mod  time.Time
	size int64
}

// themeWatcher remembers the last revision seen of the highlighted theme's file. It is owned by
// the polling goroutine; every disk access goes through its funcs so tests can fake them.
type themeWatcher struct {
	name    func() string                          // the highlighted theme
	path    func(name string) string               // its source file, "" when embedded
	stat    func(path string) (themeFileSig, bool) // ok is false when the file cannot be read
	changed func(name string)                      // a new revision appeared

	cur  string
	file string
	last themeFileSig
	have bool
}

// check stats the highlighted theme's file and reports a revision that differs from the one seen
// before: a distinct revision is reported once, however often it is polled. The first look at a
// theme only records its revision.
func (w *themeWatcher) check() {
	if name := w.name(); name != w.cur {
		w.cur, w.file, w.have = name, "", false
		if name != "" {
			w.file = w.path(name)
		}
	}
	if w.file == "" {
		return
	}
	sig, ok := w.stat(w.file)
	if !ok {
		return
	}
	if w.have && sig != w.last {
		w.changed(w.cur)
	}
	w.last, w.have = sig, true
}

// run checks once, then on every tick, until stop closes.
func (w *themeWatcher) run(stop <-chan struct{}, tick <-chan time.Time, done chan<- struct{}) {
	defer close(done)
	w.check()
	for {
		select {
		case <-stop:
			return
		case <-tick:
			w.check()
		}
	}
}

func statThemeFile(path string) (themeFileSig, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return themeFileSig{}, false
	}
	return themeFileSig{mod: info.ModTime(), size: info.Size()}, true
}

// themeWatch is the running poll of the theme picker; nil while auto reload is off or the dialog
// is closed.
type themeWatch struct {
	stop chan struct{}
	done chan struct{}
	name atomic.Pointer[string] // the highlighted theme, kept current by the event loop
}

// startThemeWatch polls the highlighted theme's file in its own goroutine until stopThemeWatch.
func (a *App) startThemeWatch() {
	a.stopThemeWatch()
	w := &themeWatch{stop: make(chan struct{}), done: make(chan struct{})}
	name := a.themeDialogSelectedName()
	w.name.Store(&name)
	a.themeWatch = w
	dir := a.paths.ThemesDir
	stat := a.themeWatchStat
	if stat == nil {
		stat = statThemeFile
	}
	watcher := &themeWatcher{
		name: func() string { return *w.name.Load() },
		path: func(name string) string { return theme.UserThemePath(dir, name) },
		stat: stat,
		changed: func(name string) {
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(themeFileChangedPayload{Name: name}))
		},
	}
	ticker := time.NewTicker(themeWatchInterval)
	go func() {
		defer ticker.Stop()
		watcher.run(w.stop, ticker.C, w.done)
	}()
}

// stopThemeWatch ends the poll, if any.
func (a *App) stopThemeWatch() {
	if a.themeWatch != nil {
		close(a.themeWatch.stop)
		a.themeWatch = nil
	}
}

// setThemeWatchName points the poll at the theme now highlighted.
func (a *App) setThemeWatchName(name string) {
	if a.themeWatch != nil {
		a.themeWatch.name.Store(&name)
	}
}

// applyThemeFileChanged reloads and re-previews the theme whose file changed, when it is still
// the highlighted one (the same path as the auto reload toggle).
func (a *App) applyThemeFileChanged(d themeFileChangedPayload) bool {
	if !a.model.ThemeDialog.Open || d.Name != a.themeDialogSelectedName() {
		return false
	}
	a.previewThemeAtSelection()
	return true
}
