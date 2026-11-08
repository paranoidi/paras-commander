package ui

import (
	"maps"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestPanelListGitColumnActiveVisibleWhilePending(t *testing.T) {
	state := panel.State{GitColumnActive: true, GitPending: true}
	if !panelListGitColumnActive(state, false) {
		t.Fatal("git column hidden while GitPending, want stable layout before status cells load")
	}
}

func TestPanelListNameWidthReservesGitColumn(t *testing.T) {
	const rowW = 40
	noGit := panelListNameWidth(rowW, panel.ColumnsModified, false, false)
	withGit := panelListNameWidth(rowW, panel.ColumnsModified, false, true)
	want := panelListGitCells + panelListGitGap
	if noGit-withGit != want {
		t.Fatalf("name width delta = %d, want %d (git cells + gap)", noGit-withGit, want)
	}
}

func TestPanelGitCellDefaultsToNotModified(t *testing.T) {
	c := panelGitCell(localfs.Entry{Path: "/tmp/x"}, nil)
	if c.Staged != gitstatus.NotModified || c.Unstaged != gitstatus.NotModified {
		t.Fatalf("cell = %+v, want - -", c)
	}
}

// A bar icon color wins on a git cell; without one, unmodified cells take the bar's foreground
// because panel.git.not_modified matches the bar background.
func TestGitNotModifiedForegroundUnderDiskMeter(t *testing.T) {
	t.Parallel()
	withIcon := theme.Default()
	noIcon := theme.Default()
	noIcon.PanelFileIconFG = maps.Clone(noIcon.PanelFileIconFG)
	delete(noIcon.PanelFileIconFG, "panel.usage.normal")
	usageFG, _, _ := withIcon.PanelUsageNormal.Decompose()

	cases := []struct {
		name   string
		styles theme.Theme
		wantFG tcell.Color
	}{
		{"bar icon color", withIcon, withIcon.PanelFileIconFG["panel.usage.normal"]},
		{"no bar icon color", noIcon, usageFG},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotFG, gotBG := gitNotModifiedCellUnderMeter(t, tc.styles)
			_, wantBG, _ := tc.styles.PanelUsageNormal.Decompose()
			if gotFG != tc.wantFG {
				t.Fatalf("git '-' foreground = %v, want %v", gotFG, tc.wantFG)
			}
			if gotBG != wantBG {
				t.Fatalf("git '-' background = %v, want usage.normal %v", gotBG, wantBG)
			}
			if gotFG == gotBG {
				t.Fatalf("git '-' foreground matches background (%v)", gotFG)
			}
		})
	}
}

func gitNotModifiedCellUnderMeter(t *testing.T, styles theme.Theme) (fg, bg tcell.Color) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 60, 10
	screen.SetSize(width, height)

	root := "/vol"
	largePath := root + "/large"
	smallPath := root + "/small"
	painter := fixedSizeDiskPainter{sizes: map[string]int64{
		largePath: 1000,
		smallPath: 1,
	}}
	state := panel.State{
		Path:            pathloc.MustParse(root),
		GitColumnActive: true,
		Entries: []localfs.Entry{
			{Name: "large", Path: largePath, Type: localfs.EntryDirectory},
			{Name: "small", Path: smallPath, Type: localfs.EntryFile, Size: 1},
		},
		Cursor: 1, // large row is non-cursor so usage.normal applies on the meter
	}
	rect := Rect{X: 0, Y: 0, Width: width, Height: height}
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{Painter: painter, ShowDiskUsage: true, ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})

	rowY := rect.Y + 2 // first data row (large directory)
	gitCol := rect.X + 1
	ch, style, _ := screen.Get(gitCol, rowY)
	r, _ := utf8.DecodeRuneInString(ch)
	if r != '-' {
		t.Fatalf("git staged cell = %q, want '-'", ch)
	}
	fg, bg, _ = style.Decompose()
	return fg, bg
}
