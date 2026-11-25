package ui

import (
	"maps"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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

const (
	slantStart = "\ue0ba "
	slantEnd   = " \ue0b8"
)

var (
	squareCaps = tabCaps{" ", " "}
	slantCaps  = tabCaps{slantStart, slantEnd}
)

// capTheme is the default theme with Nerd Font icons on and the given tab caps.
func capTheme(start, end string) theme.Theme {
	th := theme.Default()
	th.UseNerdfontIcons = true
	th.Icons = maps.Clone(th.Icons)
	th.Icons[theme.IconKeyPanelTabStart] = start
	th.Icons[theme.IconKeyPanelTabEnd] = end
	return th
}

func spanTexts(spans []tabSpan) []string {
	var out []string
	for _, s := range spans {
		out = append(out, s.Text)
	}
	return out
}

func spansOf(width int, bar TabBar, labels bool, caps tabCaps, paths map[int]string) []tabSpan {
	return tabSpans(width, bar, labels, caps, labelsFrom(paths))
}

func TestTabSpansFullLabelsWhenTheyFit(t *testing.T) {
	paths := map[int]string{1: "/srv/orchard", 2: "/srv/lantern", 5: "/"}
	spans := spansOf(60, tabBarOf(1, paths), true, squareCaps, paths)
	want := []string{" 1 orchard ", " 2 lantern ", " 5 / "}
	if got := spanTexts(spans); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("tabs = %q, want %q", got, want)
	}
	if spans[1].X != spans[0].W+1 {
		t.Fatalf("tabs must be one cell apart: %+v", spans)
	}
	spans = spansOf(60, tabBarOf(1, paths), true, slantCaps, paths)
	want = []string{slantStart + "1 orchard" + slantEnd, slantStart + "2 lantern" + slantEnd, slantStart + "5 /" + slantEnd}
	if got := spanTexts(spans); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("slanted tabs = %q, want %q", got, want)
	}
	first := spans[0]
	if first.Num != 2 || first.EndAt != 2+len("1 orchard") || first.Cut != -1 {
		t.Fatalf("rune indexes of the digit and the end cap: %+v", first)
	}
	if first.W != runewidth.StringWidth(first.Text) || spans[1].X != first.W+1 {
		t.Fatalf("caps must count in cells: %+v", spans)
	}
}

func TestTabSpansLongestShrinkToCommonCap(t *testing.T) {
	paths := map[int]string{1: "/srv/harbourmaster", 2: "/srv/ember", 3: "/srv/lanternkeeper"}
	for _, caps := range []tabCaps{squareCaps, slantCaps} {
		spans := spansOf(40, tabBarOf(1, paths), true, caps, paths)
		if len(spans) != 3 {
			t.Fatalf("spans = %+v", spans)
		}
		last := spans[2]
		if last.X+last.W > 40 {
			t.Fatalf("bar overflows 40 cells: %+v", spans)
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
			t.Fatalf("caps %q: cut index %d points at %q, not the ellipsis", caps.start, spans[0].Cut, string(r[spans[0].Cut]))
		}
		if !strings.HasPrefix(spans[0].Text, caps.start) || !strings.HasSuffix(spans[0].Text, caps.end) {
			t.Fatalf("a shrunk tab keeps its caps: %q", spans[0].Text)
		}
	}
}

