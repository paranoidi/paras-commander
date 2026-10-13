package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeMismatchTree(t *testing.T, files map[string]string) {
	t.Helper()
	for p, c := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func mismatchResolver(a ConflictAction, mismatches *int) ConflictResolver {
	return func(_ context.Context, _, _ string, f FileConflictFacts) (ConflictResolution, error) {
		if f.OverwriteUnsafe() && mismatches != nil {
			*mismatches++
		}
		return ConflictResolution{Action: a}, nil
	}
}

type mismatchRunner struct {
	name string
	run  func(t *testing.T, src, dstDir string, r ConflictResolver) error
}

func mismatchRunners() []mismatchRunner {
	opts := Options{CopyBufferKiB: 4}
	return []mismatchRunner{
		{"move", func(t *testing.T, src, dstDir string, r ConflictResolver) error {
			_, _, err := ExecuteMove(context.Background(), MustPaths(src), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, r, nil)
			return err
		}},
		{"copy", func(t *testing.T, src, dstDir string, r ConflictResolver) error {
			_, _, err := ExecuteCopy(context.Background(), MustPaths(src), MustPath(dstDir), opts, ProgressEmitThrottle{}, nil, r, nil)
			return err
		}},
	}
}

func TestOverwriteNeverDeletesDirectoryOnMismatch(t *testing.T) {
	for _, rn := range mismatchRunners() {
		t.Run(rn.name+"/file onto directory", func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "in", "orchard")
			dstDir := filepath.Join(root, "out")
			writeMismatchTree(t, map[string]string{src: "new", filepath.Join(dstDir, "orchard", "pear.txt"): "keep"})
			var n int
			if err := rn.run(t, src, dstDir, mismatchResolver(ActionOverwrite, &n)); err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Fatalf("mismatch facts seen %d times, want 1", n)
			}
			if got := readFileContent(t, filepath.Join(dstDir, "orchard", "pear.txt")); got != "keep" {
				t.Fatalf("directory contents lost: %q", got)
			}
			if _, err := os.Stat(src); err != nil {
				t.Fatalf("source must stay when skipped: %v", err)
			}
		})
		t.Run(rn.name+"/nested file onto directory inside merged dir", func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "in", "meadow")
			dstDir := filepath.Join(root, "out")
			writeMismatchTree(t, map[string]string{
				filepath.Join(src, "brook"):                           "new",
				filepath.Join(src, "clover.txt"):                      "new-clover",
				filepath.Join(dstDir, "meadow", "brook", "stone.txt"): "keep",
			})
			if err := rn.run(t, src, dstDir, mismatchResolver(ActionOverwrite, nil)); err != nil {
				t.Fatal(err)
			}
			if got := readFileContent(t, filepath.Join(dstDir, "meadow", "brook", "stone.txt")); got != "keep" {
				t.Fatalf("nested directory contents lost: %q", got)
			}
			if got := readFileContent(t, filepath.Join(dstDir, "meadow", "clover.txt")); got != "new-clover" {
				t.Fatalf("non-colliding sibling not merged: %q", got)
			}
		})
		t.Run(rn.name+"/directory onto file", func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "in", "lantern")
			dstDir := filepath.Join(root, "out")
			writeMismatchTree(t, map[string]string{filepath.Join(src, "wick.txt"): "wick", filepath.Join(dstDir, "lantern"): "keep"})
			if err := rn.run(t, src, dstDir, mismatchResolver(ActionOverwrite, nil)); err != nil {
				t.Fatal(err)
			}
			if got := readFileContent(t, filepath.Join(dstDir, "lantern")); got != "keep" {
				t.Fatalf("file replaced by directory: %q", got)
			}
			if _, err := os.Stat(filepath.Join(src, "wick.txt")); err != nil {
				t.Fatalf("source child must stay when skipped: %v", err)
			}
		})
		t.Run(rn.name+"/file onto symlink to directory", func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "in", "harbor")
			dstDir := filepath.Join(root, "out")
			target := filepath.Join(root, "elsewhere")
			writeMismatchTree(t, map[string]string{src: "new", filepath.Join(target, "boat.txt"): "keep"})
			if err := os.MkdirAll(dstDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(dstDir, "harbor")); err != nil {
				t.Fatal(err)
			}
			if err := rn.run(t, src, dstDir, mismatchResolver(ActionOverwrite, nil)); err != nil {
				t.Fatal(err)
			}
			if got := readFileContent(t, filepath.Join(target, "boat.txt")); got != "keep" {
				t.Fatalf("link target contents lost: %q", got)
			}
			if fi, err := os.Lstat(filepath.Join(dstDir, "harbor")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("symlink replaced: %v", err)
			}
		})
		t.Run(rn.name+"/directory onto symlink to directory is not merged through", func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "in", "glacier")
			dstDir := filepath.Join(root, "out")
			target := filepath.Join(root, "elsewhere")
			writeMismatchTree(t, map[string]string{filepath.Join(src, "ice.txt"): "ice", filepath.Join(target, "boat.txt"): "keep"})
			if err := os.MkdirAll(dstDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(dstDir, "glacier")); err != nil {
				t.Fatal(err)
			}
			if err := rn.run(t, src, dstDir, mismatchResolver(ActionOverwrite, nil)); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(target, "ice.txt")); !os.IsNotExist(err) {
				t.Fatalf("merged through symlink: %v", err)
			}
			if got := readFileContent(t, filepath.Join(target, "boat.txt")); got != "keep" {
				t.Fatalf("link target contents lost: %q", got)
			}
		})
		t.Run(rn.name+"/rename keeps both", func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "in", "orchard")
			dstDir := filepath.Join(root, "out")
			writeMismatchTree(t, map[string]string{src: "new", filepath.Join(dstDir, "orchard", "pear.txt"): "keep"})
			if err := rn.run(t, src, dstDir, mismatchResolver(ActionRename, nil)); err != nil {
				t.Fatal(err)
			}
			if got := readFileContent(t, filepath.Join(dstDir, "orchard", "pear.txt")); got != "keep" {
				t.Fatalf("directory contents lost: %q", got)
			}
			if got := readFileContent(t, filepath.Join(dstDir, "orchard (1)")); got != "new" {
				t.Fatalf("renamed copy = %q", got)
			}
		})
		t.Run(rn.name+"/directory onto file rename keeps both", func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "in", "lantern")
			dstDir := filepath.Join(root, "out")
			writeMismatchTree(t, map[string]string{filepath.Join(src, "wick.txt"): "wick", filepath.Join(dstDir, "lantern"): "keep"})
			if err := rn.run(t, src, dstDir, mismatchResolver(ActionRename, nil)); err != nil {
				t.Fatal(err)
			}
			if got := readFileContent(t, filepath.Join(dstDir, "lantern")); got != "keep" {
				t.Fatalf("file replaced: %q", got)
			}
			if got := readFileContent(t, filepath.Join(dstDir, "lantern (1)", "wick.txt")); got != "wick" {
				t.Fatalf("renamed directory child = %q", got)
			}
		})
	}
}

