package ui

import (
	"fmt"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestFormatByteSizeListedFitsPanelColumn(t *testing.T) {
	maxW := panellist.SizeCells
	tests := []int64{
		0, 1, 1023, 1024, 1536, 5000,
		1024 * 1024,
		1024*1024 + 524288,
		1024 * 1024 * 1024,
		1<<63 - 1,
	}
	for _, bytes := range tests {
		t.Run(strconv.FormatInt(bytes, 10), func(t *testing.T) {
			got := formatByteSizeListed(bytes)
			if len(got) > maxW {
				t.Fatalf("len(%q) = %d, want <= %d", got, len(got), maxW)
			}
		})
	}
}

func TestFormatByteSizeListedExamples(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{5000, "4.9K"},
		{1024, "1K"},
		{1024 * 1024, "1M"},
		{1024*1024 + 524288, "1.5M"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatByteSizeListed(tt.bytes); got != tt.want {
				t.Fatalf("formatByteSizeListed(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestPanelVolumeFreeSpaceTitle(t *testing.T) {
	tests := []struct {
		ok        bool
		avail     uint64
		total     uint64
		want      string
		wantEmpty bool
	}{
		{ok: false, wantEmpty: true},
		{ok: true, total: 0, wantEmpty: true},
		{
			ok:    true,
			avail: 28 * 1024 * 1024 * 1024,
			total: 1817 * 1024 * 1024 * 1024,
			want:  "──── 28G / 1817G (1%) ─",
		},
		{ok: true, avail: 500, total: 1000, want: "──── 500 / 1000 (50%) ─"},
	}
	for _, tt := range tests {
		got := panelVolumeFreeSpaceTitle(tt.ok, tt.avail, tt.total)
		if tt.wantEmpty {
			if got != "" {
				t.Fatalf("panelVolumeFreeSpaceTitle(...) = %q, want empty", got)
			}
			continue
		}
		if got != tt.want {
			t.Fatalf("panelVolumeFreeSpaceTitle(...) = %q, want %q", got, tt.want)
		}
	}
}

func TestPanelListHeaderWidthMatchesFormatEntryLayout(t *testing.T) {
	rowText := 50
	hdr, _ := panelListHeader(rowText, panel.State{}, panel.State{}.Columns, false, false, nil, false)
	nameW := panelListNameWidth(rowText, panel.ColumnsModified, false, false)
	wantLen := nameW + panelListReservedAfterName(panel.ColumnsModified, false)
	if utf8.RuneCountInString(hdr) != wantLen {
		t.Fatalf("header display width = %d, want %d", utf8.RuneCountInString(hdr), wantLen)
	}
}

func TestPanelListHeaderIconsLeadingSpaceMatchesNamePrefix(t *testing.T) {
	const rowText = 50
	nameW := panelListNameWidth(rowText, panel.ColumnsModified, false, false)
	hdr, _ := panelListHeader(rowText, panel.State{}, panel.State{}.Columns, true, false, nil, false)
	nameField := strings.TrimRight(hdr[:nameW], " ")
	want := "↓Name"
	if len(want) > nameW {
		want = want[:nameW]
	}
	if nameField != want {
		t.Fatalf("icons header name column = %q, want %q (before right-pad)", nameField, want)
	}
}

func TestPaintPanelTopTitleRowEndLabelCases(t *testing.T) {
	styles := theme.Default()
	chrome := styles.PanelChrome(true, false)
	home := "/home/user"
	path := "/home/user/projects/harborlantern"
	endLabel := " README.md "

	tests := []struct {
		name      string
		width     int
		endLabel  string
		wantEnd   bool
		wantTitle string
	}{
		{
			name:      "absent",
			width:     40,
			endLabel:  "",
			wantEnd:   false,
			wantTitle: "projects",
		},
		{
			name:     "narrow",
			width:    16,
			endLabel: endLabel,
			wantEnd:  false,
		},
		{
			name:     "exact-fit",
			width:    28,
			endLabel: endLabel,
			wantEnd:  true,
		},
		{
			name:     "clipped",
			width:    40,
			endLabel: endLabel,
			wantEnd:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			defer screen.Fini()
			screen.SetSize(tt.width, 4)
			titleX, innerRight, contentCols, y := panelTitleRowGeom(tt.width)
			for col := 0; col < contentCols; col++ {
				screen.SetContent(titleX+col, y, '─', nil, chrome.Frame)
			}
			row := newPanelTitleRow(Rect{Width: tt.width, Height: 4}, panel.State{}, theme.Default(), tt.endLabel)
			paintPanelTopTitleRow(screen, row, y, path, home,
				panelTitleStyles{Path: chrome.Title, End: chrome.Title, Border: chrome.Frame}, -1)
			got := tcelltest.TextAt(screen, titleX, y, contentCols)
			if tt.wantEnd {
				if !strings.Contains(got, tt.endLabel) {
					t.Fatalf("row = %q, want padded end label %q", got, tt.endLabel)
				}
				trail := tcelltest.TextAt(screen, innerRight, y, 1)
				if trail != "─" {
					t.Fatalf("trailing cell = %q, want ─ (one space of padding before the corner)", trail)
				}
			} else if tt.endLabel != "" && strings.Contains(got, strings.TrimSpace(tt.endLabel)) {
				t.Fatalf("row = %q, want end label hidden", got)
			}
			if tt.wantTitle != "" && !strings.Contains(got, tt.wantTitle) {
				t.Fatalf("row = %q, want title %q", got, tt.wantTitle)
			}
		})
	}
}

func TestPanelListHeaderColumnsCoverTitles(t *testing.T) {
	layouts := []MetaColumnLayout{{EntryName: "lantern", Title: "Lantern", Width: 9}}
	st := panel.State{Columns: panel.ColumnsModified, Sort: panel.SortState{Mode: panel.SortMeta, MetaColumn: "lantern"}}
	hdr, cols := panelListHeader(60, st, st.Columns, false, true, layouts, false)
	cells := headerCells(hdr)
	var got []string
	for _, c := range cols {
		got = append(got, fmt.Sprintf("%s=%v/%s", string(cells[c.X:c.X+c.Width]), c.Mode, c.MetaColumn))
	}
	want := []string{"Name=Name/", "↓Lantern=Meta/lantern", "Size=Size/", "Modified=Modified/"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("columns = %v, want %v (header %q)", got, want, hdr)
	}
}

func TestFormatEntryAndHeaderAlignForMixedColumns(t *testing.T) {
	stamp := time.Date(2024, 3, 9, 14, 5, 0, 0, time.UTC)
	entry := localfs.Entry{
		Name: "orchard.txt", Path: "/tmp/orchard.txt", Mode: 0o640, ModifiedAt: stamp,
		Owner: "wrenwithaverylongname", Group: "heron", Links: 12,
	}
	cols := panel.ColumnSet(0).With(panel.ColumnModified).With(panel.ColumnPermissions).With(panel.ColumnOwner).With(panel.ColumnGroup).With(panel.ColumnLinks)
	const width = 90
	row := formatEntry(entry, width, panelRowOpts{Columns: cols}, theme.Default(), nil, "")
	hdr, _ := panelListHeader(width, panel.State{}, cols, false, false, nil, false)
	if runewidth.StringWidth(row) != runewidth.StringWidth(hdr) {
		t.Fatalf("row and header widths differ\n%q\n%q", row, hdr)
	}
	for _, want := range []string{"2024-03-09 14:05", "-rw-r-----", "wrenwitha…", "heron", "12"} {
		if !strings.Contains(row, want) {
			t.Fatalf("row %q misses %q", row, want)
		}
	}
	nameW := panelListNameWidth(width, cols, false, false)
	if !strings.HasPrefix(row[nameW:], "  2024") {
		t.Fatalf("hidden size must leave only the two-cell gap: %q", row[nameW:])
	}
}

func TestPanelHeaderSortableColumnsWithHiddenSize(t *testing.T) {
	cols := panel.ColumnSet(0).With(panel.ColumnModified).With(panel.ColumnOwner)
	_, hit := panelListHeader(60, panel.State{}, cols, false, false, nil, false)
	if len(hit) != 2 || hit[0].Mode != panel.SortName || hit[1].Mode != panel.SortMtime {
		t.Fatalf("columns = %+v, want Name and Modified only", hit)
	}
}

func TestZoomGateCountsAllSelectedColumns(t *testing.T) {
	st := panel.State{Columns: panel.ColumnsModified.With(panel.ColumnPermissions).With(panel.ColumnOwner)}
	// Too narrow to show them all: the drawn listing drops some, the gate still counts them.
	if got := PanelNameColumnWidth(40, st, false, false, nil); got != 1 {
		t.Fatalf("name width = %d, want the clamped 1", got)
	}
}
