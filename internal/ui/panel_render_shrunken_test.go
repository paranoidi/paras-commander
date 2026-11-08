package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// rowTextWidthForShrinkGate mirrors drawPanel's listing text width (inner minus icon gutters).
func rowTextWidthForShrinkGate(panelRectWidth int, showFileIcons bool) int {
	interior := panelRectWidth - 2
	leftGutter := 0
	iconStrip := 0
	if showFileIcons {
		leftGutter = panelIconListLeadingGutter
		iconStrip = panelIconStripCells
	}
	return interior - leftGutter - iconStrip
}

func TestShrunkenListingGateHalfOf80WithoutFileIcons(t *testing.T) {
	const halfW = 80 / 2
	rowTW := rowTextWidthForShrinkGate(halfW, false)
	if rowTW >= config.NarrowPanelRowTextWidthThreshold {
		t.Fatalf("rowTextWidth=%d should be < threshold=%d so half of an 80-col terminal counts as shrunken (icons off)",
			rowTW, config.NarrowPanelRowTextWidthThreshold)
	}
}

func TestShrunkenListingGateHalfOf80WithFileIcons(t *testing.T) {
	const halfW = 80 / 2
	rowTW := rowTextWidthForShrinkGate(halfW, true)
	if rowTW >= config.NarrowPanelRowTextWidthThreshold {
		t.Fatalf("rowTextWidth=%d should be < threshold=%d so half of an 80-col terminal counts as shrunken (icons on)",
			rowTW, config.NarrowPanelRowTextWidthThreshold)
	}
}

func TestRenderShrunkenNameOnlyOmitsMtimeOnFirstListRow(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 30)

	mt := time.Date(1999, 6, 5, 4, 3, 0, 0, time.UTC)
	entry := localfs.Entry{
		Name: "listed.go", Path: "/tmp/listed.go", Type: localfs.EntryFile,
		Size: 4096, Mode: 0o644, ModifiedAt: mt,
	}
	model := Model{
		Primary: panel.State{
			Path:    pathloc.MustParse("/tmp"),
			Entries: []localfs.Entry{entry},
			Cursor:  0,
		},
		Secondary:            panel.State{Path: pathloc.MustParse("/tmp")},
		ActivePanel:          PrimaryPanel,
		ActiveSubFocus:       SubFocusFileList,
		HideMenuBar:          false,
		UseNerdfontIcons:     false,
		NarrowPanelsNameOnly: true,
	}

	Render(screen, model, theme.Default())

	const wantY = 3 // menu y=0, panel title y=1, header y=2, first row y=3
	var b strings.Builder
	for x := 1; x <= 38; x++ {
		ch, _, _ := screen.Get(x, wantY)
		b.WriteString(ch)
	}
	row := b.String()
	if strings.Contains(row, "1999") {
		t.Fatalf("shrunken name-only row should not contain formatted mtime, got %q", row)
	}
}

func TestPanelNameColumnWidth(t *testing.T) {
	cols := []MetaColumnState{{EntryName: "pebble", ColumnTitle: "Pebble", Results: map[string]string{"/harbor/a": "abcdefghij"}}}
	st := panel.State{Columns: panel.ColumnsBrief}

	// 70 cells: 2 border + 1 icon gutter + 2 icon strip leaves 65; brief reserves 6 after the name.
	if got := PanelNameColumnWidth(70, st, true, true, nil); got != 59 {
		t.Fatalf("icons on, no meta = %d, want 59", got)
	}
	// Without icons the gutter and strip are gone.
	if got := PanelNameColumnWidth(70, st, false, true, nil); got != 62 {
		t.Fatalf("icons off, no meta = %d, want 62", got)
	}
	// A 10-cell meta column costs its width plus the two-cell gap.
	if got := PanelNameColumnWidth(70, st, true, true, cols); got != 47 {
		t.Fatalf("with meta = %d, want 47", got)
	}
	// A narrow panel is name-only: meta and the size column are hidden, so the name takes the row.
	if got := PanelNameColumnWidth(30, st, true, true, cols); got != 25 {
		t.Fatalf("name-only = %d, want 25", got)
	}
}
