package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runFlatten(t *testing.T, root, dest string, recursive, removeEmpty bool, resolver ConflictResolver) (int, error) {
	t.Helper()
	done, _, err := ExecuteFlatten(context.Background(), []pathloc.Path{pathloc.MustParse(root)}, pathloc.MustParse(dest),
		recursive, removeEmpty, Options{CopyBufferKiB: 4, FlatDestNames: true}, ProgressEmitThrottle{}, nil, resolver, nil)
	return done, err
}

func TestExecuteFlattenImmediate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root, dest := filepath.Join(dir, "delta"), filepath.Join(dir, "echo")
	if err := os.MkdirAll(filepath.Join(root, "foxtrot"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "foxtrot", "hotel.txt"), "2")
	writeTestFile(t, filepath.Join(root, "golf.txt"), "1")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	done, err := runFlatten(t, root, dest, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if done != 2 {
		t.Fatalf("done = %d, want 2 immediate children", done)
	}
	for _, p := range []string{"golf.txt", filepath.Join("foxtrot", "hotel.txt")} {
		if !exists(filepath.Join(dest, p)) {
			t.Fatalf("missing %q in destination", p)
		}
	}
	if !exists(root) {
		t.Fatal("root must remain without removeEmpty")
	}
}

func TestExecuteFlattenRecursive(t *testing.T) {
	t.Parallel()
	for _, removeEmpty := range []bool{false, true} {
		dir := t.TempDir()
		root, dest := filepath.Join(dir, "hotel"), filepath.Join(dir, "india")
		nested := filepath.Join(root, "juliet", "kilo")
		writeTestFile(t, filepath.Join(nested, "lima.txt"), "1")
		writeTestFile(t, filepath.Join(root, "mike.txt"), "2")
		if err := os.Mkdir(dest, 0o755); err != nil {
			t.Fatal(err)
		}
		done, err := runFlatten(t, root, dest, true, removeEmpty, nil)
		if err != nil {
			t.Fatal(err)
		}
		if done != 2 {
			t.Fatalf("removeEmpty=%v done = %d, want 2", removeEmpty, done)
		}
		for _, name := range []string{"lima.txt", "mike.txt"} {
			if !exists(filepath.Join(dest, name)) {
				t.Fatalf("removeEmpty=%v missing %q", removeEmpty, name)
			}
		}
		if got := exists(root); got == removeEmpty {
			t.Fatalf("removeEmpty=%v root exists = %v", removeEmpty, got)
		}
		if got := exists(nested); got == removeEmpty {
			t.Fatalf("removeEmpty=%v nested exists = %v", removeEmpty, got)
		}
	}
}

func TestExecuteFlattenExpandsSameNameNestedDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "victor")
	writeTestFile(t, filepath.Join(root, "victor", "whiskey.txt"), "1")
	writeTestFile(t, filepath.Join(root, "xray.txt"), "2")
	if _, err := runFlatten(t, root, dir, false, true, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"whiskey.txt", "xray.txt"} {
		if !exists(filepath.Join(dir, name)) {
			t.Fatalf("expected %q in destination", name)
		}
	}
	if exists(root) {
		t.Fatal("emptied root should be removed")
	}
}

func TestExecuteFlattenExpandsDeepSameNameChain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "tango")
	writeTestFile(t, filepath.Join(root, "tango", "tango", "uniform.txt"), "1")
	if _, err := runFlatten(t, root, dir, false, true, nil); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dir, "uniform.txt")) || exists(root) {
		t.Fatal("leaf should land in dest and the chain be removed")
	}
}

func TestExecuteFlattenIncludesDotfiles(t *testing.T) {
	t.Parallel()
	for _, recursive := range []bool{false, true} {
		dir := t.TempDir()
		root, dest := filepath.Join(dir, "mike"), filepath.Join(dir, "november")
		writeTestFile(t, filepath.Join(root, ".papa"), "1")
		writeTestFile(t, filepath.Join(root, ".oscar", "quebec.txt"), "1")
		if err := os.Mkdir(dest, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := runFlatten(t, root, dest, recursive, false, nil); err != nil {
			t.Fatal(err)
		}
		want := ".oscar"
		if recursive {
			want = "quebec.txt"
		}
		if !exists(filepath.Join(dest, ".papa")) || !exists(filepath.Join(dest, want)) {
			t.Fatalf("recursive=%v dotfiles not moved", recursive)
		}
	}
}

func TestExecuteFlattenRejectsDestUnderRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "yankee")
	dest := filepath.Join(root, "zulu")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := runFlatten(t, root, dest, true, true, nil)
	var opsErr *Error
	if !errors.As(err, &opsErr) || opsErr.Text != "destination cannot be inside a selected directory" {
		t.Fatalf("err = %v, want dest-inside-root error", err)
	}
}

