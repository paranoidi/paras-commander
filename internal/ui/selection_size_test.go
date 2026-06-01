package ui

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestFormatSelectionByteSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1 KiB"},
		{1536, "1.5 KiB"},
		{2560, "2.5 KiB"},
		{1048576, "1 MiB"},
	}
	for _, tc := range tests {
		if got := FormatSelectionByteSize(tc.n); got != tc.want {
			t.Errorf("FormatSelectionByteSize(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
	if got := FormatSelectionByteSize(1536 + 256); strings.Count(got, ".") > 0 {
		parts := strings.SplitN(got, " ", 2)
		if len(parts) == 2 && strings.Contains(parts[0], ".") {
			frac := strings.Split(parts[0], ".")[1]
			if len(frac) > 2 {
				t.Fatalf("FormatSelectionByteSize(1792) = %q, want at most 2 fractional digits", got)
			}
		}
	}
}

type stubSelectionSizePainter struct {
	sizes map[string]int64
}

func (s stubSelectionSizePainter) ByteSize(absPath string) (int64, bool) {
	n, ok := s.sizes[absPath]
	return n, ok
}

func (stubSelectionSizePainter) PendingForPanel(string, int) bool { return false }

func (stubSelectionSizePainter) DiskScanBusy() bool { return false }

func (stubSelectionSizePainter) DiskScanExcluded(string, bool, uint64, bool, func(string) bool) bool {
	return false
}

func TestPanelSelectionSizePaddedAndCenterLayout(t *testing.T) {
	t.Parallel()
	if got := panelSelectionSizePadded("3 items (1 KiB)"); got != " 3 items (1 KiB) " {
		t.Fatalf("padded = %q", got)
	}
	rect := Rect{X: 0, Y: 0, Width: 80, Height: 12}
	padded, start, end, ok := panelSelectionSizeCenterLayout(rect, "3 items (1 KiB)")
	if !ok {
		t.Fatal("ok = false")
	}
	if padded != " 3 items (1 KiB) " {
		t.Fatalf("padded = %q", padded)
	}
	firstIn := rect.X + 1
	lastIn := rect.X + rect.Width - 2
	leftGap := start - firstIn
	rightGap := lastIn - end
	d := leftGap - rightGap
	if d < 0 {
		d = -d
	}
	if d > 1 {
		t.Fatalf("left gap = %d, right gap = %d, want centering within one cell", leftGap, rightGap)
	}
}

func TestSelectionSizeLabelFilesOnly(t *testing.T) {
	t.Parallel()
	state := panel.State{
		Path: pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{
			{Name: "a.txt", Path: "/tmp/a.txt", Type: localfs.EntryFile, Size: 100},
			{Name: "b.txt", Path: "/tmp/b.txt", Type: localfs.EntryFile, Size: 200},
		},
		SelectedPaths: map[string]bool{
			"/tmp/a.txt": true,
			"/tmp/b.txt": true,
		},
	}
	got, ok := SelectionSizeLabel(state, false, nil, false, nil, "")
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if got != "2 items (300 B)" {
		t.Fatalf("label = %q, want %q", got, "2 items (300 B)")
	}
}

func TestSelectionSizeLabelPendingWorkingGlyph(t *testing.T) {
	t.Parallel()
	dir := "/tmp/bigdir"
	state := panel.State{
		Path: pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{
			{Name: "bigdir", Path: dir, Type: localfs.EntryDirectory},
		},
		SelectedPaths: map[string]bool{dir: true},
	}
	painter := stubSelectionSizePainter{sizes: map[string]int64{}}
	got, ok := SelectionSizeLabel(state, false, painter, false, nil, "\uf017")
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if !strings.HasSuffix(got, " \uf017") {
		t.Fatalf("label = %q, want working glyph suffix", got)
	}
	if !strings.Contains(got, "1 item") {
		t.Fatalf("label = %q, want item count", got)
	}
}
