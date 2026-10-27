package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/theme"
)

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
// shifting by a column between a full render and a progress-only partial paint when the
// status_command output is truncated.
func TestStatusCommandJobsStripStableAcrossPartialPaint(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width = 160
	screen.SetSize(width, 30)

	cfg := config.Default()
	cfg.StatusCommand.Command = "true"
	cfg.StatusCommand.MaxWidth = 12
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	app.statusCmdText = statusCommandLine("harbor lantern meadow orchard", cfg.StatusCommand.MaxWidth)
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
	if lay := app.layoutForTerminalSize(width, 30); lay.StatusCmd.Width != cfg.StatusCommand.MaxWidth {
		t.Fatalf("StatusCmd.Width = %d, want %d", lay.StatusCmd.Width, cfg.StatusCommand.MaxWidth)
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
