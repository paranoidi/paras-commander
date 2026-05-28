package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestCollectPanelBottomIndicatorsOrder(t *testing.T) {
	t.Parallel()
	ctx := PanelBottomIndicatorContext{
		PanelID:                LeftPanel,
		SelectionsBottomHint:   true,
		QuickViewDriverPanelID: -1,
		State: panel.State{
			Path:            pathloc.MustParse("/tmp"),
			GitignoreActive: true,
		},
		Styles: theme.Default(),
	}
	got := collectPanelBottomIndicators(ctx)
	var nonEnd []panelBottomIndicatorSegment
	for _, seg := range got {
		if seg.Edge != PanelBottomEdgeEnd {
			nonEnd = append(nonEnd, seg)
		}
	}
	if len(nonEnd) != 2 {
		t.Fatalf("len = %d, want 2 (start + physical left)", len(nonEnd))
	}
	got = nonEnd
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
	ctx := PanelBottomIndicatorContext{
		PanelID:                LeftPanel,
		SyncDriverPanelID:      LeftPanel,
		QuickViewDriverPanelID: -1,
	}
	endX := panelBottomEndEdgeReservedStart(rect, ctx)
	lastIn := rect.X + rect.Width - 2
	syncW := len([]rune(panelSyncIndicatorLabel(LeftPanel)))
	want := lastIn - syncW
	if endX != want {
		t.Fatalf("endX = %d, want %d", endX, want)
	}
}

func TestPanelBottomEndEdgeSegmentsOrdersOtherPanelLast(t *testing.T) {
	t.Parallel()
	ctx := PanelBottomIndicatorContext{
		PanelID:                LeftPanel,
		ActivePanel:            LeftPanel,
		HideInactivePanel:      true,
		OtherPanelPath:         "/var/log",
		UserHomeDir:            "",
		SyncDriverPanelID:      LeftPanel,
		QuickViewDriverPanelID: -1,
		EndEdgePathMaxRunes:    20,
		Styles:                 theme.Default(),
	}
	end := panelBottomEndEdgeSegments(ctx)
	if len(end) != 2 {
		t.Fatalf("len = %d, want sync + other_panel", len(end))
	}
	if end[0].ID != PanelBottomIndicatorSync || end[1].ID != PanelBottomIndicatorOtherPanel {
		t.Fatalf("order = %+v, want sync then other_panel", end)
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

func TestCollectPanelBottomIndicatorsDotfilesHiddenVisible(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	ctx := PanelBottomIndicatorContext{
		PanelID:                LeftPanel,
		QuickViewDriverPanelID: -1,
		State: panel.State{
			Path:                 pathloc.MustParse("/tmp"),
			DotfilesHiddenActive: true,
		},
		Styles: styles,
	}
	got := collectPanelBottomIndicators(ctx)
	var physical []panelBottomIndicatorSegment
	for _, seg := range got {
		if seg.Edge == PanelBottomEdgePhysicalLeft {
			physical = append(physical, seg)
		}
	}
	if len(physical) != 1 || physical[0].ID != PanelBottomIndicatorDotfilesHidden {
		t.Fatalf("physical = %+v, want dotfiles_hidden only", physical)
	}
	want := " " + styles.SymbolHiddenDotfiles() + " "
	if physical[0].Label != want {
		t.Fatalf("label = %q, want %q", physical[0].Label, want)
	}
}

func TestCollectPanelBottomIndicatorsStashAfterGitignore(t *testing.T) {
	t.Parallel()
	ctx := PanelBottomIndicatorContext{
		PanelID:                LeftPanel,
		QuickViewDriverPanelID: -1,
		State: panel.State{
			Path:                 pathloc.MustParse("/tmp"),
			GitignoreActive:      true,
			DotfilesHiddenActive: true,
			SelectionStashPaths:  []string{"/tmp/a.txt"},
		},
		Styles: theme.Default(),
	}
	got := collectPanelBottomIndicators(ctx)
	var physical []panelBottomIndicatorSegment
	for _, seg := range got {
		if seg.Edge == PanelBottomEdgePhysicalLeft {
			physical = append(physical, seg)
		}
	}
	if len(physical) != 3 {
		t.Fatalf("len = %d, want dotfiles_hidden + gitignore + stash", len(physical))
	}
	got = physical
	if got[0].ID != PanelBottomIndicatorDotfilesHidden || got[1].ID != PanelBottomIndicatorGitignore || got[2].ID != PanelBottomIndicatorStash {
		t.Fatalf("order = %+v, want dotfiles_hidden then gitignore then stash", got)
	}
	if got[1].Label == "" || got[1].Label[0] != ' ' {
		t.Fatalf("stash label = %q", got[1].Label)
	}
}

func TestPanelBottomIndicatorRegistryIncludesEndEdgeIndicators(t *testing.T) {
	t.Parallel()
	var hasSync, hasQuickView, hasOther bool
	for _, spec := range panelBottomIndicatorRegistry {
		switch spec.ID {
		case PanelBottomIndicatorSync:
			hasSync = spec.Edge == PanelBottomEdgeEnd
		case PanelBottomIndicatorQuickView:
			hasQuickView = spec.Edge == PanelBottomEdgeEnd
		case PanelBottomIndicatorOtherPanel:
			hasOther = spec.Edge == PanelBottomEdgeEnd
		}
	}
	if !hasSync || !hasQuickView || !hasOther {
		t.Fatalf("registry end edge: sync=%v quick_view=%v other_panel=%v", hasSync, hasQuickView, hasOther)
	}
}
