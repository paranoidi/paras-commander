package chromaformat_test

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/preview/chromaformat"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func TestHighlightGoKeywordColored(t *testing.T) {
	base := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
	res := chromaformat.Highlight("package main\n", chromaformat.Options{
		Path:        "handler.go",
		StyleName:   "monokai",
		BaseStyle:   base,
		LineNumbers: false,
		TabWidth:    4,
	})
	if len(res.Cells) == 0 {
		t.Fatal("expected highlighted cells")
	}
	baseFG, _, _ := base.Decompose()
	foundKeyword := false
	for _, c := range res.Cells {
		if c.R == 'p' {
			fg, _, _ := c.St.Decompose()
			if fg != baseFG {
				foundKeyword = true
				break
			}
		}
	}
	if !foundKeyword {
		t.Fatal("expected colored styling on Go keyword token")
	}
}

func TestHighlightExpandsTabs(t *testing.T) {
	base := tcell.StyleDefault
	res := chromaformat.Highlight("a\tb", chromaformat.Options{
		Path:      "plain.txt",
		StyleName: "monokai",
		BaseStyle: base,
		TabWidth:  4,
	})
	spaces := 0
	for _, c := range res.Cells {
		if c.R == ' ' {
			spaces++
		}
	}
	if spaces < 3 {
		t.Fatalf("tab expansion spaces = %d, want >= 3", spaces)
	}
}

func TestHighlightLineNumbersGutterWidth(t *testing.T) {
	base := tcell.StyleDefault
	source := "one\ntwo\nthree\n"
	res := chromaformat.Highlight(source, chromaformat.Options{
		Path:        "ledger.txt",
		StyleName:   "monokai",
		BaseStyle:   base,
		LineNumbers: true,
		TabWidth:    4,
	})
	if res.GutterWidth < 2 {
		t.Fatalf("GutterWidth = %d, want >= 2", res.GutterWidth)
	}
	prefix := cellsString(res.Cells)
	if len(prefix) < 3 {
		t.Fatalf("prefix too short: %q", prefix)
	}
	if !strings.HasPrefix(prefix, "1 ") {
		t.Fatalf("prefix %q, want line number gutter starting with %q", prefix, "1 ")
	}
}

func TestHighlightLineNumbersMultiDigit(t *testing.T) {
	base := tcell.StyleDefault
	var lines strings.Builder
	for i := 1; i <= 23; i++ {
		if i > 1 {
			lines.WriteByte('\n')
		}
		lines.WriteString("x")
	}
	res := chromaformat.Highlight(lines.String(), chromaformat.Options{
		Path:        "ledger.txt",
		StyleName:   "monokai",
		BaseStyle:   base,
		LineNumbers: true,
		TabWidth:    4,
	})
	// Find start of line 23 (after 22 newlines).
	line23 := cellsStringFromLine(res.Cells, 23)
	if !strings.HasPrefix(line23, "23 ") {
		t.Fatalf("line 23 gutter %q, want prefix %q", line23, "23 ")
	}
}

func cellsStringFromLine(cells []previewpanel.AnsiCell, line int) string {
	cur := 1
	var b strings.Builder
	for _, c := range cells {
		if c.R == '\n' {
			if cur == line {
				break
			}
			cur++
			b.Reset()
			continue
		}
		if cur == line {
			b.WriteRune(c.R)
		}
	}
	return b.String()
}

func TestHighlightUnknownExtensionUsesFallbackLexer(t *testing.T) {
	base := tcell.StyleDefault
	res := chromaformat.Highlight("echo hello", chromaformat.Options{
		Path:      "randomword.xyz",
		StyleName: "monokai",
		BaseStyle: base,
	})
	if len(res.Cells) == 0 {
		t.Fatal("expected cells from fallback lexer")
	}
}

func cellsString(cells []previewpanel.AnsiCell) string {
	var b strings.Builder
	for _, c := range cells {
		if c.R == '\n' {
			break
		}
		b.WriteRune(c.R)
	}
	return b.String()
}
