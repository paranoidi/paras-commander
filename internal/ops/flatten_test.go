package ops

import (
	"context"
	"errors"
	"fmt"
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
	got, _, err := CollectFlattenSources(context.Background(), []pathloc.Path{rootLoc}, destLoc, false)
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
	got, _, err := CollectFlattenSources(context.Background(), []pathloc.Path{rootLoc}, destLoc, false)
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
	got, _, err := CollectFlattenSources(context.Background(), []pathloc.Path{rootLoc}, destLoc, false)
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
	sources, _, err := CollectFlattenSources(context.Background(), []pathloc.Path{rootLoc}, destLoc, false)
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{CopyBufferKiB: 4, FlatDestNames: true}
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
	got, _, err := CollectFlattenSources(context.Background(), []pathloc.Path{rootLoc}, destLoc, true)
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

func TestCollectFlattenSourcesHonorsCancelOnLargeTree(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "harbor")
	dest := filepath.Join(dir, "meadow")
	nested := filepath.Join(root, "lantern")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		name := filepath.Join(nested, fmt.Sprintf("willow-%03d.txt", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	SetCollectFlattenTestHook(func(context.Context) { cancel() })
	t.Cleanup(func() { SetCollectFlattenTestHook(nil) })
	_, _, err := CollectFlattenSources(ctx, []pathloc.Path{pathloc.MustParse(root)}, pathloc.MustParse(dest), true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
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

func TestCollectFlattenSourcesIncludesDotfiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "mike")
	dest := filepath.Join(dir, "november")
	if err := os.MkdirAll(filepath.Join(root, ".oscar"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".papa"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".oscar", "quebec.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	roots := []pathloc.Path{pathloc.MustParse(root)}
	destLoc := pathloc.MustParse(dest)
	for _, tc := range []struct {
		recursive bool
		want      []string
	}{
		{false, []string{filepath.Join(root, ".oscar"), filepath.Join(root, ".papa")}},
		{true, []string{filepath.Join(root, ".oscar", "quebec.txt"), filepath.Join(root, ".papa")}},
	} {
		got, _, err := CollectFlattenSources(context.Background(), roots, destLoc, tc.recursive)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, tc.want) {
			t.Fatalf("recursive=%v: sources = %v, want %v", tc.recursive, got, tc.want)
		}
	}
}

func flattenDeferredFixture(t *testing.T, recursive bool) (dir string, rootLoc, destLoc pathloc.Path) {
	t.Helper()
	dir = t.TempDir()
	root := filepath.Join(dir, "lantern")
	inner := root
	if recursive {
		inner = filepath.Join(root, "meadow")
	}
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"lantern": "same-name", "pebble": "sibling"} {
		if err := os.WriteFile(filepath.Join(inner, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, pathloc.MustParse(root), pathloc.MustParse(dir)
}

func TestCollectFlattenSourcesDefersRootNamedItem(t *testing.T) {
	t.Parallel()
	for _, recursive := range []bool{false, true} {
		dir, rootLoc, destLoc := flattenDeferredFixture(t, recursive)
		sources, deferred, err := CollectFlattenSources(context.Background(), []pathloc.Path{rootLoc}, destLoc, recursive)
		if err != nil {
			t.Fatal(err)
		}
		if len(sources) != 1 || filepath.Base(sources[0]) != "pebble" {
			t.Fatalf("recursive=%v sources = %v, want only pebble", recursive, sources)
		}
		if len(deferred) != 1 || filepath.Base(deferred[0]) != "lantern" {
			t.Fatalf("recursive=%v deferred = %v, want the lantern file", recursive, deferred)
		}

		// Simulate the main move, then finish.
		if err := os.Rename(sources[0], filepath.Join(dir, "pebble")); err != nil {
			t.Fatal(err)
		}
		// Pre-existing temp name forces the numbered fallback.
		if err := os.WriteFile(filepath.Join(dir, "lantern.flatten"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		def := []pathloc.Path{pathloc.MustParse(deferred[0])}
		if err := FinishFlattenDeferred(context.Background(), def, destLoc, []pathloc.Path{rootLoc}, true); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(dir, "lantern"))
		if err != nil || string(got) != "same-name" {
			t.Fatalf("recursive=%v lantern = %q, %v", recursive, got, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "lantern.1.flatten")); !os.IsNotExist(err) {
			t.Fatalf("recursive=%v temp left behind: %v", recursive, err)
		}
	}
}

func TestFinishFlattenDeferredKeepsTempWhenRootRemains(t *testing.T) {
	t.Parallel()
	dir, rootLoc, destLoc := flattenDeferredFixture(t, false)
	_, deferred, err := CollectFlattenSources(context.Background(), []pathloc.Path{rootLoc}, destLoc, false)
	if err != nil {
		t.Fatal(err)
	}
	// pebble stays in the root, so the root is not removed.
	err = FinishFlattenDeferred(context.Background(), []pathloc.Path{pathloc.MustParse(deferred[0])}, destLoc, []pathloc.Path{rootLoc}, true)
	if err == nil {
		t.Fatal("want error naming the temp path")
	}
	if got, rerr := os.ReadFile(filepath.Join(dir, "lantern.flatten")); rerr != nil || string(got) != "same-name" {
		t.Fatalf("temp = %q, %v", got, rerr)
	}
}
