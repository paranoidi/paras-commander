package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func exists(p string) bool { _, err := os.Lstat(p); return err == nil }

func countingResolver(answer bool, calls *[]string) ConflictResolver {
	return func(src, dst string, _ FileConflictFacts) (bool, error) {
		*calls = append(*calls, dst)
		return answer, nil
	}
}

func runMove(t *testing.T, src, dstParent string, r ConflictResolver) {
	t.Helper()
	_, _, err := ExecuteMove(context.Background(), MustPaths(src), MustPath(dstParent), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, r, nil)
	if err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
}

func TestMoveMergesIntoExistingDirectory(t *testing.T) {
	base, dstParent := t.TempDir(), t.TempDir()
	src := filepath.Join(base, "meadow")
	writeTree(t, src, map[string]string{"orchard/apple": "a", "orchard/pear": "p", "river/stone": "s"})
	writeTree(t, dstParent, map[string]string{"meadow/orchard/plum": "z"})
	var calls []string
	runMove(t, src, dstParent, countingResolver(true, &calls))
	if len(calls) != 0 {
		t.Fatalf("resolver called: %v", calls)
	}
	for _, rel := range []string{"orchard/apple", "orchard/pear", "orchard/plum", "river/stone"} {
		if !exists(filepath.Join(dstParent, "meadow", rel)) {
			t.Fatalf("missing %s", rel)
		}
	}
	if exists(src) {
		t.Fatal("source dir should be gone")
	}
}

func TestMoveMergeNestedCollision(t *testing.T) {
	for _, overwrite := range []bool{true, false} {
		base, dstParent := t.TempDir(), t.TempDir()
		src := filepath.Join(base, "forest")
		writeTree(t, src, map[string]string{"glade/lantern": "new", "glade/moss": "m", "creek": "c"})
		writeTree(t, dstParent, map[string]string{"forest/glade/lantern": "old"})
		var calls []string
		runMove(t, src, dstParent, countingResolver(overwrite, &calls))
		if len(calls) != 1 {
			t.Fatalf("overwrite=%v resolver calls = %v", overwrite, calls)
		}
		got := readFileContent(t, filepath.Join(dstParent, "forest/glade/lantern"))
		if overwrite {
			if got != "new" || exists(src) {
				t.Fatalf("overwrite: got %q, src exists %v", got, exists(src))
			}
			continue
		}
		if got != "old" || !exists(filepath.Join(src, "glade/lantern")) {
			t.Fatalf("skip: dst %q, src lantern kept %v", got, exists(filepath.Join(src, "glade/lantern")))
		}
		if exists(filepath.Join(src, "glade/moss")) || exists(filepath.Join(src, "creek")) {
			t.Fatal("non-colliding items should have moved")
		}
		if !exists(filepath.Join(dstParent, "forest/glade/moss")) || !exists(filepath.Join(dstParent, "forest/creek")) {
			t.Fatal("non-colliding items missing at dst")
		}
	}
}

func TestMoveDirOntoFileAsksResolver(t *testing.T) {
	base, dstParent := t.TempDir(), t.TempDir()
	src := filepath.Join(base, "canyon")
	writeTree(t, src, map[string]string{"echo": "e"})
	writeTree(t, dstParent, map[string]string{"canyon": "plain"})
	var calls []string
	runMove(t, src, dstParent, countingResolver(false, &calls))
	if len(calls) != 1 || !exists(filepath.Join(src, "echo")) {
		t.Fatalf("calls=%v", calls)
	}
}

func TestMoveMergeDoesNotDescendSymlinkSource(t *testing.T) {
	base, dstParent, other := t.TempDir(), t.TempDir(), t.TempDir()
	writeTree(t, other, map[string]string{"pebble": "x"})
	link := filepath.Join(base, "bridge")
	if err := os.Symlink(other, link); err != nil {
		t.Skip(err)
	}
	writeTree(t, dstParent, map[string]string{"bridge/keep": "k"})
	var calls []string
	runMove(t, link, dstParent, countingResolver(false, &calls))
	if len(calls) != 1 {
		t.Fatalf("symlink over dir should ask resolver, calls=%v", calls)
	}
	if !exists(filepath.Join(other, "pebble")) || exists(filepath.Join(dstParent, "bridge", "pebble")) {
		t.Fatal("symlink target contents were touched")
	}
}
