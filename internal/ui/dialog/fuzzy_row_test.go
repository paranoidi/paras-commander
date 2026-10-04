package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
)

func TestFuzzyPathRowContentUsesFitPathForWidth(t *testing.T) {
	line := "very/long/parent/directory/important_file.go"
	text, _ := fuzzyPathRowContent(line, nil, 28, tcell.StyleDefault)
	if strings.ContainsRune(text, '~') {
		t.Fatalf("display %q should not use tilde truncation", text)
	}
	if !strings.ContainsRune(text, primitive.Ellipsis) {
		t.Fatalf("display %q should use Ellipsis when shortening", text)
	}
	if !strings.Contains(text, "important_file.go") {
		t.Fatalf("display %q should preserve basename", text)
	}
	if strings.Contains(text, "directory") {
		t.Fatalf("display %q should shorten parent segments", text)
	}
}

func TestFuzzyPathRowContentMapsHighlightsOntoFittedPath(t *testing.T) {
	line := "pkg/util/target.go"
	ranges := []search.Range{{Start: 8, End: 14}} // "target" in basename
	_, spans := fuzzyPathRowContent(line, ranges, 40, tcell.StyleDefault)
	if len(spans) == 0 {
		t.Fatal("expected highlight spans on fitted path")
	}
}

func TestFuzzyPathRowContentFitsWideGlyphsByCells(t *testing.T) {
	line := "森の黒魔術/大量の画質/魔術.txt" // 18 runes, 32 cells
	text, _ := fuzzyPathRowContent(line, nil, 20, tcell.StyleDefault)
	if w := runewidth.StringWidth(text); w > 20 {
		t.Fatalf("row %q is %d cells, want <= 20", text, w)
	}
	if !strings.HasSuffix(text, "魔術.txt") {
		t.Fatalf("row %q lost the basename", text)
	}
}
