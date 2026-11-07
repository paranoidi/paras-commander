package chromaformat_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/preview/chromaformat"
	"github.com/paranoidi/paras-commander/internal/preview/chromastyles"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func TestHighlightCustomLoadedStyle(t *testing.T) {
	t.Cleanup(chromastyles.ResetForTest)
	dir := t.TempDir()
	xml := `<style name="cedar-glow-preview"><entry type="Keyword" style="#112233"/></style>`
	if err := os.WriteFile(filepath.Join(dir, "cedar-glow-preview.xml"), []byte(xml), 0o644); err != nil {
		t.Fatalf("write style: %v", err)
	}
	if err := chromastyles.LoadFromDir(dir); err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}

	base := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
	res := chromaformat.Highlight(t.Context(), "package main\n", chromaformat.Options{
		Path:        "handler.go",
		StyleName:   "cedar-glow-preview",
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
		t.Fatal("expected custom preview style to color Go keyword token")
	}
}

func TestHighlightGoKeywordColored(t *testing.T) {
	base := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
	res := chromaformat.Highlight(t.Context(), "package main\n", chromaformat.Options{
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
	res := chromaformat.Highlight(t.Context(), "a\tb", chromaformat.Options{
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
	res := chromaformat.Highlight(t.Context(), source, chromaformat.Options{
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
	res := chromaformat.Highlight(t.Context(), lines.String(), chromaformat.Options{
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
	res := chromaformat.Highlight(t.Context(), "echo hello", chromaformat.Options{
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

func cellText(cells []previewpanel.AnsiCell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteRune(c.R)
	}
	return b.String()
}

func TestHighlightPartialIsWholeBodyWithSameRunes(t *testing.T) {
	var src strings.Builder
	src.WriteString("package harbor\r\n\r\n")
	for i := range 400 {
		fmt.Fprintf(&src, "\tvar lantern%d = \"meadow\" // copper\r\n", i)
	}
	var partial []chromaformat.Result
	res := chromaformat.Highlight(t.Context(), src.String(), chromaformat.Options{
		Path:              "harbor.go",
		StyleName:         "monokai",
		BaseStyle:         tcell.StyleDefault,
		LineNumbers:       true,
		Partial:           func(r chromaformat.Result) { partial = append(partial, r) },
		PartialAfterCells: 256,
	})
	if len(partial) != 1 {
		t.Fatalf("partial calls = %d, want 1", len(partial))
	}
	p := partial[0]
	if p.GutterWidth != res.GutterWidth {
		t.Fatalf("gutter = %d, want %d", p.GutterWidth, res.GutterWidth)
	}
	if got, want := cellText(p.Cells), cellText(res.Cells); got != want {
		t.Fatal("partial body must hold the same runes as the final body")
	}
	for i := range 256 {
		if p.Cells[i].St != res.Cells[i].St {
			t.Fatalf("cell %d of the highlighted prefix differs from the final body", i)
		}
	}
	tailStyled := false
	for i := len(p.Cells) / 2; i < len(p.Cells); i++ {
		tailStyled = tailStyled || p.Cells[i].St != res.Cells[i].St
	}
	if !tailStyled {
		t.Fatal("tail should still be unstyled in the partial body")
	}
}

func TestHighlightShortBodyHasNoPartial(t *testing.T) {
	called := false
	chromaformat.Highlight(t.Context(), "package harbor\n", chromaformat.Options{
		Path:              "harbor.go",
		StyleName:         "monokai",
		Partial:           func(chromaformat.Result) { called = true },
		PartialAfterCells: 256,
	})
	if called {
		t.Fatal("a body under the threshold is published once, as its final result")
	}
}

func TestHighlightStopsWhenCanceled(t *testing.T) {
	src := strings.Repeat("var lantern = \"meadow\" // copper\n", 20000)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	res := chromaformat.Highlight(ctx, src, chromaformat.Options{Path: "harbor.go", StyleName: "monokai"})
	if len(res.Cells) >= len(src) {
		t.Fatalf("canceled run produced %d cells of %d", len(res.Cells), len(src))
	}
}
