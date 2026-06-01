package ui

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestFormatDeleteImpactSummary(t *testing.T) {
	t.Parallel()
	if got := FormatDeleteImpactSummary(1, 512, false, ""); got != "1 file (512 B)" {
		t.Fatalf("single file = %q", got)
	}
	if got := FormatDeleteImpactSummary(2, 300, false, ""); got != "2 files (300 B)" {
		t.Fatalf("plural = %q", got)
	}
	got := FormatDeleteImpactSummary(10, 1024, true, "⌛")
	if !strings.HasSuffix(got, " ⌛") {
		t.Fatalf("pending = %q, want working suffix", got)
	}
}

func TestPathsDeleteImpactSingleFile(t *testing.T) {
	t.Parallel()
	path := "/tmp/a.txt"
	byPath := map[string]localfs.Entry{
		path: {Path: path, Type: localfs.EntryFile, Size: 100},
	}
	files, bytes, pending := PathsDeleteImpact([]string{path}, byPath, false, 0, false, nil, false, nil)
	if files != 1 || bytes != 100 || pending {
		t.Fatalf("got files=%d bytes=%d pending=%v", files, bytes, pending)
	}
}

func TestPathsDeleteImpactCachedDirectory(t *testing.T) {
	t.Parallel()
	dir := "/tmp/proj"
	byPath := map[string]localfs.Entry{
		dir: {Path: dir, Type: localfs.EntryDirectory},
	}
	painter := stubSelectionSizePainter{
		sizes:      map[string]int64{dir: 5000},
		fileCounts: map[string]int64{dir: 42},
	}
	files, bytes, pending := PathsDeleteImpact([]string{dir}, byPath, false, 0, false, painter, false, nil)
	if files != 42 || bytes != 5000 || pending {
		t.Fatalf("got files=%d bytes=%d pending=%v", files, bytes, pending)
	}
}

func TestPathsDeleteImpactPendingDirectory(t *testing.T) {
	t.Parallel()
	dir := "/tmp/big"
	byPath := map[string]localfs.Entry{
		dir: {Path: dir, Type: localfs.EntryDirectory},
	}
	painter := stubSelectionSizePainter{sizes: map[string]int64{}}
	files, bytes, pending := PathsDeleteImpact([]string{dir}, byPath, false, 0, false, painter, false, nil)
	if files != 0 || bytes != 0 || !pending {
		t.Fatalf("got files=%d bytes=%d pending=%v", files, bytes, pending)
	}
}

func TestPathsDeleteImpactRemoteDirectory(t *testing.T) {
	t.Parallel()
	dir := "/remote/proj"
	byPath := map[string]localfs.Entry{
		dir: {Path: dir, Type: localfs.EntryDirectory},
	}
	files, bytes, pending := PathsDeleteImpact([]string{dir}, byPath, true, 0, false, nil, false, nil)
	if files != 0 || bytes != 0 || pending {
		t.Fatalf("remote dir: files=%d bytes=%d pending=%v", files, bytes, pending)
	}
}
