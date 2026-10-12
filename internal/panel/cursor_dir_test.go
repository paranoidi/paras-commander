package panel

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
	"os"
	"path/filepath"
	"testing"
)

func TestCursorDir(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "orchard")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(sub, "brook")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(sub, "willow.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !s.SetListLayout(ListLayoutTree, 20) {
		t.Fatal("SetListLayout(Tree) = false")
	}
	if err := s.ExpandTreeCursorRow(20); err != nil {
		t.Fatal(err)
	}
	check := func(name, path, want string) {
		t.Helper()
		if path != "" && !s.SelectVisibleEntryPathCentered(path, 20) {
			t.Fatalf("%s: %s not visible", name, path)
		}
		if got := s.CursorDir().String(); filepath.Clean(got) != want {
			t.Fatalf("%s: CursorDir = %q, want %q", name, got, want)
		}
	}
	check("depth-0 row", sub, root)
	check("nested file", file, sub)
	check("nested dir", inner, sub)

	empty, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := empty.CursorDir(); got != empty.Path {
		t.Fatalf("empty panel CursorDir = %q, want %q", got, empty.Path)
	}
}

// A path-select before an expanded subdirectory's async reload lands must still end on that row
// once the child load applies.
func TestSelectPathArmsTreeCursorUntilAsyncChildLoad(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "orchard")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !s.SetListLayout(ListLayoutTree, 20) {
		t.Fatal("SetListLayout(Tree) = false")
	}
	if err := s.ExpandTreeCursorRow(20); err != nil {
		t.Fatal(err)
	}
	var reqs []TreeChildLoadRequest
	s.ScheduleTreeChildLoad = func(req TreeChildLoadRequest) bool {
		reqs = append(reqs, req)
		return true
	}
	made := filepath.Join(sub, "meadow")
	if err := os.Mkdir(made, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(20); err != nil {
		t.Fatal(err)
	}
	if s.SelectVisibleEntryPathInViewport(made, 20) {
		t.Fatal("nested row should not be loaded yet")
	}
	for _, req := range reqs {
		s.ApplyTreeChildLoad(req.DirID, []localfs.Entry{{Name: "meadow", Path: made, Type: localfs.EntryDirectory}}, nil, 20)
	}
	if cur, ok := s.CurrentEntry(); !ok || filepath.Clean(cur.Path) != made {
		t.Fatalf("cursor on %q, want %q", cur.Path, made)
	}
}
