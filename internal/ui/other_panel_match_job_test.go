package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// TestOtherPanelMatchMarkSkipsRowsUnderActiveJob: the other-panel match mark is redundant on
// rows a copy is working on (source and destination side), but stays on a source still queued
// and on rows the job does not touch.
func TestOtherPanelMatchMarkSkipsRowsUnderActiveJob(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 100, 12
	screen.SetSize(width, height)

	mk := func(dir string) panel.State {
		s := panel.State{Path: pathloc.MustParse(dir)}
		for _, n := range []string{"acorn", "bramble", "clover"} {
			s.Entries = append(s.Entries, localfs.Entry{Name: n, Path: dir + "/" + n, Type: localfs.EntryDirectory})
		}
		return s
	}
	th := theme.Default()
	model := Model{
		Primary:              mk("/orchard"),
		Secondary:            mk("/meadow"),
		ActivePanel:          PrimaryPanel,
		UseNerdfontIcons:     true,
		OtherPanelMatchMarks: true,
		JobPathMarks: []JobPathMark{withIndex(JobPathMark{
			ID: "harbor", Type: string(jobs.TypeCopy), Status: string(jobs.StatusRunning),
			Sources:     []string{"/orchard/acorn", "/orchard/bramble"},
			Destination: "/meadow", DestIsDir: true,
			SourcePhases: []jobs.SourcePhase{jobs.SourceStarted, jobs.SourceQueued}, ActiveSource: 0,
		})},
	}
	Render(screen, model, th)

	mark := string(th.IconFilelistOtherMatch())
	row := func(x0, x1 int, name string) string {
		for y := range height {
			if text := tcelltest.TextAt(screen, x0, y, x1-x0); strings.Contains(text, " "+name) || strings.HasPrefix(strings.TrimLeft(text, " "), name) {
				return text
			}
		}
		t.Fatalf("no row for %s in columns %d..%d", name, x0, x1)
		return ""
	}
	half := width / 2
	for _, tc := range []struct {
		side   string
		x0, x1 int
		name   string
		want   bool
	}{
		{"source", 0, half, "acorn", false},  // being copied
		{"source", 0, half, "bramble", true}, // queued source
		{"source", 0, half, "clover", true},  // untouched
		{"destination", half, width, "acorn", false},
		{"destination", half, width, "bramble", true},
		{"destination", half, width, "clover", true},
	} {
		if got := strings.Contains(row(tc.x0, tc.x1, tc.name), mark); got != tc.want {
			t.Errorf("%s %s: match mark = %v, want %v (row %q)", tc.side, tc.name, got, tc.want, strings.TrimRight(row(tc.x0, tc.x1, tc.name), " "))
		}
	}
}
