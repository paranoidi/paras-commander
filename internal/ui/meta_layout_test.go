package ui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestLayoutMetaCells_emptyMap(t *testing.T) {
	w, m := layoutMetaCells(map[string]string{})
	if w != panelListMetaMinCells || len(m) != 0 {
		t.Fatalf("empty: w=%d len(m)=%d", w, len(m))
	}
}

func TestLayoutMetaCells_legacySingleCell(t *testing.T) {
	w, m := layoutMetaCells(map[string]string{"p": "hello"})
	if got := m["p"]; got != "hello" {
		t.Fatalf("got %q want hello", got)
	}
	if w < panelListMetaMinCells {
		t.Fatalf("w=%d", w)
	}
}

func TestLayoutMetaCells_legacyTooLongByDisplayWidth(t *testing.T) {
	// 21 ASCII letters => display width 21 > panelListMetaMax (20)
	s := strings.Repeat("a", panelListMetaMax+1)
	_, m := layoutMetaCells(map[string]string{"p": s})
	if m["p"] != "too long" {
		t.Fatalf("got %q", m["p"])
	}
}

func TestLayoutMetaCells_tabDelimitedDigitsAlign(t *testing.T) {
	_, m := layoutMetaCells(map[string]string{
		"a": "5\t12",
		"b": "13\t3",
	})
	g1, g2 := m["a"], m["b"]
	if runewidth.StringWidth(g1) != runewidth.StringWidth(g2) {
		t.Fatalf("row width mismatch %q (%d) vs %q (%d)", g1, runewidth.StringWidth(g1), g2, runewidth.StringWidth(g2))
	}
	if !strings.HasPrefix(strings.TrimLeft(g1, " "), "5") && !strings.Contains(g1, " 5") {
		t.Fatalf("unexpected row a: %q", g1)
	}
}

func TestLayoutMetaCells_newlineDelimited(t *testing.T) {
	_, m := layoutMetaCells(map[string]string{"p": "x\ny\nz"})
	if got := m["p"]; got != "x y z" {
		t.Fatalf("got %q want x y z", got)
	}
}

func TestLayoutMetaCells_dynamicMoreFieldsLaterRow(t *testing.T) {
	_, m := layoutMetaCells(map[string]string{
		"narrow": "a\tb",
		"wide":   "a\tb\tc\td",
	})
	if want := "a b c d"; m["wide"] != want {
		t.Fatalf("wide got %q want %q", m["wide"], want)
	}
	got := m["narrow"]
	if runewidth.StringWidth(got) != runewidth.StringWidth(m["wide"]) {
		t.Fatalf("narrow width %d vs wide %d: %q vs %q", runewidth.StringWidth(got), runewidth.StringWidth(m["wide"]), got, m["wide"])
	}
	if !strings.HasPrefix(got, "a b") {
		t.Fatalf("narrow got %q", got)
	}
}

func TestLayoutMetaCells_mixedLegacyAndDelimitedUsesMaxColumns(t *testing.T) {
	_, m := layoutMetaCells(map[string]string{
		"plain": "xy",
		"tab":   "1\t2",
	})
	if m["tab"] != " 1 2" {
		t.Fatalf("tab row: %q want \" 1 2\"", m["tab"])
	}
	if m["plain"] != "xy  " {
		t.Fatalf("plain row: %q want \"xy  \"", m["plain"])
	}
}

func TestLayoutMetaColumns_twoColumns(t *testing.T) {
	cols := []MetaColumnState{
		{ColumnTitle: "Lines", Results: map[string]string{"/p": "42"}},
		{ColumnTitle: "Size", Results: map[string]string{"/p": "1K"}},
	}
	layouts, totalW := LayoutMetaColumns(cols)
	if len(layouts) != 2 {
		t.Fatalf("layouts len = %d, want 2", len(layouts))
	}
	if layouts[0].Title != "Lines" || layouts[1].Title != "Size" {
		t.Fatalf("titles = %q, %q", layouts[0].Title, layouts[1].Title)
	}
	if totalW != layouts[0].Width+2+layouts[1].Width {
		t.Fatalf("totalW = %d, want %d", totalW, layouts[0].Width+2+layouts[1].Width)
	}
	hdr := MetaHeaderText(layouts)
	if !strings.Contains(hdr, "Line") || !strings.Contains(hdr, "Size") {
		t.Fatalf("header = %q", hdr)
	}
	row := MetaRowText(layouts, "/p")
	if !strings.Contains(row, "42") || !strings.Contains(row, "1K") {
		t.Fatalf("row = %q", row)
	}
}

func TestLayoutMetaCells_rawTooLarge(t *testing.T) {
	s := strings.Repeat("x", panelMetaRawMaxBytes+1)
	_, m := layoutMetaCells(map[string]string{"p": s})
	if m["p"] != "too long" {
		t.Fatalf("got %q", m["p"])
	}
}
