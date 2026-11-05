package ops

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/fschange"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// memIndexBackend is an in-memory, read-only-for-content fake of a fileindex:// backend: it
// lists and stats a set of paths, renames and removes whole trees, and refuses writes.
type memIndexBackend struct {
	dirs, files map[string]bool // fileindex:// strings
	treeRemoved []string
	removed     []string
}

func newMemIndex(dirs, files []string) *memIndexBackend {
	m := &memIndexBackend{dirs: map[string]bool{}, files: map[string]bool{}}
	for _, d := range dirs {
		m.dirs[d] = true
	}
	for _, f := range files {
		m.files[f] = true
	}
	return m
}

func (m *memIndexBackend) SameRenameRoot(_, _ pathloc.Path) bool { return true }

func (m *memIndexBackend) Scheme() pathloc.Scheme { return pathloc.SchemeFileIndex }

func (m *memIndexBackend) List(_ context.Context, dir pathloc.Path) ([]fsbackend.Entry, error) {
	var out []fsbackend.Entry
	for set, typ := range map[*map[string]bool]fsbackend.EntryType{&m.dirs: fsbackend.EntryDirectory, &m.files: fsbackend.EntryFile} {
		for p := range *set {
			loc := pathloc.MustParse(p)
			if loc.Parent().Equal(dir) && !loc.Equal(dir) {
				out = append(out, fsbackend.Entry{Name: loc.Base(), Loc: loc, Type: typ})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *memIndexBackend) Stat(_ context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	switch {
	case m.dirs[loc.String()]:
		return fsbackend.Entry{Name: loc.Base(), Loc: loc, Type: fsbackend.EntryDirectory}, nil
	case m.files[loc.String()]:
		return fsbackend.Entry{Name: loc.Base(), Loc: loc, Type: fsbackend.EntryFile}, nil
	}
	return fsbackend.Entry{}, fs.ErrNotExist
}

func (m *memIndexBackend) OpenRead(context.Context, pathloc.Path) (io.ReadSeekCloser, error) {
	return nil, fs.ErrNotExist
}

func (m *memIndexBackend) OpenWrite(context.Context, pathloc.Path, int64, fsbackend.CreateOpts) (io.WriteCloser, error) {
	return nil, fsbackend.ErrReadOnly
}

func (m *memIndexBackend) Mkdir(context.Context, pathloc.Path, fs.FileMode) error {
	return fsbackend.ErrReadOnly
}

func (m *memIndexBackend) Remove(_ context.Context, loc pathloc.Path) error {
	m.removed = append(m.removed, loc.String())
	delete(m.files, loc.String())
	delete(m.dirs, loc.String())
	return nil
}

func (m *memIndexBackend) RemoveTree(_ context.Context, dir pathloc.Path) error {
	m.treeRemoved = append(m.treeRemoved, dir.String())
	for _, set := range []map[string]bool{m.dirs, m.files} {
		for p := range set {
			if pathloc.MustParse(p).HasPrefix(dir) {
				delete(set, p)
			}
		}
	}
	return nil
}

func (m *memIndexBackend) Rename(_ context.Context, oldLoc, newLoc pathloc.Path) error {
	for _, set := range []map[string]bool{m.dirs, m.files} {
		for p := range set {
			if loc := pathloc.MustParse(p); loc.HasPrefix(oldLoc) {
				delete(set, p)
				set[newLoc.String()+strings.TrimPrefix(p, oldLoc.String())] = true
			}
		}
	}
	return nil
}

func (m *memIndexBackend) ReadSymlink(context.Context, pathloc.Path) (string, error) {
	return "", fsbackend.ErrReadOnly
}

func (m *memIndexBackend) Symlink(context.Context, pathloc.Path, string) error {
	return fsbackend.ErrReadOnly
}

func useMemIndex(t *testing.T, m *memIndexBackend) {
	t.Helper()
	prev := backendFor
	backendFor = func(loc pathloc.Path) (fsbackend.Backend, error) {
		if loc.Scheme() == pathloc.SchemeFileIndex {
			return m, nil
		}
		return prev(loc)
	}
	t.Cleanup(func() { backendFor = prev })
}

func TestFileIndexSchemeMkdirIsReadOnly(t *testing.T) {
	useMemIndex(t, newMemIndex([]string{"fileindex:///orchard"}, nil))
	err := ExecuteMkdir(context.Background(), MkdirPlan{Path: "fileindex:///orchard/pebble"})
	if !errors.Is(err, fsbackend.ErrReadOnly) {
		t.Fatalf("mkdir err = %v, want ErrReadOnly", err)
	}
}

func TestFileIndexSchemeDeleteUsesTreeRemover(t *testing.T) {
	m := newMemIndex(
		[]string{"fileindex:///orchard", "fileindex:///orchard/lantern"},
		[]string{"fileindex:///orchard/lantern/meadow.txt", "fileindex:///orchard/harbor.txt"},
	)
	useMemIndex(t, m)
	done, _, err := ExecuteDeletePaths(context.Background(), []string{"fileindex:///orchard/lantern", "fileindex:///orchard/harbor.txt"}, nil, nil, nil)
	if err != nil || done != 2 {
		t.Fatalf("delete = %d, %v", done, err)
	}
	if len(m.treeRemoved) != 1 || m.treeRemoved[0] != "fileindex:///orchard/lantern" {
		t.Fatalf("tree removes = %v, want the directory in one call", m.treeRemoved)
	}
	if len(m.removed) != 1 || m.removed[0] != "fileindex:///orchard/harbor.txt" {
		t.Fatalf("removes = %v, want the file only", m.removed)
	}
	if m.files["fileindex:///orchard/lantern/meadow.txt"] {
		t.Fatal("tree child survived")
	}
}

func TestFileIndexSchemeRenameFastPath(t *testing.T) {
	m := newMemIndex([]string{"fileindex:///orchard", "fileindex:///cedar"}, []string{"fileindex:///orchard/harbor.txt"})
	useMemIndex(t, m)
	ok, err := RenameFastPathCtx(context.Background(), pathloc.MustParse("fileindex:///orchard/harbor.txt"), pathloc.MustParse("fileindex:///cedar/harbor.txt"))
	if err != nil || !ok {
		t.Fatalf("rename fast path = %v, %v; want a backend rename inside the index", ok, err)
	}
	if !m.files["fileindex:///cedar/harbor.txt"] {
		t.Fatalf("files = %v", m.files)
	}
	ok, _ = RenameFastPathCtx(context.Background(), pathloc.MustParse("fileindex:///cedar/harbor.txt"), pathloc.MustParse("/tmp/harbor.txt"))
	if ok {
		t.Fatal("a rename out of the index must not take the fast path")
	}
}

// The tests below use real directories: a fileindex:// path counts as on disk only when its host
// path exists.

type recordSink struct {
	mu  sync.Mutex
	got []fschange.Change
}

func (r *recordSink) Apply(b []fschange.Change) {
	r.mu.Lock()
	r.got = append(r.got, b...)
	r.mu.Unlock()
}

func indexLoc(t *testing.T, host string) string {
	t.Helper()
	loc, err := pathloc.FileIndex(host)
	if err != nil {
		t.Fatal(err)
	}
	return loc.String()
}

func TestFileIndexDeleteOfEntryOnDiskActsOnDisk(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "harbor.txt")
	writeTestFile(t, file, "anchor")
	m := newMemIndex([]string{indexLoc(t, root)}, []string{indexLoc(t, file)})
	useMemIndex(t, m)
	sink := &recordSink{}
	ctx := fschange.WithSink(context.Background(), sink)
	done, _, err := ExecuteDeletePaths(ctx, []string{indexLoc(t, file)}, nil, nil, nil)
	if err != nil || done != 1 {
		t.Fatalf("delete = %d, %v", done, err)
	}
	if exists(file) {
		t.Fatal("the file is still on disk")
	}
	if len(m.removed) != 0 || len(m.treeRemoved) != 0 {
		t.Fatalf("the index backend was used (%v, %v); the disk delete reports to the sink instead", m.removed, m.treeRemoved)
	}
	if len(sink.got) != 1 || sink.got[0].Kind != fschange.Removed || sink.got[0].Path != file {
		t.Fatalf("reported %+v, want one Removed change for the disk path", sink.got)
	}
}

func TestFileIndexDeleteOfEntryGoneFromDiskIsIndexOnly(t *testing.T) {
	root := t.TempDir()
	gone := filepath.Join(root, "bramble.txt")
	m := newMemIndex([]string{indexLoc(t, root)}, []string{indexLoc(t, gone)})
	useMemIndex(t, m)
	sink := &recordSink{}
	ctx := fschange.WithSink(context.Background(), sink)
	if done, _, err := ExecuteDeletePaths(ctx, []string{indexLoc(t, gone)}, nil, nil, nil); err != nil || done != 1 {
		t.Fatalf("delete = %d, %v", done, err)
	}
	if len(m.removed) != 1 || len(sink.got) != 0 {
		t.Fatalf("backend removes %v, reported %+v; want the record removed through the backend only", m.removed, sink.got)
	}
}

func TestFileIndexDeleteOfDirectoryRemovesUnindexedFilesToo(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "lantern")
	indexed := filepath.Join(dir, "meadow.txt")
	unindexed := filepath.Join(dir, "node_modules", "pebble.txt")
	writeTestFile(t, indexed, "clover")
	writeTestFile(t, unindexed, "gravel")
	// The fake index lists the one indexed file only, as the real backend does.
	m := newMemIndex([]string{indexLoc(t, root), indexLoc(t, dir)}, []string{indexLoc(t, indexed)})
	useMemIndex(t, m)
	if _, _, err := ExecuteDeletePaths(context.Background(), []string{indexLoc(t, dir)}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if exists(dir) {
		t.Fatalf("the directory survived (indexed file there: %v, unindexed: %v)", exists(indexed), exists(unindexed))
	}
	if len(m.treeRemoved) != 0 {
		t.Fatalf("the index backend removed trees %v; the disk delete must do it", m.treeRemoved)
	}
}

func TestFileIndexRenameOnDisk(t *testing.T) {
	root := t.TempDir()
	oldP, newP := filepath.Join(root, "harbor.txt"), filepath.Join(root, "anchor.txt")
	writeTestFile(t, oldP, "rope")
	m := newMemIndex([]string{indexLoc(t, root)}, []string{indexLoc(t, oldP)})
	useMemIndex(t, m)
	sink := &recordSink{}
	plan := RenamePlan{SourcePath: indexLoc(t, oldP), NewName: "anchor.txt", NewPath: indexLoc(t, newP)}
	if err := ExecuteRename(fschange.WithSink(context.Background(), sink), plan); err != nil {
		t.Fatal(err)
	}
	if exists(oldP) || !exists(newP) {
		t.Fatalf("on disk: old %v, new %v; want renamed", exists(oldP), exists(newP))
	}
	if m.files[indexLoc(t, newP)] {
		t.Fatal("the index backend was renamed; the disk rename reports to the sink instead")
	}
	if len(sink.got) != 1 || sink.got[0].Kind != fschange.Renamed || sink.got[0].From != oldP || sink.got[0].Path != newP {
		t.Fatalf("reported %+v, want one Renamed change on the disk paths", sink.got)
	}
}

func TestFileIndexRenameOfEntryGoneFromDiskIsIndexOnly(t *testing.T) {
	root := t.TempDir()
	oldP, newP := filepath.Join(root, "harbor.txt"), filepath.Join(root, "anchor.txt")
	m := newMemIndex([]string{indexLoc(t, root)}, []string{indexLoc(t, oldP)})
	useMemIndex(t, m)
	plan := RenamePlan{SourcePath: indexLoc(t, oldP), NewName: "anchor.txt", NewPath: indexLoc(t, newP)}
	if err := ExecuteRename(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	if !m.files[indexLoc(t, newP)] || m.files[indexLoc(t, oldP)] {
		t.Fatalf("index records %v, want the record renamed", m.files)
	}
	if exists(newP) {
		t.Fatal("a rename of a record must not create a file")
	}
}

func TestFileIndexRenameOntoUnindexedFileOnDiskIsRefused(t *testing.T) {
	root := t.TempDir()
	oldP, newP := filepath.Join(root, "harbor.txt"), filepath.Join(root, "anchor.txt")
	writeTestFile(t, oldP, "rope")
	writeTestFile(t, newP, "chain") // on disk, not in the index: rename(2) would replace it silently
	m := newMemIndex([]string{indexLoc(t, root)}, []string{indexLoc(t, oldP)})
	useMemIndex(t, m)
	plan := RenamePlan{SourcePath: indexLoc(t, oldP), NewName: "anchor.txt", NewPath: indexLoc(t, newP)}
	if err := ExecuteRename(context.Background(), plan); err == nil {
		t.Fatal("renaming onto an existing file succeeded")
	}
	if body, _ := os.ReadFile(newP); string(body) != "chain" || !exists(oldP) {
		t.Fatalf("the files changed: target %q, source present %v", body, exists(oldP))
	}
}

func TestFileIndexParentCleanupNeverTouchesTheIndexBackend(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "orchard")
	writeTestFile(t, filepath.Join(parent, "node_modules", "pebble.txt"), "gravel") // unindexed content
	emptyKid := filepath.Join(parent, "empty")
	if err := os.MkdirAll(emptyKid, 0o755); err != nil {
		t.Fatal(err)
	}
	gone := indexLoc(t, filepath.Join(parent, "harbor.txt")) // the deleted source, gone from disk
	// The index lists the parent as empty, so listing-based cleanup would remove it.
	m := newMemIndex([]string{indexLoc(t, root), indexLoc(t, parent)}, nil)
	useMemIndex(t, m)
	sources := []pathloc.Path{pathloc.MustParse(gone)}
	if err := RemoveEmptyDirsUnder(context.Background(), []pathloc.Path{sources[0].Parent()}); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(parent, "node_modules", "pebble.txt")) {
		t.Fatal("cleanup removed unindexed content")
	}
	if exists(emptyKid) {
		t.Fatal("cleanup left an empty directory on disk")
	}
	if len(m.removed) != 0 || len(m.treeRemoved) != 0 {
		t.Fatalf("cleanup used the index backend: %v %v", m.removed, m.treeRemoved)
	}
	dirs, err := DanglingDirsAfter(context.Background(), sources)
	if err != nil || len(dirs) != 0 {
		t.Fatalf("dangling = %v, %v; want none (the parent still holds files)", dirs, err)
	}
}

func TestFileIndexPathsAreNeverEmptyDirsOrFlattened(t *testing.T) {
	loc := pathloc.MustParse("fileindex:///orchard/lantern")
	m := newMemIndex([]string{loc.String()}, nil)
	useMemIndex(t, m)
	be, _ := backendFor(loc)
	if removed, err := removeDirIfEmpty(context.Background(), be, loc, "test"); removed || err != nil || len(m.treeRemoved)+len(m.removed) != 0 {
		t.Fatalf("removeDirIfEmpty on the index = %v, %v, backend removes %v %v", removed, err, m.removed, m.treeRemoved)
	}
	_, _, err := ExecuteFlatten(context.Background(), []pathloc.Path{loc}, pathloc.MustParse("/meadow"), true, true, Options{}, ProgressEmitThrottle{}, nil, nil, nil)
	if err == nil {
		t.Fatal("flatten of a fileindex:// root must be refused")
	}
}
