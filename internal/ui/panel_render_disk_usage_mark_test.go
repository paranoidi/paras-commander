package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

type fixedSizeDiskPainter struct {
	sizes map[string]int64
}

func (p fixedSizeDiskPainter) ByteSize(path string) (int64, bool) {
	n, ok := p.sizes[path]
	return n, ok
}

func (fixedSizeDiskPainter) FileCount(string) (int64, bool) { return 0, false }

func (fixedSizeDiskPainter) PendingForPanel(string, int) bool { return false }

func (fixedSizeDiskPainter) DiskScanBusy() bool { return false }

func (fixedSizeDiskPainter) DiskScanExcluded(string, bool, uint64, bool, func(string) bool) bool {
	return false
}

func TestSubtreeSelectionMarkUsesDiskUsageBarBackground(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 60, 10
	screen.SetSize(width, height)

	styles := theme.Default()
	_, wantUsageBG, _ := styles.PanelUsageNormal.Decompose()
	_, rowDirBG, _ := styles.PanelRowDirectory.Decompose()

	root := "/vol"
	parentPath := root + "/parent"
	childPath := parentPath + "/nested.txt"
	painter := fixedSizeDiskPainter{sizes: map[string]int64{
		parentPath:      1000,
		root + "/small": 1,
	}}
	state := panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "parent", Path: parentPath, Type: localfs.EntryDirectory},
			{Name: "small", Path: root + "/small", Type: localfs.EntryFile, Size: 1},
		},
		Cursor:        1, // parent row is non-cursor so usage.normal applies on the meter
		SelectedPaths: map[string]bool{childPath: true},
	}
	rect := Rect{X: 0, Y: 0, Width: width, Height: height}
	drawPanel(screen, rect, state, true, false, styles, false, "",
		painter, false, nil, true, LeftPanel, nil, -1, -1, nil, false, false, false, LeftPanel, "", false, uiscrollbar.StyleNone, true)

	rowY := rect.Y + 2
	markCol := -1
	for col := rect.X + 1; col < rect.X+rect.Width-1; col++ {
		ch, _, _ := screen.Get(col, rowY)
		r, _ := utf8.DecodeRuneInString(ch)
		if r == '○' {
			markCol = col
			break
		}
	}
	if markCol < 0 {
		t.Fatal("subtree selection mark ○ not found on directory row")
	}
	_, markStyle, _ := screen.Get(markCol, rowY)
	_, gotBG, _ := markStyle.Decompose()
	if gotBG != wantUsageBG {
		t.Fatalf("○ background = %v, want panel.usage.normal %v (row.directory bg is %v)", gotBG, wantUsageBG, rowDirBG)
	}
	if gotBG == rowDirBG && wantUsageBG != rowDirBG {
		t.Fatalf("○ still using row.directory background %v", rowDirBG)
	}
}
