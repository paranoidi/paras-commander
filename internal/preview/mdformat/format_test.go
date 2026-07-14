package mdformat_test

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/preview/mdformat"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

var testBase = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)

func linesOf(cells []previewpanel.AnsiCell) []string {
	var lines []string
	var b strings.Builder
	for _, c := range cells {
		if c.R == '\n' {
			lines = append(lines, b.String())
			b.Reset()
			continue
		}
		b.WriteRune(c.R)
	}
	if b.Len() > 0 {
		lines = append(lines, b.String())
	}
	return lines
}

func cellsString(cells []previewpanel.AnsiCell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteRune(c.R)
	}
	return b.String()
}

func runeIndexOf(s string, target rune) int {
	for i, r := range []rune(s) {
		if r == target {
			return i
		}
	}
	return -1
}

func isBold(st tcell.Style) bool {
	_, _, attr := st.Decompose()
	return attr&tcell.AttrBold != 0
}

func isItalic(st tcell.Style) bool {
	_, _, attr := st.Decompose()
	return attr&tcell.AttrItalic != 0
}

func isUnderline(st tcell.Style) bool {
	_, _, attr := st.Decompose()
	return attr&tcell.AttrUnderline != 0
}

func render(t *testing.T, source string, width int) mdformat.Result {
	t.Helper()
	return mdformat.Render(source, mdformat.Options{
		Path:         "meadow.md",
		StyleName:    "monokai",
		BaseStyle:    testBase,
		ContentWidth: width,
	})
}

func TestRenderHeadingIsBoldWithHashPrefix(t *testing.T) {
	res := render(t, "# Wandering Otters\n", 60)
	lines := linesOf(res.Cells)
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# Wandering Otters") {
		t.Fatalf("lines[0] = %q, want heading with # prefix", lines[0])
	}
	foundBold := false
	for _, c := range res.Cells {
		if c.R == 'W' && isBold(c.St) {
			foundBold = true
			break
		}
	}
	if !foundBold {
		t.Fatal("expected heading text to be bold")
	}
}

func TestRenderEmphasisAndStrong(t *testing.T) {
	res := render(t, "A *velvet* and **granite** morning.\n", 60)
	var italicFound, boldFound bool
	for _, c := range res.Cells {
		switch c.R {
		case 'v':
			if isItalic(c.St) {
				italicFound = true
			}
		case 'g':
			if isBold(c.St) {
				boldFound = true
			}
		}
	}
	if !italicFound {
		t.Error("expected emphasis text to be italic")
	}
	if !boldFound {
		t.Error("expected strong text to be bold")
	}
}

func TestRenderListsAndNesting(t *testing.T) {
	source := "- lantern journey\n- copper valley\n  - hidden brook\n\n1. first errand\n2. second errand\n"
	res := render(t, source, 60)
	lines := linesOf(res.Cells)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "• lantern journey") {
		t.Errorf("missing bullet item, lines: %v", lines)
	}
	if !strings.Contains(joined, "  • hidden brook") {
		t.Errorf("expected nested bullet indented by 2 spaces, lines: %v", lines)
	}
	if !strings.Contains(joined, "1. first errand") || !strings.Contains(joined, "2. second errand") {
		t.Errorf("expected ordered list numbering, lines: %v", lines)
	}
}

func TestRenderBlockquotePrefixesEveryLine(t *testing.T) {
	res := render(t, "> stubborn kettle whistling softly through the pale winter morning air\n", 30)
	lines := linesOf(res.Cells)
	if len(lines) < 2 {
		t.Fatalf("expected quote to wrap across multiple lines, got %v", lines)
	}
	for _, l := range lines {
		if !strings.HasPrefix(l, "▌ ") {
			t.Errorf("line %q missing blockquote prefix", l)
		}
	}
}

func TestRenderFencedCodeHighlightsLanguageAndIndents(t *testing.T) {
	source := "```go\nfunc echo() int {\n\treturn 7\n}\n```\n"
	res := render(t, source, 60)
	lines := linesOf(res.Cells)
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "  func") {
		t.Fatalf("expected fenced code indented by 2 spaces, got %v", lines)
	}
	foundKeyword := false
	baseFG, _, _ := testBase.Decompose()
	for _, c := range res.Cells {
		if c.R == 'f' {
			fg, _, _ := c.St.Decompose()
			if fg != baseFG {
				foundKeyword = true
				break
			}
		}
	}
	if !foundKeyword {
		t.Error("expected fenced code body to be Chroma-highlighted for its language")
	}
}

