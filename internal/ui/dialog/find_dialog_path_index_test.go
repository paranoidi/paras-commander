package dialog

import (
	"path/filepath"
	"testing"
)

func TestFindDialogStatePathIndex(t *testing.T) {
	t.Parallel()
	st := &FindDialogState{
		RootPath: filepath.FromSlash("/root"),
		Entries: []FindEntry{
			{RelLine: "alpha.txt"},
			{RelLine: "beta", IsDir: true},
		},
	}
	st.RebuildPathIndex()

	idx, ok := st.PathIndexLookup(filepath.Clean(filepath.Join(st.RootPath, "alpha.txt")))
	if !ok || idx != 0 {
		t.Fatalf("alpha.txt: got idx=%d ok=%v, want idx=0 ok=true", idx, ok)
	}
	if _, ok := st.PathIndexLookup(filepath.Clean(filepath.Join(st.RootPath, "missing.txt"))); ok {
		t.Fatal("missing.txt: expected lookup miss")
	}

	from := len(st.Entries)
	st.Entries = append(st.Entries, FindEntry{RelLine: "gamma.txt"})
	st.ExtendPathIndex(from)

	idx, ok = st.PathIndexLookup(filepath.Clean(filepath.Join(st.RootPath, "gamma.txt")))
	if !ok || idx != 2 {
		t.Fatalf("gamma.txt after extend: got idx=%d ok=%v, want idx=2 ok=true", idx, ok)
	}
	idx, ok = st.PathIndexLookup(filepath.Clean(filepath.Join(st.RootPath, "beta")))
	if !ok || idx != 1 {
		t.Fatalf("beta survives extend: got idx=%d ok=%v, want idx=1 ok=true", idx, ok)
	}

	st.Entries = nil
	st.RebuildPathIndex()
	if _, ok := st.PathIndexLookup(filepath.Clean(filepath.Join(st.RootPath, "alpha.txt"))); ok {
		t.Fatal("alpha.txt: expected lookup miss after reset")
	}
}
