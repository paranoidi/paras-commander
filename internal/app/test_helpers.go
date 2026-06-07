package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/testutil"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

func waitUntilAppJobsFinished(t *testing.T, app *App, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		idle := true
		for _, j := range app.jobState.AllJobs() {
			if j != nil && !j.Status.IsFinished() {
				idle = false
				break
			}
		}
		if idle {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout waiting for jobs to finish")
}

func activateFileMenuItem(t *testing.T, app *App, shortcut rune) {
	t.Helper()
	app.dispatch(keymap.ActionAppOpenMenu)
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, shortcut, tcell.ModNone))
	if quit {
		t.Fatal("menu item should not quit")
	}
	if app.model.Menu.Open {
		t.Fatal("menu should be closed after activation")
	}
}

func loadTestTheme(t *testing.T) (theme.Theme, config.Paths) {
	t.Helper()
	data := theme.TestThemeBytes(t, nil)
	dir := t.TempDir()
	path := filepath.Join(dir, "test-theme.toml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	th, err := theme.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	paths := config.Paths{ThemesDir: dir}.WithResolvedLocations()
	return th, paths
}

func testAppMinimal(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	screen := uitest.Screen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: config.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	return app
}

func newScreen(t *testing.T, w, h int) tcell.SimulationScreen {
	return uitest.Screen(t, w, h)
}

func newApp(t *testing.T, screen tcell.SimulationScreen, dir string) *App {
	t.Helper()
	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	app.config.UI.PanelSyncFollowNavDebounceMS = 0
	t.Cleanup(func() {
		if !app.jobStopOnce {
			flushBackgroundJobs(t, app)
		}
		app.stopWorker()
	})
	return app
}

func screenLine(screen tcell.SimulationScreen, y, width int) string {
	var builder strings.Builder
	for x := range width {
		cell, _, _ := screen.Get(x, y)
		if cell == "" {
			cell = " "
		}
		builder.WriteString(cell)
	}
	return builder.String()
}

func writeFile(t *testing.T, path string) {
	testutil.WriteFile(t, path)
}

func flushBackgroundJobs(t *testing.T, app *App) {
	t.Helper()
	const maxIter = 2000
	for i := 0; i < maxIter; i++ {
		app.pollJobEvents()
		busy := false
		for _, j := range app.jobState.AllJobs() {
			switch j.Status {
			case jobs.StatusScanning, jobs.StatusQueued, jobs.StatusRunning, jobs.StatusWaitingDecision:
				busy = true
			}
			if busy {
				break
			}
		}
		if !busy {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for jobs to finish")
}

func selectPanelEntryByName(t *testing.T, p *panel.State, name string) {
	t.Helper()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == name {
			p.Cursor = i
			return
		}
	}
	t.Fatalf("entry %q not visible in panel %q", name, p.PathString())
}
