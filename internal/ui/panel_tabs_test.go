package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func tabBarOf(active int, paths map[int]string) TabBar {
	bar := TabBar{Active: active}
	for slot, p := range paths {
		if slot != active {
			bar.Paths[slot-1] = pathloc.MustParse(p)
		}
	}
	return bar
}

func labelsFrom(paths map[int]string) func(int) string {
	return func(slot int) string { return pathloc.MustParse(paths[slot]).Base() }
}

func spanTexts(spans []tabSpan) []string {
	var out []string
	for _, s := range spans {
		out = append(out, s.Text)
	}
	return out
}

func TestTabSpansFullLabelsWhenTheyFit(t *testing.T) {
	paths := map[int]string{1: "/srv/orchard", 2: "/srv/lantern", 5: "/"}
	spans := tabSpans(60, tabBarOf(1, paths), true, labelsFrom(paths))
	want := []string{" 1 orchard ", " 2 lantern ", " 5 / "}
	if got := spanTexts(spans); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("tabs = %q, want %q", got, want)
	}
	if spans[1].X != spans[0].W+1 {
		t.Fatalf("tabs must be one cell apart: %+v", spans)
	}
}

func TestTabSpansLongestShrinkToCommonCap(t *testing.T) {
	paths := map[int]string{1: "/srv/harbourmaster", 2: "/srv/ember", 3: "/srv/lanternkeeper"}
	spans := tabSpans(36, tabBarOf(1, paths), true, labelsFrom(paths))
	if len(spans) != 3 {
		t.Fatalf("spans = %+v", spans)
	}
	last := spans[2]
	if last.X+last.W > 36 {
		t.Fatalf("bar overflows 36 cells: %+v", spans)
	}
	if spans[0].Cut < 0 || spans[2].Cut < 0 || spans[1].Cut >= 0 {
		t.Fatalf("only the long labels may be cut: %+v", spans)
	}
	if runewidth.StringWidth(spans[0].Text) != runewidth.StringWidth(spans[2].Text) {
		t.Fatalf("cut labels share one cap: %q vs %q", spans[0].Text, spans[2].Text)
	}
	if !strings.Contains(spans[1].Text, "ember") {
		t.Fatalf("a short label must stay whole: %q", spans[1].Text)
	}
	r := []rune(spans[0].Text)
	if r[spans[0].Cut] != '…' {
		t.Fatalf("cut index %d points at %q, not the ellipsis", spans[0].Cut, string(r[spans[0].Cut]))
	}
}

func TestTabSpansNumbersOnlyBelowMinimumCap(t *testing.T) {
	paths := map[int]string{1: "/srv/harbourmaster", 2: "/srv/lanternkeeper", 3: "/srv/emberwillow"}
	// Three numbers need 11 cells; a cap of config.TabLabelMinCells needs far more than 20.
	spans := tabSpans(20, tabBarOf(1, paths), true, labelsFrom(paths))
	if got := strings.Join(spanTexts(spans), "|"); got != " 1 | 2 | 3 " {
		t.Fatalf("tabs = %q, want numbers only (min cap %d)", got, config.TabLabelMinCells)
	}
}

func TestTabSpansLabelsOffShowsNumbers(t *testing.T) {
	paths := map[int]string{1: "/srv/orchard", 2: "/srv/lantern"}
	got := strings.Join(spanTexts(tabSpans(60, tabBarOf(1, paths), false, labelsFrom(paths))), "|")
	if got != " 1 | 2 " {
		t.Fatalf("tabs = %q", got)
	}
}

func TestTabSpansWideRunesCutOnCellBoundaries(t *testing.T) {
	paths := map[int]string{1: "/srv/日本語のフォルダ名前", 2: "/srv/ember"}
	for width := 14; width <= 40; width++ {
		spans := tabSpans(width, tabBarOf(1, paths), true, labelsFrom(paths))
		for _, s := range spans {
			if s.X+s.W > width {
				t.Fatalf("width %d: %+v overflows", width, spans)
			}
			if runewidth.StringWidth(s.Text) != s.W {
				t.Fatalf("width %d: tab %q declares %d cells, is %d", width, s.Text, s.W, runewidth.StringWidth(s.Text))
			}
		}
	}
}

