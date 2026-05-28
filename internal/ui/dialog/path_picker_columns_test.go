package dialog

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestPathPickerItemSearchLine(t *testing.T) {
	item := PathPickerItem{Source: "history", Path: "/tmp/x"}
	if got := item.SearchLine(); got != "history /tmp/x" {
		t.Fatalf("SearchLine() = %q, want %q", got, "history /tmp/x")
	}
	item = PathPickerItem{Source: "fzf-marks", Name: "proj", Path: "/tmp/x"}
	if got := item.SearchLine(); got != "fzf-marks proj /tmp/x" {
		t.Fatalf("SearchLine() = %q, want %q", got, "fzf-marks proj /tmp/x")
	}
}

func TestPathPickerColumnWidthsTightColumns(t *testing.T) {
	items := []PathPickerItem{
		{Source: "history", Path: "/a"},
		{Source: "fzf-marks", Name: "ab", Path: "/b"},
		{Source: "gnome", Name: "x", Path: "/c"},
	}
	sourceW, nameW, pathW := pathPickerColumnWidths(items, 40)
	if sourceW != utf8.RuneCountInString("fzf-marks") {
		t.Fatalf("sourceW = %d, want %d", sourceW, utf8.RuneCountInString("fzf-marks"))
	}
	if nameW != 2 {
		t.Fatalf("nameW = %d, want 2", nameW)
	}
	wantPathW := 40 - sourceW - nameW - 2
	if pathW != wantPathW {
		t.Fatalf("pathW = %d, want %d", pathW, wantPathW)
	}
}

func TestPathPickerColumnWidthsShrinksWhenNarrow(t *testing.T) {
	items := []PathPickerItem{
		{Source: "fzf-marks", Name: "longname", Path: "/short"},
	}
	sourceW, nameW, pathW := pathPickerColumnWidths(items, 20)
	if pathW < pathPickerMinPathWidth {
		t.Fatalf("pathW = %d, want at least %d", pathW, pathPickerMinPathWidth)
	}
	if sourceW+nameW+pathW+pathPickerSeparatorCount(nameW) > 20 {
		t.Fatalf("columns exceed row width: %d+%d+%d", sourceW, nameW, pathW)
	}
}

func TestPathPickerColumnWidthsEmptyNameOmitsNameColumn(t *testing.T) {
	items := []PathPickerItem{{Source: "history", Path: "/tmp"}}
	sourceW, nameW, pathW := pathPickerColumnWidths(items, 30)
	if nameW != 0 {
		t.Fatalf("nameW = %d, want 0", nameW)
	}
	if pathW != 30-sourceW-1 {
		t.Fatalf("pathW = %d, want %d", pathW, 30-sourceW-1)
	}
}

func TestPathPickerRowDisplayColumns(t *testing.T) {
	item := PathPickerItem{Source: "fzf-marks", Name: "p", Path: "/tmp/x"}
	sourceW, nameW, pathW := 9, 1, 10
	got := pathPickerRowDisplay(item, sourceW, nameW, pathW)
	if !strings.HasPrefix(got, "fzf-marks") {
		t.Fatalf("row = %q, want fzf-marks prefix", got)
	}
	parts := strings.SplitN(got, " ", 3)
	if len(parts) < 3 || parts[1] != "p" {
		t.Fatalf("row = %q, want name column p", got)
	}
	if !strings.HasSuffix(parts[2], "/tmp/x") {
		t.Fatalf("row = %q, want path suffix /tmp/x", got)
	}
}

func TestPathPickerHighlightSpansAcrossColumns(t *testing.T) {
	item := PathPickerItem{Source: "history", Name: "proj", Path: "/tmp/project"}
	ranges := []search.Range{{Start: 0, End: 1}, {Start: 8, End: 12}, {Start: 13, End: 17}}
	sourceW, nameW, pathW := pathPickerColumnWidths([]PathPickerItem{item}, 50)
	style := theme.Default().FuzzyHighlight
	_, spans := pathPickerRowContent(item, ranges, sourceW, nameW, pathW, style)
	if len(spans) == 0 {
		t.Fatal("expected highlight spans")
	}
}

func TestPathPickerRowDisplayEmptyItem(t *testing.T) {
	got := pathPickerRowDisplay(PathPickerItem{}, 9, 5, 20)
	if got != "" {
		t.Fatalf("empty item row = %q, want empty", got)
	}
	text, spans := pathPickerRowContent(PathPickerItem{}, nil, 9, 5, 20, tcell.StyleDefault)
	if text != "" || len(spans) != 0 {
		t.Fatalf("empty item content = %q spans=%v, want blank", text, spans)
	}
}

func TestPathPickerRowContentHistoryEmptyName(t *testing.T) {
	item := PathPickerItem{Source: "history", Path: "/home/user/docs"}
	text, _ := pathPickerRowContent(item, nil, 7, 0, 20, tcell.StyleDefault)
	if !strings.HasPrefix(text, "history ") {
		t.Fatalf("row = %q, want history prefix", text)
	}
	if strings.Contains(text, "  history") {
		t.Fatalf("row should not contain padded history prefix: %q", text)
	}
}
