package panel

import (
	"context"
	"io"
	"io/fs"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// fakeIndexBackend lists a fixed fileindex:// tree; everything else is read-only.
type fakeIndexBackend struct{ tree map[string][]fsbackend.Entry }

func (b fakeIndexBackend) Scheme() pathloc.Scheme { return pathloc.SchemeFileIndex }
func (b fakeIndexBackend) List(_ context.Context, dir pathloc.Path) ([]fsbackend.Entry, error) {
	if e, ok := b.tree[dir.String()]; ok {
		return e, nil
	}
	return nil, fs.ErrNotExist
}
func (b fakeIndexBackend) Stat(_ context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	if _, ok := b.tree[loc.String()]; ok {
		return fsbackend.Entry{Name: loc.Base(), Loc: loc, Type: fsbackend.EntryDirectory}, nil
	}
	return fsbackend.Entry{}, fs.ErrNotExist
}
func (fakeIndexBackend) OpenRead(context.Context, pathloc.Path) (io.ReadSeekCloser, error) {
	return nil, fs.ErrNotExist
}
func (fakeIndexBackend) OpenWrite(context.Context, pathloc.Path, int64, fsbackend.CreateOpts) (io.WriteCloser, error) {
	return nil, fsbackend.ErrReadOnly
}
func (fakeIndexBackend) Mkdir(context.Context, pathloc.Path, fs.FileMode) error {
	return fsbackend.ErrReadOnly
}
func (fakeIndexBackend) Remove(context.Context, pathloc.Path) error { return fsbackend.ErrReadOnly }
func (fakeIndexBackend) Rename(context.Context, pathloc.Path, pathloc.Path) error {
	return fsbackend.ErrReadOnly
}
func (fakeIndexBackend) ReadSymlink(context.Context, pathloc.Path) (string, error) {
	return "", fsbackend.ErrReadOnly
}
func (fakeIndexBackend) Symlink(context.Context, pathloc.Path, string) error {
	return fsbackend.ErrReadOnly
}

var registerFakeIndex sync.Once

func TestFileIndexSchemeListsThroughBackend(t *testing.T) {
	registerFakeIndex.Do(func() {
		at := func(s string) pathloc.Path { return pathloc.MustParse(s) }
		err := fsbackend.RegisterDefault(fakeIndexBackend{tree: map[string][]fsbackend.Entry{
			"fileindex:///orchard": {
				{Name: "lantern", Loc: at("fileindex:///orchard/lantern"), Type: fsbackend.EntryDirectory},
				{Name: "meadow.txt", Loc: at("fileindex:///orchard/meadow.txt"), Type: fsbackend.EntryFile, Size: 1200, ModifiedAt: time.Unix(1700000000, 0)},
			},
			"fileindex:///orchard/lantern": nil,
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
	st, err := New("fileindex:///orchard")
	if err != nil {
		t.Fatal(err)
	}
	if st.Path.Scheme() != pathloc.SchemeFileIndex {
		t.Fatalf("panel path = %s", st.Path)
	}
	var names []string
	for _, e := range st.Entries {
		names = append(names, e.Name)
	}
	if len(names) != 2 || names[0] != "lantern" || names[1] != "meadow.txt" {
		t.Fatalf("entries = %v", names)
	}
	if st.GitColumnActive || st.VolumeSpaceOK {
		t.Fatal("git and volume probes must stay off for the scheme")
	}
	if got := st.Entries[1].Path; got != "fileindex:///orchard/meadow.txt" {
		t.Fatalf("entry path = %q", got)
	}
}