func TestTabSpansDropTabsKeepsActive(t *testing.T) {
	paths := map[int]string{}
	for slot := 1; slot <= 9; slot++ {
		paths[slot] = "/srv/ember"
	}
	for _, active := range []int{1, 4, 9} {
		spans := tabSpans(14, tabBarOf(active, paths), true, labelsFrom(paths))
		var slots []int
		kept := false
		for _, s := range spans {
			slots = append(slots, s.Slot)
			kept = kept || s.Slot == active
			if s.X+s.W > 14 {
				t.Fatalf("active %d: %+v overflows", active, spans)
			}
		}
		if !kept || len(spans) >= 9 {
			t.Fatalf("active %d: kept slots %v, want a subset holding the active tab", active, slots)
		}
	}
}

func tabsModel(width int) (Model, Layout) {
	model := Model{
		ViewMode:       ViewBrowser,
		ActivePanel:    PrimaryPanel,
		ActiveSubFocus: SubFocusFileList,
		TabLabels:      true,
		Primary: panel.State{
			Path:    pathloc.MustParse("/srv/orchard"),
			Entries: []localfs.Entry{{Name: "ember", Path: "/srv/orchard/ember", Type: localfs.EntryDirectory}},
		},
		Secondary: panel.State{Path: pathloc.MustParse("/srv/lantern")},
	}
	model.Tabs[PrimaryPanel] = tabBarOf(1, map[int]string{1: "/srv/orchard", 3: "/srv/willow"})
	model.Tabs[SecondaryPanel] = tabBarOf(1, map[int]string{1: "/srv/lantern"})
	p, s := model.TabRows()
	return model, CalculateLayoutWithOrientation(width, 24, true, PanelPaneSplit{ActivePanel: PrimaryPanel}, SplitHorizontal, 0, 0, p, s)
}

func TestTabRowsOnlyForPanelsWithMoreThanOneTab(t *testing.T) {
	model, _ := tabsModel(80)
	if p, s := model.TabRows(); !p || s {
		t.Fatalf("TabRows = %v %v, want true false", p, s)
	}
	model.ViewMode = ViewJobs
	if p, s := model.TabRows(); p || s {
		t.Fatal("tab rows outside the browser")
	}
}

func TestPanelTabAtMatchesDrawnSpans(t *testing.T) {
	model, layout := tabsModel(80)
	if layout.PrimaryTabs.Width == 0 {
		t.Fatal("primary has tabs but no row")
	}
	spans := tabSpans(layout.PrimaryTabs.Width, model.Tabs[PrimaryPanel], model.TabLabels,
		func(slot int) string { return model.tabLabel(PrimaryPanel, slot) })
	if len(spans) != 2 {
		t.Fatalf("spans = %+v", spans)
	}
	for _, sp := range spans {
		for dx := 0; dx < sp.W; dx++ {
			id, slot, ok := PanelTabAt(layout, &model, layout.PrimaryTabs.X+sp.X+dx, layout.PrimaryTabs.Y)
			if !ok || id != PrimaryPanel || slot != sp.Slot {
				t.Fatalf("cell %d of tab %d hit (%d,%d,%v)", dx, sp.Slot, id, slot, ok)
			}
		}
	}
	gap := layout.PrimaryTabs.X + spans[0].W
	if _, _, ok := PanelTabAt(layout, &model, gap, layout.PrimaryTabs.Y); ok {
		t.Fatal("the blank cell between tabs is no tab")
	}
	if _, _, ok := PanelTabAt(layout, &model, layout.PrimaryTabs.X, layout.PrimaryTabs.Y+1); ok {
		t.Fatal("below the bar is no tab")
	}
	// Secondary has one tab: its row is blank and not clickable.
	if _, _, ok := PanelTabAt(layout, &model, layout.SecondaryTabs.X+1, layout.SecondaryTabs.Y); ok {
		t.Fatal("a single tab shows no bar")
	}
}