func TestExecuteFlattenSkipsItemsAlreadyAtDestination(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "amber")
	writeTestFile(t, filepath.Join(root, "birch.txt"), "1")
	writeTestFile(t, filepath.Join(root, "cedar", "dune.txt"), "2")
	// dest == root: birch.txt is already at its destination; recursion moves dune.txt up.
	done, err := runFlatten(t, root, root, true, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if done != 1 {
		t.Fatalf("done = %d, want 1", done)
	}
	if !exists(filepath.Join(root, "birch.txt")) || !exists(filepath.Join(root, "dune.txt")) || exists(filepath.Join(root, "cedar")) {
		t.Fatal("unexpected layout after flatten into own root")
	}
}

func TestExecuteFlattenSameBasenameAsksResolverOnce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root, dest := filepath.Join(dir, "ember"), filepath.Join(dir, "frost")
	writeTestFile(t, filepath.Join(root, "glade", "heron.txt"), "first")
	writeTestFile(t, filepath.Join(root, "ivory", "heron.txt"), "second")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	calls := 0
	resolver := func(_ context.Context, src, dst string, _ FileConflictFacts) (ConflictResolution, error) {
		calls++
		return ow(false), nil
	}
	if _, err := runFlatten(t, root, dest, true, true, resolver); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("resolver calls = %d, want 1", calls)
	}
	// glade is listed first, so ivory/heron.txt was skipped and keeps ivory alive.
	got, err := os.ReadFile(filepath.Join(dest, "heron.txt"))
	if err != nil || string(got) != "first" {
		t.Fatalf("dest heron = %q, %v", got, err)
	}
	if !exists(filepath.Join(root, "ivory", "heron.txt")) {
		t.Fatal("skipped file must stay in place")
	}
	if exists(filepath.Join(root, "glade")) || !exists(filepath.Join(root, "ivory")) {
		t.Fatal("emptied dir removed, non-empty dir kept")
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

func flattenDeferredFixture(t *testing.T, recursive bool) (dir, root string) {
	t.Helper()
	dir = t.TempDir()
	root = filepath.Join(dir, "lantern")
	inner := root
	if recursive {
		inner = filepath.Join(root, "meadow")
	}
	writeTestFile(t, filepath.Join(inner, "lantern"), "same-name")
	writeTestFile(t, filepath.Join(inner, "pebble"), "sibling")
	return dir, root
}

func TestExecuteFlattenDefersRootNamedItem(t *testing.T) {
	t.Parallel()
	for _, recursive := range []bool{false, true} {
		dir, root := flattenDeferredFixture(t, recursive)
		// Pre-existing temp name forces the numbered fallback.
		writeTestFile(t, filepath.Join(dir, "lantern.flatten"), "x")
		if _, err := runFlatten(t, root, dir, recursive, true, nil); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(dir, "lantern"))
		if err != nil || string(got) != "same-name" {
			t.Fatalf("recursive=%v lantern = %q, %v", recursive, got, err)
		}
		if !exists(filepath.Join(dir, "pebble")) {
			t.Fatalf("recursive=%v pebble not moved", recursive)
		}
		if exists(filepath.Join(dir, "lantern.1.flatten")) {
			t.Fatalf("recursive=%v temp left behind", recursive)
		}
	}
}

func TestExecuteFlattenKeepsTempWhenRootRemains(t *testing.T) {
	t.Parallel()
	dir, root := flattenDeferredFixture(t, false)
	// Without removeEmpty the root stays, so the final name is taken.
	_, err := runFlatten(t, root, dir, false, false, nil)
	if err == nil {
		t.Fatal("want error naming the temp path")
	}
	if got, rerr := os.ReadFile(filepath.Join(dir, "lantern.flatten")); rerr != nil || string(got) != "same-name" {
		t.Fatalf("temp = %q, %v", got, rerr)
	}
}
