package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
)

func TestFuzzyPathRowContentUsesFitPathForWidth(t *testing.T) {
	line := "very/long/parent/directory/important_file.go"
	text, _ := fuzzyPathRowContent(line, nil, 28, tcell.StyleDefault)
	if strings.HasSuffix(text, "~") {
		t.Fatalf("display %q should not use tilde truncation", text)
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
