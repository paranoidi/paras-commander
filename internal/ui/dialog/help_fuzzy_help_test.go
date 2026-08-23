package dialog

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/search"
)

func TestHelpAltHighlightStaysInKeysColumn(t *testing.T) {
	// Title deliberately has no a/l/t letters, so a fuzzy "alt" query can only match inside
	// the Keys segment — with the letters shared, this greedy fuzzy matcher can otherwise
	// stitch a match across title+keys (see TestHelpAltPlusOHighlightsKeys for that query).
	ent := HelpEntry{
		Keys:       "Alt+O",
		Section:    "Panels",
		Title:      "Copy",
		FuzzyExtra: "panel.open-dir-in-other",
	}
	layout := Layout{Width: 80, Height: 24}
	m, ok := ComputeHelpDialogListMetrics(layout)
	if !ok {
		t.Fatal("metrics")
	}
	line, keyStart, keyEnd := FormatHelpRow(ent, m.KeyColWidth, m.InputWidth)
	q := search.Parse("alt")
	opts := search.Options{CaseInsensitive: true}
	res := q.Match(line, opts)
	if !res.Matched {
		t.Fatal("expected query to match painted row")
	}
	for _, r := range res.Ranges {
		if r.Start < keyStart || r.End > keyEnd {
			t.Fatalf("highlight range %v outside keys column [%d,%d); line=%q", r, keyStart, keyEnd, line)
		}
	}
}

func TestBuildHelpVisualRowsAlwaysGroupsBySection(t *testing.T) {
	entries := []HelpEntry{
		{Title: "Up", Section: "Navigation"},
		{Title: "Down", Section: "Navigation"},
		{Title: "Copy", Section: "File operations"},
	}
	ranked := []int{0, 1, 2} // identity order, as if Query == "" (browse mode)

	rows := BuildHelpVisualRows(entries, ranked)
	// No blank spacer, ever: "Navigation", Up, Down, "File operations", Copy = 5 rows,
	// headers at 0 and 3.
	wantHeaders := []int{0, 3}
	gotHeaders := []int{}
	for i, r := range rows {
		if r.IsHeader {
			gotHeaders = append(gotHeaders, i)
		}
	}
	if len(rows) != 5 {
		t.Fatalf("rows = %d, want 5: %+v", len(rows), rows)
	}
	if len(gotHeaders) != 2 || gotHeaders[0] != wantHeaders[0] || gotHeaders[1] != wantHeaders[1] {
		t.Fatalf("header positions = %v, want %v", gotHeaders, wantHeaders)
	}
	if rows[0].Header != "Navigation" || rows[3].Header != "File operations" {
		t.Fatalf("header text wrong: rows[0]=%q rows[3]=%q", rows[0].Header, rows[3].Header)
	}

	// Filtered order interleaving sections: headers still appear at every boundary, including
	// immediately before the first row, and can repeat when a section reappears later.
	filtered := []int{2, 0, 1} // Copy (File operations), Up, Down (Navigation)
	rows = BuildHelpVisualRows(entries, filtered)
	// "File operations", Copy, "Navigation", Up, Down = 5 rows, headers at 0 and 2.
	if len(rows) != 5 {
		t.Fatalf("filtered rows = %d, want 5: %+v", len(rows), rows)
	}
	if !rows[0].IsHeader || rows[0].Header != "File operations" {
		t.Fatalf("rows[0] = %+v, want File operations header", rows[0])
	}
	if !rows[2].IsHeader || rows[2].Header != "Navigation" {
		t.Fatalf("rows[2] = %+v, want Navigation header", rows[2])
	}
}

func TestHelpAltPlusOHighlightsKeys(t *testing.T) {
	ent := HelpEntry{
		Keys:       "Alt+O",
		Section:    "Panels",
		Title:      "Copy",
		FuzzyExtra: "panel.open-dir-in-other",
	}
	layout := Layout{Width: 80, Height: 24}
	m, ok := ComputeHelpDialogListMetrics(layout)
	if !ok {
		t.Fatal("metrics")
	}
	line, keyStart, keyEnd := FormatHelpRow(ent, m.KeyColWidth, m.InputWidth)
	q := search.Parse("alt+o")
	opts := search.Options{CaseInsensitive: true}
	res := q.Match(line, opts)
	if !res.Matched {
		t.Fatalf("expected query to match painted row; line=%q", line)
	}
	for _, r := range res.Ranges {
		if r.Start < keyStart || r.End > keyEnd {
			t.Fatalf("highlight range %v outside keys column [%d,%d); line=%q", r, keyStart, keyEnd, line)
		}
	}
}

func TestFormatHelpRowRightAlignsKeysInLeftColumn(t *testing.T) {
	ent := HelpEntry{Keys: "F5", Title: "Copy"}
	line, keyStart, keyEnd := FormatHelpRow(ent, 10, 30)
	want := strings.Repeat(" ", 8) + "F5" + " " + "Copy" // keys right-aligned in a 10-col field, then 1-space gap, then title
	if line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}
	if keyStart != 8 {
		t.Fatalf("keyStart = %d, want 8", keyStart)
	}
	if keyEnd != 10 {
		t.Fatalf("keyEnd = %d, want 10", keyEnd)
	}
}

func TestFormatHelpRowGrowsColumnForOverlongKeys(t *testing.T) {
	ent := HelpEntry{Keys: "Ctrl-Alt-Shift-F12", Title: "Copy"}
	line, keyStart, keyEnd := FormatHelpRow(ent, 10, 40)
	want := "Ctrl-Alt-Shift-F12 Copy" // keys wider than the 10-col field still get their 1-space gap
	if line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}
	if keyStart != 0 {
		t.Fatalf("keyStart = %d, want 0", keyStart)
	}
	if keyEnd != len(ent.Keys) {
		t.Fatalf("keyEnd = %d, want %d", keyEnd, len(ent.Keys))
	}
}
