package panelcarousel

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func TestCarouselHeaderAlignsWithRowText(t *testing.T) {
	const colWidth = 28
	showIcons := true
	listW := columnListTextWidth(colWidth, showIcons, 0)
	hdr := briefHeader(listNameHeaderTitle(showIcons), "Size", listW, true)
	row := formatBriefRow(localfs.Entry{Name: "another", Type: localfs.EntryDirectory}, colWidth, showIcons, true, panellist.RowSuffix{}, theme.Default(), nil, 0)
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
	hdr := briefHeader(" Name", "↓Size", listW, true)
	if !strings.Contains(hdr, "↓Size") {
		t.Fatalf("header %q should contain full ↓Size title", hdr)
	}
}

func TestBriefHeaderNameOnlyWhenSizeHidden(t *testing.T) {
	const listW = 24
	hdr := briefHeader("Name", "", listW, false)
	if strings.Contains(hdr, "Size") {
		t.Fatalf("header %q should not contain size column", hdr)
	}
	if len([]rune(hdr)) != listW {
		t.Fatalf("header width %d, want %d", len([]rune(hdr)), listW)
	}
}

func TestColumnListContentOriginSkipsIconStrip(t *testing.T) {
	const colX = 10
	const colWidth = 20
	x, w := columnListContentOrigin(colX, colWidth, true, 0)
	if x != colX+3 {
		t.Fatalf("list X = %d, want %d (gutter+icon strip)", x, colX+3)
	}
	if w != colWidth-3 {
		t.Fatalf("list width = %d, want %d", w, colWidth-3)
	}
}

func TestColumnListTextWidthReservesScrollbarLane(t *testing.T) {
	const colWidth = 30
	if got := columnListTextWidth(colWidth, false, 1); got != colWidth-1 {
		t.Fatalf("with scrollbar reserve got %d, want %d", got, colWidth-1)
	}
}

func TestColumnScrollbarReserveOnlyWhenScrollNeeded(t *testing.T) {
	t.Parallel()
	if got := columnScrollbarReserve(true, true, uiscrollbar.StyleThumb, 5, 10, 0); got != 0 {
		t.Fatalf("short list reserve = %d, want 0", got)
	}
	if got := columnScrollbarReserve(true, true, uiscrollbar.StyleThumb, 40, 10, 15); got != 1 {
		t.Fatalf("scrollable list reserve = %d, want 1", got)
	}
}