func TestFileConflictFactsMismatch(t *testing.T) {
	cases := []struct {
		src, dst PathTarget
		want     bool
	}{
		{TargetOther, TargetOther, false},
		{TargetOther, TargetDir, true},
		{TargetDir, TargetOther, true},
		{TargetOther, TargetDirLink, true},
		{TargetDir, TargetDirLink, true},
		{TargetDirLink, TargetDir, true},
		{TargetDirLink, TargetDirLink, false},
		{TargetDir, TargetDir, true},
	}
	for _, c := range cases {
		if got := (FileConflictFacts{SourceTarget: c.src, DestTarget: c.dst}).OverwriteUnsafe(); got != c.want {
			t.Errorf("Mismatch(%v,%v) = %v, want %v", c.src, c.dst, got, c.want)
		}
	}
}

func TestResolveDestConflictOverwriteKeepsDirectory(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "reed")
	dst := filepath.Join(root, "out", "reed")
	writeMismatchTree(t, map[string]string{src: "new", filepath.Join(dst, "pond.txt"): "keep"})
	res, _, err := resolveDestConflict(context.Background(), MustPath(src), MustPath(dst), mismatchResolver(ActionOverwrite, nil))
	if err != nil || res != copySkipped {
		t.Fatalf("res = %v, err = %v; want copySkipped", res, err)
	}
	if got := readFileContent(t, filepath.Join(dst, "pond.txt")); got != "keep" {
		t.Fatalf("directory contents lost: %q", got)
	}
}
