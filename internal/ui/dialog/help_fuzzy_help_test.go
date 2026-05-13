package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/search"
)

func TestComputeHelpDialogListMetricsWideLayoutsSamePads(t *testing.T) {
	m120, ok120 := ComputeHelpDialogListMetrics(Layout{Width: 120, Height: 40})
	m100, ok100 := ComputeHelpDialogListMetrics(Layout{Width: 100, Height: 40})
	if !ok120 || !ok100 {
		t.Fatalf("metrics ok120=%v ok100=%v", ok120, ok100)
	}
	if m120.KeyPad != m100.KeyPad || m120.SecPad != m100.SecPad {
		t.Fatalf("pads differ: 120x → key=%d sec=%d, 100x → key=%d sec=%d",
			m120.KeyPad, m120.SecPad, m100.KeyPad, m100.SecPad)
	}
}

func TestHelpNavigationHighlightStaysInSectionColumn(t *testing.T) {
	ent := HelpEntry{
		Keys:       "F1",
		Section:    "Navigation",
		Title:      "Help",
		FuzzyExtra: "app.show-help",
	}
	layout := Layout{Width: 80, Height: 24}
	m, ok := ComputeHelpDialogListMetrics(layout)
	if !ok {
		t.Fatal("metrics")
	}
	line := FormatHelpRow(ent, 0, m.KeyPad, m.KeyPad+m.SecPad, m.InputWidth)
	keysLen := len([]rune(padRight(ent.Keys, m.KeyPad)))
	q := search.Parse("navigation")
	opts := search.Options{CaseInsensitive: true}
	res := q.Match(line, opts)
	if !res.Matched {
		t.Fatal("expected query to match painted row")
	}
	for _, r := range res.Ranges {
		if r.Start < keysLen {
			t.Fatalf("highlight range %v starts in keys column (keysLen=%d); line=%q", r, keysLen, line)
		}
	}
}

func TestHelpAltHighlightStaysInKeysColumn(t *testing.T) {
	ent := HelpEntry{
		Keys:       "Alt+O",
		Section:    "Panels",
		Title:      "Open directory in other panel",
		FuzzyExtra: "panel.open-dir-in-other",
	}
	layout := Layout{Width: 80, Height: 24}
	m, ok := ComputeHelpDialogListMetrics(layout)
	if !ok {
		t.Fatal("metrics")
	}
	line := FormatHelpRow(ent, 0, m.KeyPad, m.KeyPad+m.SecPad, m.InputWidth)
	keysLen := len([]rune(padRight(ent.Keys, m.KeyPad)))
	q := search.Parse("alt")
	opts := search.Options{CaseInsensitive: true}
	res := q.Match(line, opts)
	if !res.Matched {
		t.Fatal("expected query to match painted row")
	}
	for _, r := range res.Ranges {
		if r.End > keysLen {
			t.Fatalf("highlight range %v extends past keys column (keysLen=%d); line=%q", r, keysLen, line)
		}
	}
}

func TestHelpAltPlusOHighlightsKeys(t *testing.T) {
	ent := HelpEntry{
		Keys:       "Alt+O",
		Section:    "Panels",
		Title:      "Open directory in other panel",
		FuzzyExtra: "panel.open-dir-in-other",
	}
	layout := Layout{Width: 80, Height: 24}
	m, ok := ComputeHelpDialogListMetrics(layout)
	if !ok {
		t.Fatal("metrics")
	}
	line := FormatHelpRow(ent, 0, m.KeyPad, m.KeyPad+m.SecPad, m.InputWidth)
	keysLen := len([]rune(padRight(ent.Keys, m.KeyPad)))
	q := search.Parse("alt+o")
	opts := search.Options{CaseInsensitive: true}
	res := q.Match(line, opts)
	if !res.Matched {
		t.Fatalf("expected query to match painted row; line=%q", line)
	}
	for _, r := range res.Ranges {
		if r.End > keysLen {
			t.Fatalf("highlight range %v extends past keys column (keysLen=%d); line=%q", r, keysLen, line)
		}
	}
}