func TestRenderLinkShowsTextAndDestination(t *testing.T) {
	res := render(t, "Visit the [orchard map](https://example.com/orchard) today.\n", 80)
	s := cellsString(res.Cells)
	if !strings.Contains(s, "orchard map") {
		t.Errorf("expected link text present, got %q", s)
	}
	if !strings.Contains(s, "(https://example.com/orchard)") {
		t.Errorf("expected link destination shown in parens, got %q", s)
	}
	foundUnderline := false
	for _, c := range res.Cells {
		if c.R == 'o' && isUnderline(c.St) {
			foundUnderline = true
			break
		}
	}
	if !foundUnderline {
		t.Error("expected link text to be underlined")
	}
}

func TestRenderThematicBreakSpansContentWidth(t *testing.T) {
	res := render(t, "above\n\n---\n\nbelow\n", 20)
	lines := linesOf(res.Cells)
	found := false
	for _, l := range lines {
		if strings.Count(l, "─") == 20 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a rule line of 20 dashes, got %v", lines)
	}
}

func TestRenderTableAlignsColumnsWithRuleAndHeaderStyle(t *testing.T) {
	source := "| Animal | Sound | Habitat |\n" +
		"|---|---|---|\n" +
		"| otter | chirp | river |\n" +
		"| falcon | screech | cliff |\n"
	res := render(t, source, 60)
	lines := linesOf(res.Cells)
	if len(lines) < 3 {
		t.Fatalf("expected header, rule, and body lines, got %v", lines)
	}
	if !strings.Contains(lines[0], "│") {
		t.Errorf("header row missing column separator, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "─") || !strings.Contains(lines[1], "┼") {
		t.Errorf("expected rule line with dashes and crosses, got %q", lines[1])
	}
	// Header cell text is bold.
	foundBold := false
	for _, c := range res.Cells {
		if c.R == 'A' && isBold(c.St) {
			foundBold = true
			break
		}
	}
	if !foundBold {
		t.Error("expected table header text to be bold")
	}
	// The "│" position should match across the header, rule, and body rows
	// (columns aligned). Compare rune positions, not byte offsets: "─"/"┼"/"│"
	// are multi-byte UTF-8 runes so strings.IndexRune's byte offset isn't
	// comparable between a dash-filled rule line and a space-filled row.
	headerBar := runeIndexOf(lines[0], '│')
	bodyBar := runeIndexOf(lines[2], '│')
	if headerBar == -1 || headerBar != bodyBar {
		t.Errorf("expected aligned column separators, header bar at %d, body bar at %d", headerBar, bodyBar)
	}
	ruleCross := runeIndexOf(lines[1], '┼')
	if ruleCross != headerBar {
		t.Errorf("expected rule cross to line up with column separators, cross at %d, bar at %d", ruleCross, headerBar)
	}
}

func TestRenderTableWrapsCellsWhenWiderThanContentWidth(t *testing.T) {
	source := "| Action | Description |\n" +
		"|---|---|\n" +
		"| gadget.press | Opens the gadget drawer and retires the legacy chord entirely |\n"
	res := render(t, source, 40)
	lines := linesOf(res.Cells)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "gadget.press") {
		t.Fatalf("expected action cell text preserved, got %v", lines)
	}
	for _, l := range lines {
		if len([]rune(l)) > 40 {
			t.Errorf("line %q exceeds content width 40", l)
		}
	}
	// The long description cell must wrap across multiple lines within the
	// same column: find the "Opens the gadget drawer" line and confirm the
	// next line continues the sentence rather than being a new row.
	foundWrap := false
	for i, l := range lines {
		if strings.Contains(l, "Opens the gadget") && i+1 < len(lines) {
			if strings.Contains(lines[i+1], "retires") || strings.Contains(lines[i+1], "legacy") {
				foundWrap = true
			}
		}
	}
	if !foundWrap {
		t.Errorf("expected description cell to word-wrap across lines, got %v", lines)
	}
}

func TestRenderWordWrapAtSmallWidth(t *testing.T) {
	res := render(t, "the quick brown fox jumps over the lazy dog again and again\n", 10)
	lines := linesOf(res.Cells)
	for _, l := range lines {
		if len([]rune(l)) > 10 {
			t.Errorf("line %q exceeds width 10", l)
		}
	}
	joined := strings.Join(lines, " ")
	if !strings.Contains(joined, "quick") || !strings.Contains(joined, "brown") {
		t.Errorf("expected words preserved intact across wrap, got %v", lines)
	}
	// No line should end mid-word with the next line continuing the same word:
	// every wrapped line breaks on a space boundary from the source.
	for _, l := range lines {
		if strings.HasSuffix(l, "-") {
			t.Errorf("unexpected hard hyphenation in line %q", l)
		}
	}
}
