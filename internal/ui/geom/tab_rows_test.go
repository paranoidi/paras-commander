package geom

import "testing"

func tabLayout(in LayoutInput) Layout {
	if in.Width == 0 {
		in.Width, in.Height = 100, 30
	}
	in.ShowMenuBar = true
	return CalculateLayoutWithOrientation(in)
}

func TestTabRowsSideBySideLowersBothPanes(t *testing.T) {
	plain := tabLayout(LayoutInput{})
	l := tabLayout(LayoutInput{PrimaryTabs: true})
	for name, c := range map[string]struct{ tabs, pane, base Rect }{
		"primary":   {l.PrimaryTabs, l.Primary, plain.Primary},
		"secondary": {l.SecondaryTabs, l.Secondary, plain.Secondary},
	} {
		if c.tabs != (Rect{X: c.base.X, Y: c.base.Y, Width: c.base.Width, Height: 1}) {
			t.Errorf("%s tabs = %+v, want one row at the old top of %+v", name, c.tabs, c.base)
		}
		if c.pane != (Rect{X: c.base.X, Y: c.base.Y + 1, Width: c.base.Width, Height: c.base.Height - 1}) {
			t.Errorf("%s pane = %+v, want %+v minus its top row", name, c.pane, c.base)
		}
	}
	if l.Footer != plain.Footer || l.Menu != plain.Menu {
		t.Error("tab rows moved the menu or footer")
	}
}

func TestTabRowsOffByDefault(t *testing.T) {
	l := tabLayout(LayoutInput{})
	if l.PrimaryTabs != (Rect{}) || l.SecondaryTabs != (Rect{}) {
		t.Fatalf("tab rects without a request: %+v %+v", l.PrimaryTabs, l.SecondaryTabs)
	}
}

func TestTabRowsSwappedPanes(t *testing.T) {
	l := tabLayout(LayoutInput{SecondaryTabs: true, Split: PanelPaneSplit{SwapPanes: true}})
	if l.SecondaryTabs.X != 0 || l.PrimaryTabs.X == 0 {
		t.Fatalf("swapped: secondary tabs %+v primary tabs %+v, want secondary on the left", l.SecondaryTabs, l.PrimaryTabs)
	}
	if l.Primary.Y != l.Secondary.Y {
		t.Fatalf("pane tops differ: %d vs %d", l.Primary.Y, l.Secondary.Y)
	}
}

func TestTabRowsHiddenPaneNeverRequests(t *testing.T) {
	// Only the hidden (zero width) pane has tabs: no row at all.
	hidden := PanelPaneSplit{ActivePanel: 0, HideInactivePanel: true}
	l := tabLayout(LayoutInput{SecondaryTabs: true, Split: hidden})
	plain := tabLayout(LayoutInput{Split: hidden})
	if l.Primary != plain.Primary || l.PrimaryTabs != (Rect{}) || l.SecondaryTabs != (Rect{}) {
		t.Fatalf("a hidden pane's tabs took a row: %+v / %+v", l.Primary, l.PrimaryTabs)
	}
	// The visible pane's own request still works and spans the full width.
	l = tabLayout(LayoutInput{PrimaryTabs: true, SecondaryTabs: true, Split: hidden})
	if l.PrimaryTabs.Width != 100 || l.SecondaryTabs != (Rect{}) || l.Primary.Height != plain.Primary.Height-1 {
		t.Fatalf("visible pane tabs = %+v pane %+v", l.PrimaryTabs, l.Primary)
	}
}

func TestTabRowsStackedOnlyFlaggedPane(t *testing.T) {
	plain := tabLayout(LayoutInput{Orientation: SplitVertical, Height: 40})
	l := tabLayout(LayoutInput{Orientation: SplitVertical, Height: 40, SecondaryTabs: true})
	if l.PrimaryTabs != (Rect{}) || l.Primary != plain.Primary {
		t.Fatalf("unflagged primary changed: tabs %+v pane %+v", l.PrimaryTabs, l.Primary)
	}
	want := Rect{X: 0, Y: plain.Secondary.Y, Width: 100, Height: 1}
	if l.SecondaryTabs != want {
		t.Fatalf("secondary tabs = %+v, want %+v", l.SecondaryTabs, want)
	}
	if l.Secondary.Y != plain.Secondary.Y+1 || l.Secondary.Height != plain.Secondary.Height-1 {
		t.Fatalf("secondary pane = %+v, want top row taken from %+v", l.Secondary, plain.Secondary)
	}
}

func TestTabRowsWithTerminalStripAndMinimumHeight(t *testing.T) {
	l := tabLayout(LayoutInput{Width: 40, Height: 8, PrimaryTabs: true})
	if l.TooSmall || PanelListRows(l.Primary) < 1 {
		t.Fatalf("minimum terminal lost its list rows: %+v", l.Primary)
	}
	l = tabLayout(LayoutInput{Height: 30, TerminalRows: 6, PrimaryTabs: true})
	if l.Terminal.Height != 6 || l.Primary.Y+l.Primary.Height != l.Terminal.Y {
		t.Fatalf("panes %+v must end at the terminal strip %+v", l.Primary, l.Terminal)
	}
	if l.PrimaryTabs.Y != 1 {
		t.Fatalf("tab row at %d, want directly under the menu", l.PrimaryTabs.Y)
	}
	l = tabLayout(LayoutInput{Width: 40, Height: 16, Orientation: SplitVertical, PrimaryTabs: true, SecondaryTabs: true})
	if PanelListRows(l.Primary) < 1 || PanelListRows(l.Secondary) < 1 {
		t.Fatalf("stacked minimum lost list rows: %+v %+v", l.Primary, l.Secondary)
	}
}
