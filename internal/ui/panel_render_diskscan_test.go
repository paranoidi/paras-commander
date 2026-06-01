package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

type panicDiskScanPainter struct{}

func (panicDiskScanPainter) ByteSize(string) (int64, bool)    { return 0, false }
func (panicDiskScanPainter) PendingForPanel(string, int) bool { return false }
func (panicDiskScanPainter) DiskScanBusy() bool               { return false }
func (panicDiskScanPainter) DiskScanExcluded(string, bool, uint64, bool, func(string) bool) bool {
	panic("DiskScanExcluded must not run when disk-usage metering is off")
}

func TestDrawPanelSkipsDiskScanExcludedWhenMeteringOff(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(40, 12)

	state := panel.State{
		Path: pathloc.MustParse("/mnt/nas"),
		Entries: []localfs.Entry{
			{Name: "dirA", Path: "/mnt/nas/dirA", Type: localfs.EntryDirectory},
			{Name: "file", Path: "/mnt/nas/file", Type: localfs.EntryFile},
		},
		Cursor: 0,
	}
	rect := Rect{X: 0, Y: 0, Width: 40, Height: 12}
	drawPanel(screen, rect, state, true, false, theme.Default(), true, "",
		panicDiskScanPainter{}, false, nil, false, LeftPanel, nil, -1, -1, nil, false, false, false, LeftPanel, "", false)
}
