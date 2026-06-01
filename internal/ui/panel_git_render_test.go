package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestPanelListNameWidthReservesGitColumn(t *testing.T) {
	const rowW = 40
	noGit := panelListNameWidth(rowW, panel.ListFormatMtime, false, false)
	withGit := panelListNameWidth(rowW, panel.ListFormatMtime, false, true)
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

func TestGitNotModifiedUsesUsageForegroundUnderDiskMeter(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 60, 10
	screen.SetSize(width, height)

	styles := theme.Default()
	wantUsageFG, wantUsageBG, _ := styles.PanelUsageNormal.Decompose()
	gitNotModifiedFG, _, _ := styles.PanelGitNotModified.Decompose()

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
	drawPanel(screen, rect, state, true, false, styles, false, "",
		painter, false, nil, true, LeftPanel, nil, -1, -1, nil, false, false, false, LeftPanel, "", false)

	rowY := rect.Y + 2 // first data row (large directory)
	gitCol := rect.X + 1
	ch, style, _ := screen.Get(gitCol, rowY)
	r, _ := utf8.DecodeRuneInString(ch)
	if r != '-' {
		t.Fatalf("git staged cell = %q, want '-'", ch)
	}
	gotFG, gotBG, _ := style.Decompose()
	if gotFG != wantUsageFG {
		t.Fatalf("git '-' foreground = %v, want usage.normal %v (panel.git.not_modified is %v)", gotFG, wantUsageFG, gitNotModifiedFG)
	}
	if gotFG == gitNotModifiedFG && gitNotModifiedFG == wantUsageBG {
		t.Fatalf("git '-' still using panel.git.not_modified foreground %v that matches usage background", gitNotModifiedFG)
	}
	if gotBG != wantUsageBG {
		t.Fatalf("git '-' background = %v, want usage.normal %v", gotBG, wantUsageBG)
	}
	if gotFG == gotBG {
		t.Fatalf("git '-' foreground matches background (%v)", gotFG)
	}
}
