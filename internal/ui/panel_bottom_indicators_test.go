package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestCollectPanelBottomIndicatorsOrder(t *testing.T) {
	t.Parallel()
	ctx := PanelBottomIndicatorContext{
		PanelID:                PrimaryPanel,
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
		PanelID:                PrimaryPanel,
		SyncDriverPanelID:      PrimaryPanel,
		QuickViewDriverPanelID: -1,
	}
	endX := panelBottomEndEdgeReservedStart(rect, ctx)
	lastIn := rect.X + rect.Width - 2
	syncW := len([]rune(panelSyncIndicatorLabel(PrimaryPanel, SplitHorizontal)))
	want := lastIn - syncW
	if endX != want {
		t.Fatalf("endX = %d, want %d", endX, want)
	}
}

func TestPanelBottomEndEdgeSegmentsOrdersOtherPanelLast(t *testing.T) {
	t.Parallel()
	ctx := PanelBottomIndicatorContext{
		PanelID:                PrimaryPanel,
		ActivePanel:            PrimaryPanel,
		HideInactivePanel:      true,
		OtherPanelPath:         "/var/log",
		UserHomeDir:            "",
		SyncDriverPanelID:      PrimaryPanel,
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

func TestPanelBottomIndicatorStyleHonorsGitignoreAndDotfilesTheme(t *testing.T) {
	t.Parallel()
	status := tcell.StyleDefault.Foreground(tcell.ColorRed)
	frame := tcell.StyleDefault.Foreground(tcell.ColorBlue)
	styles := theme.Default()
	styles.PanelStatusGitignore = status
	styles.PanelStatusDotfilesHidden = status
	styles.PanelActiveFrame = frame
	styles.PanelInactiveFrame = frame
	ctx := PanelBottomIndicatorContext{
		FileListActive: true,
		BorderStyle:    frame,
		Styles:         styles,
	}
	if got := panelBottomIndicatorStyle(ctx, PanelBottomIndicatorGitignore); got != status {
		t.Fatalf("gitignore style = %v, want status %v (not frame %v)", got, status, frame)
	}
	if got := panelBottomIndicatorStyle(ctx, PanelBottomIndicatorDotfilesHidden); got != status {
		t.Fatalf("dotfiles_hidden style = %v, want status %v (not frame %v)", got, status, frame)
	}
}

func TestPanelBottomIndicatorStyleUsesThemeAndFrame(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	frame := styles.PanelActiveFrame
	ctx := PanelBottomIndicatorContext{
		FileListActive: true,
		BorderStyle:    frame,
		Styles:         styles,
	}
	wantSel := styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelections, true, false)
	if got := panelBottomIndicatorStyle(ctx, PanelBottomIndicatorSelections); got != wantSel {
		t.Fatalf("selections style = %v, want %v", got, wantSel)
	}
	wantGit := styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeyGitignore, true, false)
	if got := panelBottomIndicatorStyle(ctx, PanelBottomIndicatorGitignore); got != wantGit {
		t.Fatalf("gitignore style = %v, want %v", got, wantGit)
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
		PanelID:                PrimaryPanel,
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
	want := " " + styles.IconHiddenDotfiles() + " "
	if physical[0].Label != want {
		t.Fatalf("label = %q, want %q", physical[0].Label, want)
	}
}

func TestCollectPanelBottomIndicatorsStashAfterGitignore(t *testing.T) {
	t.Parallel()
	ctx := PanelBottomIndicatorContext{
		PanelID:                PrimaryPanel,
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

func TestCollectPanelBottomIndicatorsJobWriteVisible(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	ctx := PanelBottomIndicatorContext{
		PanelID:                PrimaryPanel,
		QuickViewDriverPanelID: -1,
		State:                  panel.State{Path: pathloc.MustParse("/tmp")},
		Styles:                 styles,
		JobWriteMark:           true,
		JobWriteStatus:         "running",
	}
	got := collectPanelBottomIndicators(ctx)
	var seg *panelBottomIndicatorSegment
	for i := range got {
		if got[i].ID == PanelBottomIndicatorJobWrite {
			seg = &got[i]
		}
	}
	if seg == nil {
		t.Fatal("job_write segment not present when JobWriteMark is true")
	}
	if seg.Edge != PanelBottomEdgePhysicalRight {
		t.Fatalf("job_write edge = %v, want PhysicalRight", seg.Edge)
	}
	wantLabel := " " + string(styles.IconFilelistJob()) + " "
	if seg.Label != wantLabel {
		t.Fatalf("label = %q, want %q", seg.Label, wantLabel)
	}
}

func TestPanelBottomIndicatorRegistryIncludesEndEdgeIndicators(t *testing.T) {
	t.Parallel()
	var hasSync, hasQuickView, hasOther, hasJobWrite bool
	for _, spec := range panelBottomIndicatorRegistry {
		switch spec.ID {
		case PanelBottomIndicatorSync:
			hasSync = spec.Edge == PanelBottomEdgeEnd
		case PanelBottomIndicatorQuickView:
			hasQuickView = spec.Edge == PanelBottomEdgeEnd
		case PanelBottomIndicatorOtherPanel:
			hasOther = spec.Edge == PanelBottomEdgeEnd
		case PanelBottomIndicatorJobWrite:
			hasJobWrite = spec.Edge == PanelBottomEdgePhysicalRight
		}
	}
	if !hasSync || !hasQuickView || !hasOther {
		t.Fatalf("registry end edge: sync=%v quick_view=%v other_panel=%v", hasSync, hasQuickView, hasOther)
	}
	if !hasJobWrite {
		t.Fatal("job_write must be on PhysicalRight")
	}
}

func TestDrawPanelBottomIndicatorsJobWriteOnPhysicalRight(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	icon := styles.IconFilelistJob()
	label := " " + string(icon) + " "
	labelW := len([]rune(label))

	for _, panelID := range []int{PrimaryPanel, SecondaryPanel} {
		t.Run(fmt.Sprintf("panel_%d", panelID), func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatalf("Init: %v", err)
			}
			defer screen.Fini()
			const width, height = 40, 8
			screen.SetSize(width, height)
			rect := Rect{X: 0, Y: 0, Width: width, Height: height}
			ctx := PanelBottomIndicatorContext{
				PanelID:                panelID,
				SyncDriverPanelID:      -1,
				QuickViewDriverPanelID: -1,
				State:                  panel.State{Path: pathloc.MustParse("/tmp")},
				Styles:                 styles,
				BorderStyle:            styles.PanelActiveFrame,
				JobWriteMark:           true,
				JobWriteStatus:         "running",
				SplitOrientation:       SplitHorizontal,
			}
			finalizeBottomCtx(rect, &ctx)
			drawPanelBottomIndicators(screen, rect, ctx)

			bottomY := height - 1
			lastIn := width - 2
			row := tcelltest.TextAt(screen, 1, bottomY, width-2)
			idx := strings.LastIndex(row, label)
			if idx < 0 {
				t.Fatalf("panel %d bottom = %q, want job_write label %q on physical right", panelID, row, label)
			}
			// idx is relative to TextAt start at column 1; glyph should end at lastIn.
			rightmost := 1 + idx + labelW - 1
			if rightmost != lastIn {
				t.Fatalf("panel %d job_write ends at col %d, want lastIn %d (row=%q)", panelID, rightmost, lastIn, row)
			}
		})
	}
}
