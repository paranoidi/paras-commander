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

// The tests below use real directories: a fileindex:// path is the index's, and the disk beside it
// must stay as it is.

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

func TestFileIndexDeleteOfEntryOnDiskLeavesDiskAlone(t *testing.T) {
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
	if !exists(file) {
		t.Fatal("the file was removed from disk")
	}
	if len(m.removed) != 1 || m.files[indexLoc(t, file)] {
		t.Fatalf("backend removes %v, records %v; want the record removed", m.removed, m.files)
	}
	if len(sink.got) != 0 {
		t.Fatalf("reported %+v; a backend change is not a disk change", sink.got)
	}
}

func TestFileIndexDeleteOfDirectoryRemovesIndexTreeOnly(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "lantern")
	indexed := filepath.Join(dir, "meadow.txt")
	writeTestFile(t, indexed, "clover")
	m := newMemIndex([]string{indexLoc(t, root), indexLoc(t, dir)}, []string{indexLoc(t, indexed)})
	useMemIndex(t, m)
	if _, _, err := ExecuteDeletePaths(context.Background(), []string{indexLoc(t, dir)}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !exists(indexed) {
		t.Fatal("the directory was removed from disk")
	}
	if len(m.treeRemoved) != 1 || m.files[indexLoc(t, indexed)] {
		t.Fatalf("tree removes %v, records %v; want the index tree removed", m.treeRemoved, m.files)
	}
}

func TestFileIndexRenameMovesTheRecordOnly(t *testing.T) {
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
	if !m.files[indexLoc(t, newP)] || m.files[indexLoc(t, oldP)] {
		t.Fatalf("index records %v, want the record renamed", m.files)
	}
	if !exists(oldP) || exists(newP) {
		t.Fatalf("on disk: old %v, new %v; want untouched", exists(oldP), exists(newP))
	}
	if len(sink.got) != 0 {
		t.Fatalf("reported %+v; a backend change is not a disk change", sink.got)
	}
}

func TestFileIndexCleanupSkipsFileIndexPaths(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "orchard")
	emptyKid := filepath.Join(parent, "empty")
	if err := os.MkdirAll(emptyKid, 0o755); err != nil {
		t.Fatal(err)
	}
	gone := pathloc.MustParse(indexLoc(t, filepath.Join(parent, "harbor.txt")))
	// The index lists the parent as empty, so listing-based cleanup would remove it.
	m := newMemIndex([]string{indexLoc(t, root), indexLoc(t, parent)}, nil)
	useMemIndex(t, m)
	if err := RemoveEmptyDirsUnder(context.Background(), []pathloc.Path{gone.Parent()}); err != nil {
		t.Fatal(err)
	}
	dirs, err := DanglingDirsAfter(context.Background(), []pathloc.Path{gone})
	if err != nil || len(dirs) != 0 {
		t.Fatalf("dangling = %v, %v; want none", dirs, err)
	}
	if !exists(emptyKid) {
		t.Fatal("cleanup removed a directory from disk")
	}
	if len(m.removed) != 0 || len(m.treeRemoved) != 0 {
		t.Fatalf("cleanup used the index backend: %v %v", m.removed, m.treeRemoved)
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
	local := pathloc.MustParse("/meadow")
	for name, c := range map[string]struct {
		roots []pathloc.Path
		dest  pathloc.Path
	}{
		"root": {[]pathloc.Path{loc}, local},
		"dest": {[]pathloc.Path{pathloc.MustParse("/orchard")}, loc},
	} {
		err := ValidateFlattenTarget(c.roots, c.dest)
		var opsErr *Error
		if !errors.As(err, &opsErr) || opsErr.Text != FlattenIndexRefusal {
			t.Fatalf("%s: validate = %v, want the fileindex refusal", name, err)
		}
		if _, _, err := ExecuteFlatten(context.Background(), c.roots, c.dest, true, true, Options{}, ProgressEmitThrottle{}, nil, nil, nil); err == nil {
			t.Fatalf("%s: flatten of a fileindex:// path must be refused", name)
		}
	}
	if err := ValidateFlattenTarget([]pathloc.Path{loc}, pathloc.Path{}); err == nil || strings.Contains(err.Error(), "fileindex") {
		t.Fatalf("a zero destination is reported as such, got %v", err)
	}
}

func TestOnDiskPathsTranslatesOnlyFilesThatExist(t *testing.T) {
	root := t.TempDir()
	here, gone := filepath.Join(root, "harbor.txt"), filepath.Join(root, "bramble.txt")
	writeTestFile(t, here, "anchor")
	local := pathloc.MustParse("/meadow/pebble.txt")
	got := OnDiskPaths([]pathloc.Path{pathloc.MustParse(indexLoc(t, here)), pathloc.MustParse(indexLoc(t, gone)), local})
	if got[0].String() != pathloc.MustParse(here).String() {
		t.Fatalf("a file on disk = %q, want its disk path", got[0])
	}
	if got[1].String() != indexLoc(t, gone) {
		t.Fatalf("a file gone from disk = %q, want the fileindex:// path kept", got[1])
	}
	if !got[2].Equal(local) {
		t.Fatalf("a local path = %q, want it unchanged", got[2])
	}
}
