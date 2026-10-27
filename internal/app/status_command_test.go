package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/statuscmds"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

const statusTestTOML = `show = ["walnut"]

[[command]]
name = "walnut"
command = "echo harbor"

[[command]]
name = "pebble"
label = "Pebble"
command = "echo gravel"
color = "bright_yellow"
`

func TestStatusCommandsDialogSavesShowAndRestarts(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(configDir, "status.toml")
	if err := os.WriteFile(path, []byte(statusTestTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	opts := testOptions(dir)
	opts.Paths.ConfigDir = configDir
	app := newTestApp(t, screen, opts)
	app.config.UI.KeyRepeatDebounceMS = 0

	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool {
		return slices.Equal(statusTexts(app), []string{"harbor"})
	})
	staleGen := app.statusCmdGen

	app.handleKey(tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModAlt|tcell.ModCtrl))
	st := &app.model.StatusCommandsDialog
	if !st.Open {
		t.Fatal("status commands dialog not open after Ctrl+Alt+Y")
	}
	if len(st.Rows) != len(statuscmds.Builtins())+2 {
		t.Fatalf("rows = %+v, want built-ins + walnut + pebble", st.Rows)
	}
	app.applyStatusCommandsDialogEvent(dlg.Event{ID: "status:walnut"})
	app.applyStatusCommandsDialogEvent(dlg.Event{ID: "status:pebble"})
	app.applyStatusCommandsDialogEvent(dlg.Event{ID: dlg.OK})
	if st.Open {
		t.Fatal("dialog still open after OK")
	}

	set, err := statuscmds.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !slices.Equal(set.Show, []string{"pebble"}) {
		t.Fatalf("saved show = %v, want [pebble]", set.Show)
	}
	if app.applyStatusCommandResult(statusCommandResultPayload{Gen: staleGen, Index: 0, Text: "stale"}) {
		t.Fatal("stale-generation result applied")
	}
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool {
		return slices.Equal(statusTexts(app), []string{"gravel"})
	})
	yellow, err := app.styles.ResolveColor("bright_yellow")
	if err != nil {
		t.Fatalf("ResolveColor: %v", err)
	}
	if got := app.statusCommandSegments()[0].Style; got != app.styles.MenuStatus.Foreground(yellow) {
		t.Fatalf("pebble style = %v, want menu.status with bright_yellow foreground", got)
	}
}

// An unresolvable color keeps menu.status rather than dropping the segment.
func TestStatusCommandUnknownColorFallsBack(t *testing.T) {
	app := testAppMinimal(t)
	app.statusCmds = []*statusCmdRunner{{def: statuscmds.Def{Name: "quartz", Color: "nonesuch"}, text: "flint"}}
	segs := app.statusCommandSegments()
	if len(segs) != 1 || segs[0].Style != app.styles.MenuStatus {
		t.Fatalf("segments = %+v, want flint in menu.status", segs)
	}
}

func statusTexts(app *App) []string {
	var out []string
	for _, s := range app.statusCommandSegments() {
		out = append(out, s.Text)
	}
	return out
}

func TestStatusCommandLineFitsMaxWidth(t *testing.T) {
	const maxWidth = 12
	cases := []string{
		"harbor lantern meadow orchard\nsecond line",
		"界界界界界界界界界界", // wide cells
		"short",
	}
	for _, in := range cases {
		line := statusCommandLine(in, maxWidth)
		if w := runewidth.StringWidth(line); w > maxWidth {
			t.Fatalf("statusCommandLine(%q) = %q width %d, want <= %d", in, line, w, maxWidth)
		}
	}
	if got := statusCommandLine("harbor lantern meadow orchard", maxWidth); runewidth.StringWidth(got) != maxWidth {
		t.Fatalf("long output %q should fill max width %d", got, maxWidth)
	}
}

// TestStatusCommandJobsStripStableAcrossPartialPaint guards against the progress strip
// shifting by a column between a full render and a progress-only partial paint when a
// status command's output is truncated.
func TestStatusCommandJobsStripStableAcrossPartialPaint(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, maxWidth = 160, 12
	screen.SetSize(width, 30)

	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: config.Default(),
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	app.statusCmds = []*statusCmdRunner{{
		def:  statuscmds.Def{Name: "harbor", MaxWidth: maxWidth},
		text: statusCommandLine("harbor lantern meadow orchard", maxWidth),
	}}
	app.devDeleteBarDemoStart = time.Now()

	app.render()
	full := menuRowText(screen, width)
	if painted, _ := app.paintMenuBarJobsStripOnly(); !painted {
		t.Fatal("partial paint did not draw the jobs strip")
	}
	partial := menuRowText(screen, width)
	if full != partial {
		t.Fatalf("menu row changed after partial paint:\nfull:    %q\npartial: %q", full, partial)
	}
	// " <text> ": one gap on each side of the single segment.
	if lay := app.layoutForTerminalSize(width, 30); lay.StatusCmd.Width != maxWidth+2 {
		t.Fatalf("StatusCmd.Width = %d, want %d", lay.StatusCmd.Width, maxWidth+2)
	}
}

func menuRowText(screen tcell.SimulationScreen, width int) string {
	cells, _, _ := screen.GetContents()
	var b strings.Builder
	for x := range width {
		b.WriteString(string(cells[x].Runes))
	}
	return b.String()
}
