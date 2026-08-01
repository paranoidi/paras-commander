package dialog

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/paranoidi/paras-commander/internal/theme"
)

type stubMarkedSelectionPainter struct {
	sizes map[string]int64
}

func (s stubMarkedSelectionPainter) ByteSize(absPath string) (int64, bool) {
	n, ok := s.sizes[absPath]
	return n, ok
}

func (s stubMarkedSelectionPainter) DiskScanExcluded(string, bool, uint64, bool, func(string) bool) bool {
	return false
}

func TestFindMarkedSelectionSizeLabelUsesPathSizeWithoutStat(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	b := filepath.Join(root, "b.txt")
	ca := filepath.Clean(a)
	cb := filepath.Clean(b)
	st := &FindDialogState{
		MarkedPaths: map[string]bool{
			ca: true,
			cb: true,
		},
		PathMeta: func(path string) (isDir bool, size int64, ok bool) {
			switch path {
			case ca:
				return false, 5, true
			case cb:
				return false, 5, true
			default:
				return false, 0, false
			}
		},
	}
	got, ok := st.MarkedSelectionSizeLabel(false, nil, false, nil, "")
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if got != "2 items (10 B)" {
		t.Fatalf("label = %q, want %q", got, "2 items (10 B)")
	}
	// Cached on second call.
	got2, ok2 := st.MarkedSelectionSizeLabel(false, nil, false, nil, "")
	if !ok2 || got2 != got {
		t.Fatalf("cached label = %q, want %q", got2, got)
	}
}

func TestFindMarkedSelectionDerivedCacheInvalidation(t *testing.T) {
	t.Parallel()
	path := filepath.Clean(filepath.Join(t.TempDir(), "f.txt"))
	st := &FindDialogState{
		MarkedPaths: map[string]bool{path: true},
		PathMeta: func(p string) (isDir bool, size int64, ok bool) {
			if p == path {
				return false, 100, true
			}
			return false, 0, false
		},
	}
	if _, ok := st.MarkedSelectionSizeLabel(false, nil, false, nil, ""); !ok {
		t.Fatal("expected label")
	}
	gen := st.MarkedSelGen()
	st.InvalidateMarkedSelectionDerived()
	if st.MarkedSelGen() != gen+1 {
		t.Fatalf("gen = %d, want %d", st.MarkedSelGen(), gen+1)
	}
}

func TestFindMarkedSelectionSizeLabelPendingDirs(t *testing.T) {
	t.Parallel()
	dir := filepath.Clean(filepath.Join(t.TempDir(), "big"))
	st := &FindDialogState{
		MarkedPaths: map[string]bool{dir: true},
		PathMeta: func(p string) (isDir bool, size int64, ok bool) {
			if p == dir {
				return true, 0, true
			}
			return false, 0, false
		},
	}
	working := theme.Default().SymbolWorking()
	got, ok := st.MarkedSelectionSizeLabel(false, stubMarkedSelectionPainter{}, false, nil, working)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if got != "1 item (0 B) "+working {
		t.Fatalf("label = %q, want pending glyph", got)
	}
	st.InvalidateMarkedSelectionSizeLabel()
	painter := stubMarkedSelectionPainter{sizes: map[string]int64{dir: 1024}}
	got2, ok2 := st.MarkedSelectionSizeLabel(false, painter, false, nil, working)
	if !ok2 {
		t.Fatal("ok2 = false after disk refresh")
	}
	if got2 != "1 item (1 KiB)" {
		t.Fatalf("label after refresh = %q, want 1 KiB without glyph", got2)
	}
}

func TestPrunedMarkedRootsFilesOnlyFastPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	marked := make(map[string]bool, 1000)
	pathMeta := make(map[string]struct {
		isDir bool
		size  int64
	}, 1000)
	for i := range 1000 {
		p := filepath.Clean(filepath.Join(root, "file_"+strconv.Itoa(i)+".txt"))
		marked[p] = true
		pathMeta[p] = struct {
			isDir bool
			size  int64
		}{false, 1}
	}
	st := &FindDialogState{
		MarkedPaths: marked,
		PathMeta: func(path string) (isDir bool, size int64, ok bool) {
			meta, ok := pathMeta[path]
			if !ok {
				return false, 0, false
			}
			return meta.isDir, meta.size, true
		},
	}
	pruned := st.PrunedMarkedRoots()
	if len(pruned) != 1000 {
		t.Fatalf("pruned len = %d, want 1000", len(pruned))
	}
}
