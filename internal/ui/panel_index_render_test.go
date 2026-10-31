package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestPanelIndexStripPaintsStateIcons(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 60, 12
	screen.SetSize(width, height)

	styles := theme.Default()
	styles.UseNerdfontIcons = true
	root := "/vol"
	states := []string{"indexed", "changed", "new", "missing", "excluded", "tagged", "differs", "scanning"}
	var entries []localfs.Entry
	for i, st := range states {
		name := string(rune('a'+i)) + "willow.txt"
		entries = append(entries, localfs.Entry{Name: name, Path: root + "/" + name, Type: localfs.EntryFile, IndexStatus: st})
	}
	entries = append(entries, localfs.Entry{Name: "zplain.txt", Path: root + "/zplain.txt", Type: localfs.EntryFile})
	state := panel.State{Path: pathloc.MustParse(root), WithIndex: true, Entries: entries, Cursor: len(entries) - 1}
	// a task job (scan) works on the "scanning" row: the icon follows the job, not the status
	scanning := root + "/" + string(rune('a'+len(states)-1)) + "willow.txt"
	marks := []JobPathMark{{Type: string(jobs.TypeTask), Status: string(jobs.StatusRunning), Sources: []string{scanning}}}
	rect := Rect{X: 0, Y: 0, Width: width, Height: height}
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout(), JobMarks: marks})

	// header: the fileindex icon takes the git column's place
	if ch, _, _ := screen.Get(rect.X+1, rect.Y+1); ch != string([]rune(styles.IconFileIndex())[0]) {
		t.Errorf("header cell = %q, want the fileindex icon %q", ch, styles.IconFileIndex())
	}
	for i, st := range states {
		ch, style, _ := screen.Get(rect.X+1, rect.Y+2+i)
		r, _ := utf8.DecodeRuneInString(ch)
		if want := styles.IconFileIndexState(st); r != want {
			t.Errorf("%s row icon = %q, want %q", st, r, want)
		}
		wantFG, _, _ := styles.PanelFileIndexStyle(st).Decompose()
		if gotFG, _, _ := style.Decompose(); gotFG != wantFG {
			t.Errorf("%s row icon fg = %v, want %v", st, gotFG, wantFG)
		}
	}
	if ch, _, _ := screen.Get(rect.X+1, rect.Y+2+len(states)); ch != " " {
		t.Errorf("row without an index state shows %q, want a blank cell", ch)
	}
}

func TestIndexRowStateScanningOverridesOnlyIndexedRows(t *testing.T) {
	t.Parallel()
	marks := []JobPathMark{{Type: string(jobs.TypeTask), Status: string(jobs.StatusRunning), Sources: []string{"/vol/thistle"}}}
	if got := indexRowState(localfs.Entry{Path: "/vol/thistle/bramble.txt", IndexStatus: "indexed"}, marks); got != "scanning" {
		t.Fatalf("row under a running scan = %q, want scanning", got)
	}
	if got := indexRowState(localfs.Entry{Path: "/vol/thistle/bramble.txt"}, marks); got != "" {
		t.Fatalf("row without index state = %q, want blank", got)
	}
	queued := []JobPathMark{{Type: string(jobs.TypeTask), Status: string(jobs.StatusQueued), Sources: []string{"/vol/thistle"}}}
	if got := indexRowState(localfs.Entry{Path: "/vol/thistle/bramble.txt", IndexStatus: "new"}, queued); got != "new" {
		t.Fatalf("row under a queued scan = %q, want its own state", got)
	}
}

func TestIndexStateIconsFallBackToASCII(t *testing.T) {
	t.Parallel()
	th := theme.Default()
	th.UseNerdfontIcons = false
	for _, st := range []string{"indexed", "changed", "new", "missing", "excluded", "scanning", "differs", "tagged"} {
		if r := th.IconFileIndexState(st); r == 0 || r > 0x7f {
			t.Errorf("%s without Nerd Font icons = %q, want an ASCII letter", st, r)
		}
	}
}
