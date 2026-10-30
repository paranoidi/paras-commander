package ops

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
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
