package panelcarousel

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestCarouselHeaderAlignsWithRowText(t *testing.T) {
	const colWidth = 28
	showIcons := true
	listW := columnListTextWidth(colWidth, showIcons)
	hdr := briefHeader(listNameHeaderTitle(showIcons), "Size", listW)
	row := formatBriefRow(localfs.Entry{Name: "another", Type: localfs.EntryDirectory}, colWidth, showIcons, panellist.RowSuffix{}, theme.Default(), nil)
	if len([]rune(hdr)) != listW {
		t.Fatalf("header rune width %d, want list text width %d", len([]rune(hdr)), listW)
	}
	if len([]rune(row)) != listW {
		t.Fatalf("row rune width %d, want list text width %d", len([]rune(row)), listW)
	}
	// Name column starts with the same leading space as entry names.
	if !strings.HasPrefix(hdr, " Name") {
		t.Fatalf("header %q should start with leading space before Name", hdr)
	}
	if !strings.HasPrefix(row, " another") {
		t.Fatalf("row %q should start with leading space before entry name", row)
	}
}

func TestBriefHeaderKeepsDiskUsageSortArrow(t *testing.T) {
	const listW = 24
	hdr := briefHeader(" Name", "↓Size", listW)
	if !strings.Contains(hdr, "↓Size") {
		t.Fatalf("header %q should contain full ↓Size title", hdr)
	}
}

func TestColumnListContentOriginSkipsIconStrip(t *testing.T) {
	const colX = 10
	const colWidth = 20
	x, w := columnListContentOrigin(colX, colWidth, true)
	if x != colX+3 {
		t.Fatalf("list X = %d, want %d (gutter+icon strip)", x, colX+3)
	}
	if w != colWidth-3 {
		t.Fatalf("list width = %d, want %d", w, colWidth-3)
	}
}
