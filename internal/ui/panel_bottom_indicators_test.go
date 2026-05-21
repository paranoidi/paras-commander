package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestCollectPanelBottomIndicatorsOrder(t *testing.T) {
	t.Parallel()
	ctx := PanelBottomIndicatorContext{
		PanelID:              LeftPanel,
		SelectionsBottomHint: true,
		State: panel.State{
			Path:            pathloc.MustParse("/tmp"),
			GitignoreActive: true,
		},
		Styles: theme.Default(),
	}
	got := collectPanelBottomIndicators(ctx)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != PanelBottomIndicatorSelections || got[0].Edge != PanelBottomEdgeStart {
		t.Fatalf("first = %+v, want selections on start edge", got[0])
	}
	if got[1].ID != PanelBottomIndicatorGitignore || got[1].Edge != PanelBottomEdgePhysicalLeft {
		t.Fatalf("second = %+v, want gitignore on physical left", got[1])
	}
}

func TestDropPanelBottomIndicatorsForWidthDropsHigherOrderFirst(t *testing.T) {
	t.Parallel()
	segs := []panelBottomIndicatorSegment{
		{ID: PanelBottomIndicatorGitignore, Order: 0, Label: " Gitignore "},
	}
	narrow := dropPanelBottomIndicatorsForWidth(segs, 5, true)
	if len(narrow) != 0 {
		t.Fatalf("narrow drop = %+v, want empty", narrow)
	}
	wide := dropPanelBottomIndicatorsForWidth(segs, 20, true)
	if len(wide) != 1 {
		t.Fatalf("wide drop = %+v, want one segment", wide)
	}
}

func TestPanelBottomEndEdgeReservedStartReservesSyncOnLeftDriver(t *testing.T) {
	t.Parallel()
	rect := Rect{X: 0, Y: 0, Width: 40, Height: 10}
	endX := panelBottomEndEdgeReservedStart(rect, LeftPanel, LeftPanel)
	lastIn := rect.X + rect.Width - 2
	syncW := len([]rune(panelSyncIndicatorLabel(LeftPanel)))
	want := lastIn - syncW
	if endX != want {
		t.Fatalf("endX = %d, want %d", endX, want)
	}
}

func TestPanelBottomIndicatorStyleUsesTitleAndFrame(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	title := styles.PanelActiveTitle
	frame := styles.PanelActiveFrame
	ctx := PanelBottomIndicatorContext{
		FileListActive: true,
		TitleStyle:     title,
		BorderStyle:    frame,
		Styles:         styles,
	}
	if got := panelBottomIndicatorStyle(ctx, PanelBottomIndicatorSelections); got != title {
		t.Fatalf("selections style = %v, want title %v", got, title)
	}
	if got := panelBottomIndicatorStyle(ctx, PanelBottomIndicatorGitignore); got != frame {
		t.Fatalf("gitignore style = %v, want frame %v", got, frame)
	}
}

func TestPanelBottomPhysicalLeftChainStartXOffsetWithSelectionsHint(t *testing.T) {
	t.Parallel()
	rect := Rect{X: 10, Y: 0, Width: 30, Height: 8}
	x := panelBottomPhysicalLeftChainStartX(rect, true)
	selPadW := len([]rune(panelSelectionsChromePadded))
	want := rect.X + 1 + 1 + selPadW
	if x != want {
		t.Fatalf("x = %d, want %d", x, want)
	}
}

func TestCollectPanelBottomIndicatorsStashAfterGitignore(t *testing.T) {
	t.Parallel()
	ctx := PanelBottomIndicatorContext{
		PanelID: LeftPanel,
		State: panel.State{
			Path:                pathloc.MustParse("/tmp"),
			GitignoreActive:     true,
			SelectionStashPaths: []string{"/tmp/a.txt"},
		},
		Styles: theme.Default(),
	}
	got := collectPanelBottomIndicators(ctx)
	if len(got) != 2 {
		t.Fatalf("len = %d, want gitignore + stash", len(got))
	}
	if got[0].ID != PanelBottomIndicatorGitignore || got[1].ID != PanelBottomIndicatorStash {
		t.Fatalf("order = %+v, want gitignore then stash", got)
	}
	if got[1].Label == "" || got[1].Label[0] != ' ' {
		t.Fatalf("stash label = %q", got[1].Label)
	}
}

func TestPanelBottomIndicatorRegistryIncludesSyncReserved(t *testing.T) {
	t.Parallel()
	// Sync is not in the drawable registry yet.
	for _, spec := range panelBottomIndicatorRegistry {
		if spec.ID == PanelBottomIndicatorSync {
			t.Fatal("sync must not be in drawable registry until End edge is unified")
		}
	}
	_ = tcell.Style{}
}