func TestTabSpansFallbackOrderNumbersThenSquareNumbers(t *testing.T) {
	paths := map[int]string{1: "/srv/harbourmaster", 2: "/srv/lanternkeeper", 3: "/srv/emberwillow"}
	bar := tabBarOf(1, paths)
	// Square tabs: three numbers need 11 cells; a cap of config.TabLabelMinCells needs far more than 20.
	if got := strings.Join(spanTexts(spansOf(20, bar, true, squareCaps, paths)), "|"); got != " 1 | 2 | 3 " {
		t.Fatalf("tabs = %q, want numbers only (min cap %d)", got, config.TabLabelMinCells)
	}
	// Slanted numbers are 5 cells each (17 in all); labels would need more than 17.
	numbers := strings.Join([]string{slantStart + "1" + slantEnd, slantStart + "2" + slantEnd, slantStart + "3" + slantEnd}, "|")
	if got := strings.Join(spanTexts(spansOf(17, bar, true, slantCaps, paths)), "|"); got != numbers {
		t.Fatalf("tabs = %q, want slanted numbers %q", got, numbers)
	}
	// One cell less and the numbers turn square.
	if got := strings.Join(spanTexts(spansOf(16, bar, true, slantCaps, paths)), "|"); got != " 1 | 2 | 3 " {
		t.Fatalf("tabs = %q, want square numbers", got)
	}
	// Square numbers take 11 cells; below that tabs are dropped.
	if got := spansOf(10, bar, true, slantCaps, paths); len(got) != 2 || got[0].Text != " 1 " {
		t.Fatalf("tabs = %+v, want two square numbers", got)
	}
}

func TestTabSpansLabelsOffShowsNumbers(t *testing.T) {
	paths := map[int]string{1: "/srv/orchard", 2: "/srv/lantern"}
	got := strings.Join(spanTexts(spansOf(60, tabBarOf(1, paths), false, squareCaps, paths)), "|")
	if got != " 1 | 2 " {
		t.Fatalf("tabs = %q", got)
	}
}

