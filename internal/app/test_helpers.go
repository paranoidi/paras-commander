package app

import (
	"fmt"
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
	"github.com/paranoidi/paras-commander/internal/theme"
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

	styleKeys := []string{
		"menu.bar", "menu.bar.selected", "menu.dropdown", "menu.dropdown.selected",
		"menu.dropdown.frame", "menu.bar.accent", "menu.bar.alert", "menu.dropdown.accent",
		"menu.detail", "menu.spinner", "menu.progress.done", "menu.progress.remaining",
		"menu.job.scanning", "menu.job.queued", "menu.job.running", "menu.job.paused", "menu.job.canceled",
		"menu.job.failed", "menu.job.decision", "menu.job.completed",
		"panel.active.frame", "panel.inactive.frame", "panel.active.surface", "panel.inactive.surface",
		"panel.active.title", "panel.inactive.title", "panel.active.disk_usage_overview", "panel.inactive.disk_usage_overview",
		"panel.active.header", "panel.inactive.header",
		"panel.active.row.cursor", "panel.active.row.cursor.selected",
		"panel.active.usage.cursor", "panel.active.usage.cursor.selected",
		"panel.inactive.row.cursor", "panel.inactive.row.cursor.selected",
		"panel.inactive.usage.cursor", "panel.inactive.usage.cursor.selected",
		"panel.row.file", "panel.row.directory", "panel.row.symlink", "panel.row.selected",
		"panel.text", "panel.sync.indicator",
		"panel.bottom.indicator.selections", "panel.bottom.indicator.gitignore", "panel.bottom.indicator.stash",
		"panel.blocked.frame", "panel.blocked.surface", "panel.blocked.title",
		"panel.blocked.disk_usage_overview",
		"panel.blocked.header", "panel.blocked.row.file", "panel.blocked.row.directory",
		"panel.blocked.row.symlink", "panel.blocked.row.selected", "panel.blocked.row.cursor",
		"panel.blocked.row.cursor.selected", "panel.blocked.text",
		"panel.folder.diskscan", "panel.folder.diskscan_excluded",
		"panel.usage.normal", "panel.usage.selected",
		"panel.git.not_modified", "panel.git.new", "panel.git.modified", "panel.git.deleted",
		"panel.git.renamed", "panel.git.typechange", "panel.git.ignored", "panel.git.conflicted",
		"fuzzy.input", "fuzzy.input.nomatch", "fuzzy.highlight", "fuzzy.highlight.cursor",
		"dialog.frame", "dialog.title", "dialog.text", "dialog.surface", "dialog.accent",
		"dialog.input.active", "dialog.input.active.placeholder", "dialog.input.active.error", "dialog.input.inactive",
		"dialog.input.inactive.placeholder", "dialog.input.inactive.error", "dialog.button.inactive", "dialog.button.active",
		"dialog.option.inactive", "dialog.option.active", "dialog.option.selected",
		"message.info", "message.warn", "message.error",
		"jobs.row", "jobs.running", "jobs.done", "jobs.failed",
		"jobs.progress.track", "jobs.progress.fill", "jobs.progress.label.on_fill",
		"jobs.progress.label.on_track",
		"jobs.icons.scanning", "jobs.icons.queued", "jobs.icons.ongoing", "jobs.icons.paused", "jobs.icons.stopped",
		"jobs.icons.error", "jobs.icons.input_required", "jobs.icons.completed",
		"footer.key", "footer.label",
	}
	var buf strings.Builder
	buf.WriteString(`name = "test-theme"

[palette]
black = "#000000"
white = "#ffffff"
yellow = "#ffff00"

`)
	writeTestThemeStyleSections(&buf, styleKeys)

	dir := t.TempDir()
	path := filepath.Join(dir, "test-theme.toml")
	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	th, err := theme.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	paths := config.Paths{ThemesDir: dir}.WithResolvedLocations()
	return th, paths
}

func newScreen(t *testing.T, w, h int) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init() error = %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(w, h)
	return screen
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
	t.Helper()
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
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

func writeTestThemeStyleSections(buf *strings.Builder, fullKeys []string) {
	sectionOrder := []string{"menu", "panel", "dialog", "jobs", "message", "footer", "fuzzy"}
	bySection := map[string][]string{}
	for _, key := range fullKeys {
		for _, root := range sectionOrder {
			prefix := root + "."
			if strings.HasPrefix(key, prefix) {
				bySection[root] = append(bySection[root], strings.TrimPrefix(key, prefix))
				break
			}
		}
	}
	for _, root := range sectionOrder {
		keys := bySection[root]
		if len(keys) == 0 {
			continue
		}
		fmt.Fprintf(buf, "[%s]\n", root)
		for _, rel := range keys {
			fmt.Fprintf(buf, "%s = { fg = \"white\", bg = \"black\" }\n", rel)
		}
		buf.WriteString("\n")
	}
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
