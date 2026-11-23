package ui

import (
	"slices"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// TabBar is what a panel's tab bar needs: the active slot (1-based, 0 before the app publishes
// one) and the directory of every other used slot (zero Path = unused slot). The active tab's
// label reads the live panel path, so navigation needs no republish; App.syncTabBar is the only
// writer.
type TabBar struct {
	Active int
	Paths  [config.TabSlots]pathloc.Path
}

func (b TabBar) used(i int) bool { return i == b.Active-1 || !b.Paths[i].IsZero() }

// Count is the number of used slots.
func (b TabBar) Count() int {
	n := 0
	for i := range b.Paths {
		if b.used(i) {
			n++
		}
	}
	return n
}

// TabRows reports which browser panels have a tab-bar row: those with more than one tab, in the
// browser only. The one source for ui.Render and App's layout; it must not depend on whether the
// inactive panel is hidden (geom gives a pane with no area no row).
func (m *Model) TabRows() (primary, secondary bool) {
	if m.ViewMode != ViewBrowser {
		return false, false
	}
	return m.Tabs[PrimaryPanel].Count() > 1, m.Tabs[SecondaryPanel].Count() > 1
}

// tabSpan is one drawn tab: its slot (1-based), the first cell relative to the bar's left edge,
// its text (start cap, "N" or "N label", end cap), the rune index of the slot digit (the start
// cap is the runes before it), the rune index where the end cap begins, and the rune index of
// the cut mark in text (-1 when the label is whole or absent).
type tabSpan struct {
	Slot  int
	X, W  int
	Text  string
	Num   int
	EndAt int
	Cut   int
}

// tabCaps is [start, end] of the applied theme ([icons.panel.tab]).
type tabCaps struct{ start, end string }

// squareTabCaps are the caps of the " N " tab the layout falls back to before it drops tabs.
var squareTabCaps = tabCaps{" ", " "}

func tabCapsOf(styles theme.Theme) tabCaps {
	start, end := styles.PanelTabCaps()
	return tabCaps{start, end}
}

// tabLabel is the label of slot (1-based) of panelID's bar: the directory name, "/" at a root.
func (m *Model) tabLabel(panelID, slot int) string {
	loc := m.Tabs[panelID].Paths[slot-1]
	if slot == m.Tabs[panelID].Active {
		loc = m.Primary.Path
		if panelID == SecondaryPanel {
			loc = m.Secondary.Path
		}
	}
	if loc.IsZero() {
		return ""
	}
	return loc.Base()
}

// tabSpans is the one layout of a tab bar width cells wide, shared by the painter and the click
// hit test. A tab is the start cap, "N" or "N label" and the end cap, the caps counted in cells,
// one blank cell apart; the caps are part of the tab (and of its click area). The fit, in order:
// full labels; the longest labels shrunk to a common cap (primitive.TruncateRight, cells not
// runes) of at least config.TabLabelMinCells; slot numbers; slot numbers in square " N " tabs;
// then the tabs after the last that fits are dropped, keeping the active one.
func tabSpans(width int, bar TabBar, labels bool, caps tabCaps, labelOf func(slot int) string) []tabSpan {
	var slots []int
	for i := range bar.Paths {
		if bar.used(i) {
			slots = append(slots, i+1)
		}
	}
	if len(slots) == 0 || width <= 0 {
		return nil
	}
	full := make([]string, len(slots))
	if labels {
		for i, slot := range slots {
			full[i] = labelOf(slot)
		}
	}
	build := func(caps tabCaps, limit int) ([]tabSpan, int) {
		out := make([]tabSpan, 0, len(slots))
		x := 0
		for i, slot := range slots {
			sp := tabText(caps, slot, full[i], limit)
			sp.X = x
			out = append(out, sp)
			x += sp.W + 1
		}
		return out, x - 1
	}
	longest := 0
	for _, l := range full {
		longest = max(longest, runewidth.StringWidth(l))
	}
	if longest > 0 {
		if out, total := build(caps, longest); total <= width {
			return out
		}
		for limit := longest - 1; limit >= config.TabLabelMinCells; limit-- {
			if out, total := build(caps, limit); total <= width {
				return out
			}
		}
	}
	if out, total := build(caps, 0); total <= width {
		return out
	}
	out, total := build(squareTabCaps, 0)
	if total <= width {
		return out
	}
	return dropTabsToFit(out, width, bar.Active)
}

// tabText lays out one tab (X unset): the caps around "N" (limit 0 or no label) or "N label" with
// label cut to limit cells.
func tabText(caps tabCaps, slot int, label string, limit int) tabSpan {
	body := string(rune('0' + slot))
	cut := -1
	if label != "" && limit > 0 {
		shown := primitive.TruncateRight(label, limit)
		body += " " + shown
		if shown != label {
			cut = utf8.RuneCountInString(caps.start+body) - 1 // the ellipsis is TruncateRight's last rune
		}
	}
	text := caps.start + body + caps.end
	return tabSpan{
		Slot:  slot,
		W:     runewidth.StringWidth(text),
		Text:  text,
		Num:   utf8.RuneCountInString(caps.start),
		EndAt: utf8.RuneCountInString(caps.start + body),
		Cut:   cut,
	}
}

// dropTabsToFit keeps the leading tabs that fit width; when the active one is not among them, the
// last kept tabs give way to it. The kept tabs are re-spaced.
func dropTabsToFit(spans []tabSpan, width, active int) []tabSpan {
	total := func(set []tabSpan) int {
		n := len(set) - 1
		for _, s := range set {
			n += s.W
		}
		return n
	}
	var keep []tabSpan
	for _, s := range spans {
		if total(append(keep[:len(keep):len(keep)], s)) > width {
			break
		}
		keep = append(keep, s)
	}
	if !slices.ContainsFunc(keep, func(s tabSpan) bool { return s.Slot == active }) {
		if i := slices.IndexFunc(spans, func(s tabSpan) bool { return s.Slot == active }); i >= 0 {
			for len(keep) > 0 && total(append(keep[:len(keep):len(keep)], spans[i])) > width {
				keep = keep[:len(keep)-1]
			}
			keep = append(keep, spans[i])
		}
	}
	x := 0
	for i := range keep {
		keep[i].X = x
		x += keep[i].W + 1
	}
	return keep
}

// drawPanelTabs paints the tab bar of every panel in scope that has more than one tab, in the
// row geom reserved for it. A pane without tabs leaves its row as the screen background.
func drawPanelTabs(screen tcell.Screen, layout Layout, model Model, styles theme.Theme, chromeBlocked bool, inScope func(panelID int) bool) {
	for _, id := range [...]int{PrimaryPanel, SecondaryPanel} {
		rect := layout.PrimaryTabs
		if id == SecondaryPanel {
			rect = layout.SecondaryTabs
		}
		if rect.Width <= 0 || rect.Height <= 0 || !inScope(id) || model.Tabs[id].Count() < 2 {
			continue
		}
		active, inactive := styles.PanelTabActive, styles.PanelTabInactive
		if chromeBlocked {
			active, inactive = styles.PanelTabBlocked, styles.PanelTabBlocked
		}
		// The gaps keep the screen background (as Render fills it), so each tab stands out.
		_, gapBG, _ := styles.PanelInactiveSurface.Decompose()
		primitive.Fill(screen, primitive.Rect{X: rect.X, Y: rect.Y, Width: rect.Width, Height: 1}, ' ', tcell.StyleDefault.Background(gapBG))
		caps := tabCapsOf(styles)
		gapStyle := tcell.StyleDefault.Background(gapBG)
		for _, sp := range tabSpans(rect.Width, model.Tabs[id], model.TabLabels, caps, func(slot int) string { return model.tabLabel(id, slot) }) {
			isActive := sp.Slot == model.Tabs[id].Active
			st := inactive
			if isActive {
				st = active
			}
			// A blocked bar stays one colour: no digit style.
			var spans []primitive.Span
			if !chromeBlocked {
				spans = []primitive.Span{{Start: sp.Num, End: sp.Num + 1, Style: styles.PanelTabNumberOn(isActive, st)}}
			}
			if sp.Cut >= 0 {
				spans = append(spans, primitive.Span{Start: sp.Cut, End: sp.Cut + 1, Style: theme.CutMarkStyle(st)})
			}
			// Cap glyphs take the tab's background as foreground on the gap; spaces stay in the
			// tab's style. A tab with no background of its own cannot do that (a default
			// foreground would show a stray wedge), so its glyph cells are blanked afterwards.
			_, tabBG, _ := st.Decompose()
			runes := []rune(sp.Text)
			var blanks []int // cells of glyph runes to blank
			cell := 0
			for i, r := range runes {
				if (i < sp.Num || i >= sp.EndAt) && r != ' ' {
					if tabBG == tcell.ColorDefault {
						blanks = append(blanks, cell)
					} else {
						spans = append(spans, primitive.Span{Start: i, End: i + 1, Style: gapStyle.Foreground(tabBG)})
					}
				}
				cell += runewidth.RuneWidth(r)
			}
			w := min(sp.W, rect.Width-sp.X)
			primitive.StyledText(screen, rect.X+sp.X, rect.Y, w, sp.Text, st, spans)
			for _, c := range blanks {
				if c < w {
					screen.SetContent(rect.X+sp.X+c, rect.Y, ' ', nil, gapStyle)
				}
			}
		}
	}
}
