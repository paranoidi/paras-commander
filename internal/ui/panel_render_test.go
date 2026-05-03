package ui

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestFormatByteSizeListedFitsPanelColumn(t *testing.T) {
	maxW := panelListSizeCells
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

func TestFormatByteSizeBinaryOneDecimal(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1024*1024 + 524288, "1.5 MiB"}, // MiB + 0.5 MiB
		{-1, "0 B"},
	}
	for _, tt := range tests {
		t.Run(strconv.FormatInt(tt.bytes, 10), func(t *testing.T) {
			if got := FormatByteSize(tt.bytes); got != tt.want {
				t.Fatalf("FormatByteSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestPanelListHeaderWidthMatchesFormatEntryLayout(t *testing.T) {
	rowText := 50
	hdr := panelListHeader(rowText, panel.State{}, false)
	nameW := panelListNameWidth(rowText)
	wantLen := nameW + panelListAfterName
	if utf8.RuneCountInString(hdr) != wantLen {
		t.Fatalf("header display width = %d, want %d", utf8.RuneCountInString(hdr), wantLen)
	}
}

func TestPanelListHeaderIconsLeadingSpaceMatchesNamePrefix(t *testing.T) {
	const rowText = 50
	nameW := panelListNameWidth(rowText)
	hdr := panelListHeader(rowText, panel.State{}, true)
	nameField := strings.TrimRight(hdr[:nameW], " ")
	want := "↑ Name"
	if len(want) > nameW {
		want = want[:nameW]
	}
	if nameField != want {
		t.Fatalf("icons header name column = %q, want %q (before right-pad)", nameField, want)
	}
}
