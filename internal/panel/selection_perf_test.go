package panel

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func makeFlatFileListing(b testing.TB, n int) (*State, []localfs.Entry) {
	b.Helper()
	root := b.TempDir()
	entries := make([]localfs.Entry, n)
	for i := 0; i < n; i++ {
		p := filepath.Join(root, fmt.Sprintf("file_%05d.txt", i))
		entries[i] = localfs.Entry{
			Name: filepath.Base(p),
			Path: p,
			Type: localfs.EntryFile,
			Size: 64,
		}
	}
	s := &State{
		Path:    pathloc.MustParse(root),
		Entries: entries,
	}
	s.rebuildListingByPath()
	return s, entries
}

func BenchmarkToggleSelectionGrowingFiles(b *testing.B) {
	const n = 5000
	s, entries := makeFlatFileListing(b, n)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := i % n
		s.Cursor = idx
		s.ToggleSelection()
	}
	_ = entries
}

func BenchmarkToggleSelectionAt5000Selected(b *testing.B) {
	s, entries := makeFlatFileListing(b, 5000)
	for i := range entries {
		s.Cursor = i
		s.ToggleSelection()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Cursor = 0
		s.ToggleSelection()
		s.ToggleSelection()
	}
}

func BenchmarkInvertSelection9000(b *testing.B) {
	s, _ := makeFlatFileListing(b, 9000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.InvertSelection()
	}
}

func BenchmarkPruneNestedPathsDirs1000(b *testing.B) {
	root := b.TempDir()
	paths := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		paths[i] = filepath.Join(root, fmt.Sprintf("dir_%03d", i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = PruneNestedPaths(paths)
	}
}

// TestSelectGroupSelectAllLargeDirectoryListingIsFast exercises the real select-all entry point
// (the "+" keybinding's SelectGroup("*", ...) call, per internal/app/dialog_settings.go) on a
// listing of many sibling directories, the scenario that previously froze the UI via O(n^2)
// conflict-clearing in BulkApplySelectionAdds's slow path.
func TestSelectGroupSelectAllLargeDirectoryListingIsFast(t *testing.T) {
	root := t.TempDir()
	const n = 5000
	entries := make([]localfs.Entry, n)
	for i := range n {
		p := filepath.Join(root, fmt.Sprintf("dir_%05d", i))
		entries[i] = localfs.Entry{Name: filepath.Base(p), Path: p, Type: localfs.EntryDirectory}
	}
	s := &State{Path: pathloc.MustParse(root), Entries: entries}
	s.rebuildListingByPath()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := s.SelectGroup("*", false, false, false, GroupPatternShell, GroupSelectMeta{}); err != nil {
			t.Error(err)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("select-all took too long on a large directory listing (likely quadratic)")
	}
	if got := s.SelectedPathCount(); got != n {
		t.Fatalf("got %d selected, want %d", got, n)
	}
}

func BenchmarkPrunedSelectionRootsRebuildPerToggle(b *testing.B) {
	s, entries := makeFlatFileListing(b, 1500)
	for i := range entries {
		s.Cursor = i
		s.ToggleSelection()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Cursor = i % len(entries)
		s.ToggleSelection()
		_ = s.PrunedSelectionRoots()
	}
}