func TestRenderDrawsTabBarAndPartialPaintUpdatesLabel(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	const w, h = 80, 24
	screen.SetSize(w, h)
	model, layout := tabsModel(w)
	styles := theme.Default()

	Render(screen, model, styles)
	row := screenRowText(screen, layout.PrimaryTabs, layout.PrimaryTabs.Y)
	if !strings.HasPrefix(row, " 1 orchard   3 willow ") {
		t.Fatalf("tab row = %q", row)
	}
	_, st, _ := screen.Get(layout.PrimaryTabs.X, layout.PrimaryTabs.Y)
	if st != styles.PanelTabActive {
		t.Fatal("the active tab must use the active style")
	}
	_, st, _ = screen.Get(layout.PrimaryTabs.X+2, layout.PrimaryTabs.Y)
	if st != styles.PanelTabActive {
		t.Fatal("the active tab's label must keep the tab style")
	}
	r, st, _ := screen.Get(layout.PrimaryTabs.X+1, layout.PrimaryTabs.Y)
	wantFG, _, wantAttrs := styles.PanelTabActiveNumber.Decompose()
	gotFG, gotBG, gotAttrs := st.Decompose()
	_, tabBG, _ := styles.PanelTabActive.Decompose()
	if r != "1" || gotFG != wantFG || gotAttrs != wantAttrs || gotBG != tabBG {
		t.Fatalf("active digit %q = %v, want number fg/attrs on the tab bg", r, st)
	}
	inactiveX := layout.PrimaryTabs.X + len(" 1 orchard ") + 1
	r, st, _ = screen.Get(inactiveX+1, layout.PrimaryTabs.Y)
	wantFG, _, wantAttrs = styles.PanelTabInactiveNumber.Decompose()
	gotFG, gotBG, gotAttrs = st.Decompose()
	_, tabBG, _ = styles.PanelTabInactive.Decompose()
	if r != "3" || gotFG != wantFG || gotAttrs != wantAttrs || gotBG != tabBG {
		t.Fatalf("inactive digit %q = %v, want number fg/attrs on the tab bg", r, st)
	}
	if _, st, _ = screen.Get(inactiveX+3, layout.PrimaryTabs.Y); st != styles.PanelTabInactive {
		t.Fatal("the inactive tab's label must keep the tab style")
	}
	_, st, _ = screen.Get(layout.PrimaryTabs.X+len(" 1 orchard "), layout.PrimaryTabs.Y)
	_, gapBG, _ := st.Decompose()
	if _, tabBG, _ := styles.PanelTabInactive.Decompose(); gapBG == tabBG {
		t.Fatal("the gap between tabs must not have the tab background")
	}
	if got := screenRowText(screen, layout.SecondaryTabs, layout.SecondaryTabs.Y); strings.TrimSpace(got) != "" {
		t.Fatalf("a panel with one tab drew %q", got)
	}

	// Navigating repaints only the panel; the label must follow the live path.
	model.Primary.Path = pathloc.MustParse("/srv/harbour")
	if !PaintBrowserListNavPanelOnly(screen, layout, model, styles, PrimaryPanel) {
		t.Fatal("partial paint declined")
	}
	row = screenRowText(screen, layout.PrimaryTabs, layout.PrimaryTabs.Y)
	if !strings.HasPrefix(row, " 1 harbour   3 willow ") {
		t.Fatalf("tab row after partial paint = %q", row)
	}

	model.Menu.Open = true
	Render(screen, model, styles)
	_, st, _ = screen.Get(layout.PrimaryTabs.X+1, layout.PrimaryTabs.Y)
	if st != styles.PanelTabBlocked {
		t.Fatal("tabs must use the blocked style while a menu has focus")
	}
	_, st, _ = screen.Get(inactiveX+1, layout.PrimaryTabs.Y)
	if st != styles.PanelTabBlocked {
		t.Fatal("a blocked tab's digit must stay in the blocked style")
	}
}