func TestTabSpansWideRunesCutOnCellBoundaries(t *testing.T) {
	paths := map[int]string{1: "/srv/日本語のフォルダ名前", 2: "/srv/ember"}
	wide := tabCaps{"＜", "＞"} // fullwidth: two cells each
	for _, caps := range []tabCaps{squareCaps, slantCaps, wide} {
		for width := 14; width <= 50; width++ {
			spans := spansOf(width, tabBarOf(1, paths), true, caps, paths)
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
	spans := spansOf(60, tabBarOf(1, paths), true, wide, map[int]string{1: "/srv/ember", 2: "/srv/ember"})
	if spans[0].Text != "＜1 ember＞" || spans[0].W != 4+len("1 ember") {
		t.Fatalf("wide caps: %+v", spans[0])
	}
}

func TestTabSpansDropTabsKeepsActive(t *testing.T) {
	paths := map[int]string{}
	for slot := 1; slot <= 9; slot++ {
		paths[slot] = "/srv/ember"
	}
	for _, caps := range []tabCaps{squareCaps, slantCaps} {
		for _, active := range []int{1, 4, 9} {
			spans := spansOf(14, tabBarOf(active, paths), true, caps, paths)
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
	for _, styles := range []theme.Theme{theme.Default(), capTheme(slantStart, slantEnd), capTheme("\ue0b6 ", " \ue0b4")} {
		model, layout := tabsModel(80)
		if layout.PrimaryTabs.Width == 0 {
			t.Fatal("primary has tabs but no row")
		}
		spans := tabSpans(layout.PrimaryTabs.Width, model.Tabs[PrimaryPanel], model.TabLabels, tabCapsOf(styles),
			func(slot int) string { return model.tabLabel(styles, PrimaryPanel, slot) })
		if len(spans) != 2 {
			t.Fatalf("spans = %+v", spans)
		}
		for _, sp := range spans {
			// Every cell, the caps included, hits the tab.
			for dx := 0; dx < sp.W; dx++ {
				id, slot, ok := PanelTabAt(layout, &model, styles, layout.PrimaryTabs.X+sp.X+dx, layout.PrimaryTabs.Y)
				if !ok || id != PrimaryPanel || slot != sp.Slot {
					t.Fatalf("cell %d of tab %d hit (%d,%d,%v)", dx, sp.Slot, id, slot, ok)
				}
			}
		}
		gap := layout.PrimaryTabs.X + spans[0].W
		if _, _, ok := PanelTabAt(layout, &model, styles, gap, layout.PrimaryTabs.Y); ok {
			t.Fatal("the blank cell between tabs is no tab")
		}
		if _, _, ok := PanelTabAt(layout, &model, styles, layout.PrimaryTabs.X, layout.PrimaryTabs.Y+1); ok {
			t.Fatal("below the bar is no tab")
		}
		// Secondary has one tab: its row is blank and not clickable.
		if _, _, ok := PanelTabAt(layout, &model, styles, layout.SecondaryTabs.X+1, layout.SecondaryTabs.Y); ok {
			t.Fatal("a single tab shows no bar")
		}
	}
}

// cellAt returns the rune and style at column x of the primary tab bar.
func cellAt(screen tcell.SimulationScreen, layout Layout, x int) (string, tcell.Style) {
	ch, st, _ := screen.Get(layout.PrimaryTabs.X+x, layout.PrimaryTabs.Y)
	return ch, st
}

// wantCaps checks the cap cells of the tab at [x, x+w): glyph runes (the first and last cell)
// take the tab's background as foreground on the gap background with no attributes, the padding
// spaces stay in the tab's own style.
func wantCaps(t *testing.T, screen tcell.SimulationScreen, layout Layout, x, w int, tab tcell.Style, gapBG tcell.Color, start, end string) {
	t.Helper()
	_, tabBG, _ := tab.Decompose()
	glyph := func(cell int, want string) {
		ch, st := cellAt(screen, layout, cell)
		fg, bg, attrs := st.Decompose()
		if ch != want || fg != tabBG || bg != gapBG || attrs != tcell.AttrNone {
			t.Fatalf("cap cell %d = %q fg %v bg %v attrs %v, want %q fg %v on gap bg %v with no attributes", cell, ch, fg, bg, attrs, want, tabBG, gapBG)
		}
	}
	space := func(cell int) {
		ch, st := cellAt(screen, layout, cell)
		if ch != " " || st != tab {
			t.Fatalf("cap padding cell %d = %q %v, want a space in the tab style", cell, ch, st)
		}
	}
	glyph(x, strings.TrimSpace(start))
	space(x + 1)
	space(x + w - 2)
	glyph(x+w-1, strings.TrimSpace(end))
}

func TestRenderDrawsTabBarAndPartialPaintUpdatesLabel(t *testing.T) {
	const w, h = 80, 24
	screen := newTreeTestScreen(t, w, h)
	model, layout := tabsModel(w)
	styles := capTheme(slantStart, slantEnd)
	_, gapBG, _ := styles.PanelInactiveSurface.Decompose()

	Render(screen, model, styles)
	row := screenRowText(screen, layout.PrimaryTabs, layout.PrimaryTabs.Y)
	if !strings.HasPrefix(row, slantStart+"1 orchard"+slantEnd+" "+slantStart+"3 willow"+slantEnd) {
		t.Fatalf("tab row = %q", row)
	}
	// "◢ 1 orchard ◣" is 13 cells, the gap one, the second tab starts at 14.
	const activeW, inactiveX, inactiveW = 13, 14, 12
	wantCaps(t, screen, layout, 0, activeW, styles.PanelTabActive, gapBG, slantStart, slantEnd)
	wantCaps(t, screen, layout, inactiveX, inactiveW, styles.PanelTabInactive, gapBG, slantStart, slantEnd)
	if _, st := cellAt(screen, layout, activeW); func() bool { _, bg, _ := st.Decompose(); return bg != gapBG }() {
		t.Fatal("the gap between tabs must keep the gap background")
	}
	if got := screenRowText(screen, layout.SecondaryTabs, layout.SecondaryTabs.Y); strings.TrimSpace(got) != "" {
		t.Fatalf("a panel with one tab drew %q", got)
	}

	// The digit is rune 2 of the active tab: number fg and attributes on the tab's background.
	r, st := cellAt(screen, layout, 2)
	wantFG, _, wantAttrs := styles.PanelTabActiveNumber.Decompose()
	gotFG, gotBG, gotAttrs := st.Decompose()
	_, tabBG, _ := styles.PanelTabActive.Decompose()
	if r != "1" || gotFG != wantFG || gotAttrs != wantAttrs || gotBG != tabBG {
		t.Fatalf("active digit %q = %v, want number fg/attrs on the tab bg", r, st)
	}
	if _, st = cellAt(screen, layout, 4); st != styles.PanelTabActive {
		t.Fatal("the active tab's label must keep the tab style")
	}
	r, st = cellAt(screen, layout, inactiveX+2)
	wantFG, _, wantAttrs = styles.PanelTabInactiveNumber.Decompose()
	gotFG, gotBG, gotAttrs = st.Decompose()
	_, tabBG, _ = styles.PanelTabInactive.Decompose()
	if r != "3" || gotFG != wantFG || gotAttrs != wantAttrs || gotBG != tabBG {
		t.Fatalf("inactive digit %q = %v, want number fg/attrs on the tab bg", r, st)
	}
	if _, st = cellAt(screen, layout, inactiveX+4); st != styles.PanelTabInactive {
		t.Fatal("the inactive tab's label must keep the tab style")
	}

	// Navigating repaints only the panel; the label must follow the live path.
	model.Primary.Path = pathloc.MustParse("/srv/harbour")
	if !PaintBrowserListNavPanelOnly(screen, layout, model, styles, PrimaryPanel) {
		t.Fatal("partial paint declined")
	}
	row = screenRowText(screen, layout.PrimaryTabs, layout.PrimaryTabs.Y)
	if !strings.HasPrefix(row, slantStart+"1 harbour"+slantEnd+" "+slantStart+"3 willow"+slantEnd) {
		t.Fatalf("tab row after partial paint = %q", row)
	}
	wantCaps(t, screen, layout, 0, activeW, styles.PanelTabActive, gapBG, slantStart, slantEnd)

	// A blocked bar: caps in the blocked tab's background, every other cell one style.
	model.Menu.Open = true
	Render(screen, model, styles)
	wantCaps(t, screen, layout, 0, activeW, styles.PanelTabBlocked, gapBG, slantStart, slantEnd)
	wantCaps(t, screen, layout, inactiveX, inactiveW, styles.PanelTabBlocked, gapBG, slantStart, slantEnd)
	for _, x := range []int{2, inactiveX + 2} {
		if _, st = cellAt(screen, layout, x); st != styles.PanelTabBlocked {
			t.Fatalf("a blocked tab's digit at %d must stay in the blocked style", x)
		}
	}
}

func TestRenderTabNumbersLitWhileDialogTakesTabPaths(t *testing.T) {
	const w, h = 80, 24
	styles := capTheme(slantStart, slantEnd)
	pathField := []dialog.FileDialogField{{Label: "Target", PathPicker: true}}
	for name, tc := range map[string]struct {
		open func(m *Model)
		lit  bool
	}{
		"copy":          {func(m *Model) { m.TransferDialog.Open = true }, true},
		"flatten":       {func(m *Model) { m.FlattenDialog.Open = true }, true},
		"symlink":       {func(m *Model) { m.FileDialog = dialog.FileDialogState{Open: true, Fields: pathField} }, true},
		"copy advanced": {func(m *Model) { m.TransferDialog.Open, m.TransferDialog.Advanced = true, true }, false},
		"copy rename": {func(m *Model) {
			m.TransferDialog.Open, m.TransferDialog.Phase = true, dialog.TransferPhaseSelfCopyRename
		}, false},
		"path picker":  {func(m *Model) { m.TransferDialog.Open, m.PathPicker.Open = true, true }, false},
		"no path":      {func(m *Model) { m.FileDialog = dialog.FileDialogState{Open: true} }, false},
		"active panel": {func(m *Model) { m.TransferDialog.Open, m.ActivePanel = true, PrimaryPanel }, false},
	} {
		screen := newTreeTestScreen(t, w, h)
		model, layout := tabsModel(w)
		model.ActivePanel = SecondaryPanel // the tabs are on the primary, inactive panel
		tc.open(&model)
		drawPanelTabs(screen, layout, model, styles, true, func(int) bool { return true })
		for _, x := range []int{2, 14 + 2} { // the digits of both tabs
			want := styles.PanelTabBlocked
			if tc.lit {
				want = styles.PanelTabNumberOn(false, want)
			}
			if _, st := cellAt(screen, layout, x); st != want {
				t.Fatalf("%s: digit at %d = %v, want %v", name, x, st, want)
			}
		}
		if _, st := cellAt(screen, layout, 4); st != styles.PanelTabBlocked {
			t.Fatalf("%s: the label must stay blocked", name)
		}
	}
}

func TestRenderTabCutMarkFollowsCaps(t *testing.T) {
	const w, h = 80, 24
	screen := newTreeTestScreen(t, w, h)
	model, layout := tabsModel(w)
	model.Primary.Path = pathloc.MustParse("/srv/harbourmasterlongname")
	model.Tabs[PrimaryPanel] = tabBarOf(1, map[int]string{1: "/srv/x", 3: "/srv/willowlanternkeeper"})
	styles := capTheme(slantStart, slantEnd)
	Render(screen, model, styles)
	spans := tabSpans(layout.PrimaryTabs.Width, model.Tabs[PrimaryPanel], true, slantCaps,
		func(slot int) string { return model.tabLabel(styles, PrimaryPanel, slot) })
	if spans[0].Cut < 0 {
		t.Fatalf("test needs a cut label: %+v", spans)
	}
	for _, sp := range spans {
		ch, st := cellAt(screen, layout, sp.X+runewidth.StringWidth(string([]rune(sp.Text)[:sp.Cut])))
		fg, _, _ := st.Decompose()
		tab := styles.PanelTabInactive
		if sp.Slot == 1 {
			tab = styles.PanelTabActive
		}
		wantFG, _, _ := theme.CutMarkStyle(tab).Decompose()
		if ch != "…" || fg != wantFG {
			t.Fatalf("tab %d cut mark = %q fg %v, want ellipsis fg %v", sp.Slot, ch, fg, wantFG)
		}
	}

	// A blocked bar stays one colour: the ellipsis keeps the blocked style.
	model.Menu.Open = true
	Render(screen, model, styles)
	for _, sp := range spans {
		ch, st := cellAt(screen, layout, sp.X+runewidth.StringWidth(string([]rune(sp.Text)[:sp.Cut])))
		if ch != "…" || st != styles.PanelTabBlocked {
			t.Fatalf("blocked tab %d cut mark = %q %v, want an ellipsis in the blocked style", sp.Slot, ch, st)
		}
	}
}

func TestRenderTabsWithoutNerdFontIconsAreSquare(t *testing.T) {
	const w, h = 80, 24
	screen := newTreeTestScreen(t, w, h)
	model, layout := tabsModel(w)
	styles := theme.Default()
	styles.UseNerdfontIcons = false
	Render(screen, model, styles)
	if row := screenRowText(screen, layout.PrimaryTabs, layout.PrimaryTabs.Y); !strings.HasPrefix(row, " 1 orchard   3 willow ") {
		t.Fatalf("tab row = %q", row)
	}
	if _, st := cellAt(screen, layout, 0); st != styles.PanelTabActive {
		t.Fatal("a square cap is a space in the tab style")
	}
}

func TestRenderPillCapsOfCustomTheme(t *testing.T) {
	const w, h = 80, 24
	screen := newTreeTestScreen(t, w, h)
	model, layout := tabsModel(w)
	styles := capTheme("\ue0b6 ", " \ue0b4")
	_, gapBG, _ := styles.PanelInactiveSurface.Decompose()
	Render(screen, model, styles)
	if row := screenRowText(screen, layout.PrimaryTabs, layout.PrimaryTabs.Y); !strings.HasPrefix(row, "\ue0b6 1 orchard \ue0b4 \ue0b6 3 willow \ue0b4") {
		t.Fatalf("tab row = %q", row)
	}
	wantCaps(t, screen, layout, 0, 13, styles.PanelTabActive, gapBG, "\ue0b6", "\ue0b4")
	wantCaps(t, screen, layout, 14, 12, styles.PanelTabInactive, gapBG, "\ue0b6", "\ue0b4")
}

func TestRenderTabWithDefaultBackgroundBlanksCapGlyphs(t *testing.T) {
	const w, h = 80, 24
	screen := newTreeTestScreen(t, w, h)
	model, layout := tabsModel(w)
	styles := capTheme(slantStart, slantEnd)
	styles.PanelTabInactive = tcell.StyleDefault.Foreground(tcell.ColorWhite) // no background of its own
	_, gapBG, _ := styles.PanelInactiveSurface.Decompose()
	Render(screen, model, styles)
	for _, x := range []int{14, 14 + 11} { // the inactive tab's start and end glyph cells
		ch, st := cellAt(screen, layout, x)
		if _, bg, _ := st.Decompose(); ch != " " || bg != gapBG {
			t.Fatalf("cell %d = %q on %v, want a blank on the gap background %v", x, ch, bg, gapBG)
		}
	}
	// The padding spaces and the label stay in the tab's style.
	if ch, st := cellAt(screen, layout, 15); ch != " " || st != styles.PanelTabInactive {
		t.Fatalf("padding = %q %v", ch, st)
	}
	if row := screenRowText(screen, layout.PrimaryTabs, layout.PrimaryTabs.Y); strings.ContainsAny(row[len(slantStart+"1 orchard"+slantEnd):], "\ue0ba\ue0b8") {
		t.Fatalf("a cap glyph leaked onto a default-background tab: %q", row)
	}
}

func TestRenderWideCapsKeepWidthsInStep(t *testing.T) {
	const w, h = 80, 24
	screen := newTreeTestScreen(t, w, h)
	model, layout := tabsModel(w)
	styles := capTheme("＜", "＞")
	_, gapBG, _ := styles.PanelInactiveSurface.Decompose()
	Render(screen, model, styles)
	ch, st := cellAt(screen, layout, 0)
	fg, bg, _ := st.Decompose()
	_, tabBG, _ := styles.PanelTabActive.Decompose()
	if ch != "＜" || fg != tabBG || bg != gapBG {
		t.Fatalf("wide start cap = %q fg %v bg %v", ch, fg, bg)
	}
	if ch, _ = cellAt(screen, layout, 2); ch != "1" {
		t.Fatalf("digit after a two-cell cap = %q", ch)
	}
	id, slot, ok := PanelTabAt(layout, &model, styles, layout.PrimaryTabs.X+1, layout.PrimaryTabs.Y)
	if !ok || id != PrimaryPanel || slot != 1 {
		t.Fatalf("the second cell of a wide cap hit (%d,%d,%v)", id, slot, ok)
	}
}

func TestTabLabelOfTagView(t *testing.T) {
	const icon = "\U000F01BC"
	withIcon := capTheme(slantStart, slantEnd)
	withIcon.Icons[theme.IconKeyFileIndex] = icon
	plain := theme.Default()
	plain.UseNerdfontIcons = false
	tests := []struct {
		name, path string
		styles     theme.Theme
		want       string
	}{
		{"flat list", "fileindex@(beach|holiday):///", withIcon, icon + " beach|holiday"},
		{"flat inside a row", "fileindex@(beach|holiday):///srv/orchard//ember", withIcon, icon + " beach|holiday"},
		{"structure", "fileindex(beach&holiday):///srv/photos/summer", withIcon, icon + " beach&holiday /summer"},
		{"structure root", "fileindex(beach&holiday):///", withIcon, icon + " beach&holiday /"},
		{"escaped tag", "fileindex(a%26b):///srv/lantern", withIcon, icon + " a&b /lantern"},
		{"no icons flat", "fileindex@(beach|holiday):///", plain, "beach|holiday"},
		{"no icons structure", "fileindex(beach&holiday):///srv/photos/summer", plain, "beach&holiday /summer"},
		{"plain index", "fileindex:///srv/orchard", withIcon, "orchard"},
		{"local", "/srv/orchard", withIcon, "orchard"},
	}
	for _, tc := range tests {
		for _, active := range []bool{true, false} {
			model, _ := tabsModel(80)
			loc := pathloc.MustParse(tc.path)
			slot := 2
			if active {
				model.Primary.Path = loc
				slot = 1
			} else {
				model.Tabs[PrimaryPanel].Paths[1] = loc
			}
			if got := model.tabLabel(tc.styles, PrimaryPanel, slot); got != tc.want {
				t.Errorf("%s (active=%v): label = %q, want %q", tc.name, active, got, tc.want)
			}
		}
	}
}
