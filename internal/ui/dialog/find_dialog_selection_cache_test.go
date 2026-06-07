package dialog

import (
	"path/filepath"
	"strconv"
	"testing"
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
	st := &FindDialogState{
		MarkedPaths: map[string]bool{
			filepath.Clean(a): true,
			filepath.Clean(b): true,
		},
		PathIsDir: map[string]bool{
			filepath.Clean(a): false,
			filepath.Clean(b): false,
		},
		PathSize: map[string]int64{
			filepath.Clean(a): 5,
			filepath.Clean(b): 5,
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
		PathIsDir:   map[string]bool{path: false},
		PathSize:    map[string]int64{path: 100},
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
		PathIsDir:   map[string]bool{dir: true},
	}
	got, ok := st.MarkedSelectionSizeLabel(false, stubMarkedSelectionPainter{}, false, nil, "\uf017")
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if got != "1 item (0 B) \uf017" {
		t.Fatalf("label = %q, want pending glyph", got)
	}
	st.InvalidateMarkedSelectionSizeLabel()
	st.PathIsDir[dir] = true
	painter := stubMarkedSelectionPainter{sizes: map[string]int64{dir: 1024}}
	got2, ok2 := st.MarkedSelectionSizeLabel(false, painter, false, nil, "\uf017")
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
	pathIsDir := make(map[string]bool, 1000)
	pathSize := make(map[string]int64, 1000)
	for i := range 1000 {
		p := filepath.Clean(filepath.Join(root, "file_"+strconv.Itoa(i)+".txt"))
		marked[p] = true
		pathIsDir[p] = false
		pathSize[p] = 1
	}
	st := &FindDialogState{
		MarkedPaths: marked,
		PathIsDir:   pathIsDir,
		PathSize:    pathSize,
	}
	pruned := st.PrunedMarkedRoots()
	if len(pruned) != 1000 {
		t.Fatalf("pruned len = %d, want 1000", len(pruned))
	}
}
