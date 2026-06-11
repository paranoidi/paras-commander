package ops

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestValidateFlattenSourceMixedSelection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "alpha.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(dir, "bravo")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := &panel.State{
		SelectedPaths: map[string]bool{
			filepath.Clean(filePath): true,
			filepath.Clean(subDir):   true,
		},
	}
	_, err := ValidateFlattenSource(p)
	if err == nil {
		t.Fatal("expected error for mixed selection")
	}
	opsErr, ok := err.(*Error)
	if !ok || opsErr.Text != "cannot mix files and directories in selection" {
		t.Fatalf("err = %v, want mixed-selection message", err)
	}
}

func TestCollectFlattenSourcesImmediate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "delta")
	dest := filepath.Join(dir, "echo")
	if err := os.MkdirAll(filepath.Join(root, "foxtrot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "golf.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	rootLoc := pathloc.MustParse(root)
	destLoc := pathloc.MustParse(dest)
	got, err := CollectFlattenSources(context.Background(), []pathloc.Path{rootLoc}, destLoc, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("sources = %v, want 2 immediate children", got)
	}
}

func TestCollectFlattenSourcesExpandsSameNameNestedDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "quebec")
	dest := dir
	nested := filepath.Join(root, "quebec")
	innerFile := filepath.Join(nested, "romeo.txt")
	siblingFile := filepath.Join(root, "sierra.txt")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(innerFile, []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(siblingFile, []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootLoc := pathloc.MustParse(root)
	destLoc := pathloc.MustParse(dest)
	got, err := CollectFlattenSources(context.Background(), []pathloc.Path{rootLoc}, destLoc, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Clean(innerFile), filepath.Clean(siblingFile)}
	if len(got) != len(want) {
		t.Fatalf("sources = %v, want %v", got, want)
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Fatalf("sources = %v, missing %q", got, w)
		}
	}
	nestedDir := filepath.Clean(nested)
	for _, s := range got {
		if s == nestedDir {
			t.Fatalf("sources = %v, must not include colliding directory %q", got, nestedDir)
		}
	}
}

func TestCollectFlattenSourcesExpandsDeepSameNameChain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "tango")
	dest := dir
	leaf := filepath.Join(root, "tango", "tango", "uniform.txt")
	if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(leaf, []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootLoc := pathloc.MustParse(root)
	destLoc := pathloc.MustParse(dest)
	got, err := CollectFlattenSources(context.Background(), []pathloc.Path{rootLoc}, destLoc, false)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean(leaf)
	if len(got) != 1 || got[0] != want {
		t.Fatalf("sources = %v, want [%q]", got, want)
	}
}

func TestFlattenSameNameNestedDirMove(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "victor")
	dest := dir
	nested := filepath.Join(root, "victor")
	innerFile := filepath.Join(nested, "whiskey.txt")
	siblingFile := filepath.Join(root, "xray.txt")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(innerFile, []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(siblingFile, []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootLoc := pathloc.MustParse(root)
	destLoc := pathloc.MustParse(dest)
	sources, err := CollectFlattenSources(context.Background(), []pathloc.Path{rootLoc}, destLoc, false)
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{CopyBufferKiB: 4}
	done, _, err := ExecuteMove(context.Background(), MustPaths(sources...), destLoc, opts, ProgressEmitThrottle{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
	if done < 2 {
		t.Fatalf("done files = %d, want >= 2", done)
	}
	for _, name := range []string{"whiskey.txt", "xray.txt"} {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Fatalf("expected %q in destination: %v", name, err)
		}
	}
	if _, err := os.Stat(innerFile); !os.IsNotExist(err) {
		t.Fatalf("inner source should be moved: %v", err)
	}
	if _, err := os.Stat(siblingFile); !os.IsNotExist(err) {
		t.Fatalf("sibling source should be moved: %v", err)
	}
}

func TestCollectFlattenSourcesRecursive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "hotel")
	dest := filepath.Join(dir, "india")
	nested := filepath.Join(root, "juliet", "kilo")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "lima.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	rootLoc := pathloc.MustParse(root)
	destLoc := pathloc.MustParse(dest)
	got, err := CollectFlattenSources(context.Background(), []pathloc.Path{rootLoc}, destLoc, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("sources = %v, want 1 file", got)
	}
	if filepath.Base(got[0]) != "lima.txt" {
		t.Fatalf("source = %q, want lima.txt", got[0])
	}
}

func TestRemoveEmptyDirsUnder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "mike")
	empty := filepath.Join(root, "november")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "oscar.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootLoc := pathloc.MustParse(root)
	if err := RemoveEmptyDirsUnder(context.Background(), []pathloc.Path{rootLoc}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(empty); !os.IsNotExist(err) {
		t.Fatalf("empty subdir should be removed: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("root with file should remain: %v", err)
	}
}

func TestValidateFlattenSourceDirectoryOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sub := filepath.Join(dir, "papa")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	p := &panel.State{
		Entries: []localfs.Entry{{
			Name: "papa",
			Path: filepath.Clean(sub),
			Type: localfs.EntryDirectory,
		}},
		Cursor: 0,
	}
	roots, err := ValidateFlattenSource(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 {
		t.Fatalf("roots = %d, want 1", len(roots))
	}
}
