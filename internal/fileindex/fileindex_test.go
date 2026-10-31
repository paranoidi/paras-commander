package fileindex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// testIndex returns an Index on dbPath that reads no user config.ini.
func testIndex(t *testing.T, dbPath string) *Index {
	t.Helper()
	x := New(config.FileIndexConfig{Database: dbPath, Workers: 2})
	x.configPath = filepath.Join(t.TempDir(), "absent-config.ini")
	t.Cleanup(func() { _ = x.Close() })
	return x
}

// initIndex creates an empty schema-current database (the fileindex CLI's `init`, which pc never runs).
func initIndex(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "index.sqlite")
	d, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Init(io.Discard); err != nil {
		t.Fatal(err)
	}
	_ = d.Close()
	return path
}

// tree writes a small tree of random-word files under a resolved temp root.
func tree(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"harbor.txt":         "anchor and rope",
		"lantern/meadow.txt": "clover",
		"lantern/thistle.md": "bramble patch",
	} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

type recReporter struct {
	mu       sync.Mutex
	last     jobs.TaskProgress
	warns    []string
	finished []int
}

func (r *recReporter) Progress(p jobs.TaskProgress) { r.mu.Lock(); r.last = p; r.mu.Unlock() }
func (r *recReporter) Warn(m string)                { r.mu.Lock(); r.warns = append(r.warns, m); r.mu.Unlock() }
func (r *recReporter) FinishSource(i int) {
	r.mu.Lock()
	r.finished = append(r.finished, i)
	r.mu.Unlock()
}

// TestScanTaskFinishesEachPath: every path is reported done once and the counts cover them all.
func TestScanTaskFinishesEachPath(t *testing.T) {
	x := testIndex(t, initIndex(t))
	root := tree(t)
	rep := &recReporter{}
	paths := []string{filepath.Join(root, "harbor.txt"), filepath.Join(root, "lantern")}
	if err := x.ScanTask(paths).Run(context.Background(), rep); err != nil {
		t.Fatal(err)
	}
	slices.Sort(rep.finished)
	if !slices.Equal(rep.finished, []int{0, 1}) {
		t.Fatalf("finished = %v, want [0 1]", rep.finished)
	}
	if rep.last.DoneFiles != 3 || rep.last.TotalFiles != 3 || rep.last.DoneBytes != int64(len("anchor and rope")+len("clover")+len("bramble patch")) {
		t.Fatalf("scan progress = %+v, want 3/3 files and their bytes", rep.last)
	}
}

func scanned(t *testing.T) (*Index, string) {
	t.Helper()
	x := testIndex(t, initIndex(t))
	root := tree(t)
	rep := &recReporter{}
	if err := x.ScanTask([]string{root}).Run(context.Background(), rep); err != nil {
		t.Fatal(err)
	}
	if rep.last.DoneFiles != 3 || rep.last.DoneBytes != int64(len("anchor and rope")+len("clover")+len("bramble patch")) {
		t.Fatalf("scan progress = %+v, want 3 files and their bytes", rep.last)
	}
	return x, root
}

func at(t *testing.T, host string) pathloc.Path {
	t.Helper()
	loc, err := pathloc.FileIndex(host)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func names(entries []fsbackend.Entry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}

func TestOpenFailureKeptUntilFileChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchard.sqlite")
	x := testIndex(t, path)
	_, first := x.DB()
	if !errors.Is(first, ErrNoIndex) {
		t.Fatalf("err = %v, want ErrNoIndex", first)
	}
	if _, again := x.DB(); again != first {
		t.Fatalf("unchanged file reopened: %v", again)
	}
	src := initIndex(t)
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := x.DB(); err != nil {
		t.Fatalf("database created outside pc not picked up: %v", err)
	}
}

func TestMissingIndexReportsOnce(t *testing.T) {
	x := testIndex(t, filepath.Join(t.TempDir(), "nowhere", "index.sqlite"))
	var reports int
	x.OnOpenError = func(error) { reports++ }
	for range 3 {
		if _, err := x.DB(); !errors.Is(err, ErrNoIndex) {
			t.Fatalf("err = %v, want ErrNoIndex", err)
		}
	}
	if reports != 1 {
		t.Fatalf("open error reported %d times, want once", reports)
	}
	if _, err := os.Stat(filepath.Dir(x.cfg.Database)); !os.IsNotExist(err) {
		t.Fatal("opening a missing index must not create its directory")
	}
	if _, err := NewBackend(x).List(context.Background(), pathloc.MustParse("fileindex:///")); !errors.Is(err, ErrNoIndex) {
		t.Fatalf("list without an index = %v", err)
	}
}

func TestBackendListsStatsAndReads(t *testing.T) {
	x, root := scanned(t)
	b := NewBackend(x)
	ctx := context.Background()

	entries, err := b.List(ctx, at(t, root))
	if err != nil {
		t.Fatal(err)
	}
	if got := names(entries); !slices.Equal(got, []string{"lantern", "harbor.txt"}) {
		t.Fatalf("list = %v, want subdirs then files", got)
	}
	if entries[0].Type != fsbackend.EntryDirectory || entries[1].Type != fsbackend.EntryFile || entries[1].Size != int64(len("anchor and rope")) || entries[1].ModifiedAt.IsZero() {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[1].Loc.String() != "fileindex://"+filepath.Join(root, "harbor.txt") {
		t.Fatalf("loc = %s", entries[1].Loc)
	}
	st, err := b.Stat(ctx, at(t, filepath.Join(root, "lantern", "meadow.txt")))
	if err != nil || st.Type != fsbackend.EntryFile || st.Size != int64(len("clover")) {
		t.Fatalf("stat = %+v, %v", st, err)
	}
	if _, err := b.Stat(ctx, at(t, filepath.Join(root, "pebble"))); err == nil {
		t.Fatal("stat of an unindexed path must fail")
	}
	if root, err := b.Stat(ctx, pathloc.MustParse("fileindex:///")); err != nil || root.Type != fsbackend.EntryDirectory {
		t.Fatalf("index root = %+v, %v", root, err)
	}

	f, err := b.OpenRead(ctx, at(t, filepath.Join(root, "harbor.txt")))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(f)
	_ = f.Close()
	if string(body) != "anchor and rope" {
		t.Fatalf("read = %q", body)
	}
	if err := os.Remove(filepath.Join(root, "harbor.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.OpenRead(ctx, at(t, filepath.Join(root, "harbor.txt"))); !errors.Is(err, ErrNotOnDisk) {
		t.Fatalf("read of a file gone from disk = %v, want ErrNotOnDisk", err)
	}
	if entries, _ := b.List(ctx, at(t, root)); !slices.Contains(names(entries), "harbor.txt") {
		t.Fatal("the index still lists a file gone from disk")
	}
}

func TestBackendIsReadOnlyButRenamesAndRemovesInIndex(t *testing.T) {
	x, root := scanned(t)
	b := NewBackend(x)
	ctx := context.Background()
	if err := b.Mkdir(ctx, at(t, filepath.Join(root, "pebble")), 0o755); !errors.Is(err, fsbackend.ErrReadOnly) {
		t.Fatalf("mkdir = %v", err)
	}
	if _, err := b.OpenWrite(ctx, at(t, filepath.Join(root, "pebble.txt")), 1, fsbackend.CreateOpts{}); !errors.Is(err, fsbackend.ErrReadOnly) {
		t.Fatalf("write = %v", err)
	}
	if err := b.Symlink(ctx, at(t, filepath.Join(root, "pebble")), "harbor.txt"); !errors.Is(err, fsbackend.ErrReadOnly) {
		t.Fatalf("symlink = %v", err)
	}

	if err := b.Rename(ctx, at(t, filepath.Join(root, "harbor.txt")), at(t, filepath.Join(root, "lantern", "wharf.txt"))); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "harbor.txt")); err != nil {
		t.Fatal("an index rename must not touch the disk")
	}
	entries, _ := b.List(ctx, at(t, filepath.Join(root, "lantern")))
	if !slices.Contains(names(entries), "wharf.txt") {
		t.Fatalf("lantern = %v, want the moved entry", names(entries))
	}

	if err := b.Remove(ctx, at(t, filepath.Join(root, "lantern", "wharf.txt"))); err != nil {
		t.Fatal(err)
	}
	if err := b.RemoveTree(ctx, at(t, filepath.Join(root, "lantern"))); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "lantern", "meadow.txt")); err != nil {
		t.Fatal("an index remove must not touch the disk")
	}
	if _, err := b.List(ctx, at(t, filepath.Join(root, "lantern"))); err == nil {
		t.Fatal("removed tree still listed")
	}
}

func TestTagsOnFileGoneFromDiskAndColumn(t *testing.T) {
	x, root := scanned(t)
	meadow := filepath.Join(root, "lantern", "meadow.txt")
	if err := os.Remove(meadow); err != nil {
		t.Fatal(err)
	}
	gone := "fileindex://" + meadow
	res, err := x.EditTags([]string{gone, filepath.Join(root, "lantern")}, ParseTags("clover, field  field"), true, false)
	if err != nil || res.Changed != 4 || res.Targeted != 2 {
		t.Fatalf("add = %+v, %v; want 2 tags on the file and the dir", res, err)
	}
	res, err = x.EditTags([]string{filepath.Join(root, "pebble.txt")}, []string{"clover"}, true, false)
	if err != nil || len(res.Unknown) != 1 || res.Targeted != 0 {
		t.Fatalf("unindexed path = %+v, %v", res, err)
	}

	values := func(dir string, es ...localfs.Entry) map[string]string {
		t.Helper()
		v, err := x.TagsProvider().Values(context.Background(), dir, es)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	lanternLocal := localfs.Entry{Name: "lantern", Path: filepath.Join(root, "lantern"), Type: localfs.EntryDirectory}
	if got := values(root, lanternLocal)[lanternLocal.Path]; got != "clover, field" {
		t.Fatalf("dir tags on a local panel = %q", got)
	}
	meadowIdx := localfs.Entry{Name: "meadow.txt", Path: gone, Type: localfs.EntryFile}
	if got := values("fileindex://"+filepath.Join(root, "lantern"), meadowIdx)[gone]; got != "clover, field" {
		t.Fatalf("file tags on an index panel = %q", got)
	}

	res, err = x.EditTags([]string{filepath.Join(root, "lantern")}, []string{"field"}, false, true)
	if err != nil || res.Changed != 2 {
		t.Fatalf("recursive remove = %+v, %v; want the dir's and its file's", res, err)
	}
	if got := values("fileindex://"+filepath.Join(root, "lantern"), meadowIdx)[gone]; got != "clover" {
		t.Fatalf("after remove = %q", got)
	}
	if v := values("sftp://user@host/lantern", meadowIdx); len(v) != 0 {
		t.Fatalf("sftp panel tags = %v, want none", v)
	}
}

func TestScanTaskCancel(t *testing.T) {
	x := testIndex(t, initIndex(t))
	root := tree(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := x.ScanTask([]string{root}).Run(ctx, &recReporter{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled scan = %v, want context.Canceled", err)
	}
}

func TestPruneTaskRemovesFilesGoneFromDisk(t *testing.T) {
	x, root := scanned(t)
	if err := os.Remove(filepath.Join(root, "lantern", "thistle.md")); err != nil {
		t.Fatal(err)
	}
	rep := &recReporter{}
	if err := x.PruneTask(root).Run(context.Background(), rep); err != nil {
		t.Fatal(err)
	}
	entries, _ := NewBackend(x).List(context.Background(), at(t, filepath.Join(root, "lantern")))
	if got := names(entries); !slices.Equal(got, []string{"meadow.txt"}) {
		t.Fatalf("after prune = %v", got)
	}
	if len(rep.warns) != 1 || rep.warns[0] != "Pruning removed file: "+filepath.Join(root, "lantern", "thistle.md") {
		t.Fatalf("prune log = %v", rep.warns)
	}
}

func TestScanTaskThroughJobQueue(t *testing.T) {
	x := testIndex(t, initIndex(t))
	root := tree(t)
	s := jobs.NewState()
	stop := make(chan struct{})
	s.StartWorker(stop)
	defer close(stop)
	job := jobs.NewTaskJob(x.ScanTask([]string{root}), []pathloc.Path{pathloc.MustParse(root)})
	s.AddJob(job)
	for ev := range s.Events() {
		s.ApplyEvent(ev)
		if ev.JobID == job.ID && ev.Status.IsFinished() {
			break
		}
	}
	got := s.Job(job.ID)
	if got.Status != jobs.StatusCompleted || got.DoneFiles != 3 || got.TotalFiles != 3 {
		t.Fatalf("job = %s %d/%d (%s)", got.Status, got.DoneFiles, got.TotalFiles, got.Error)
	}
}

func TestCreateMakesUsableIndexOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orchard", "index.sqlite")
	x := testIndex(t, path)
	if _, err := x.DB(); !errors.Is(err, ErrNoIndex) {
		t.Fatalf("before create = %v", err)
	}
	got, err := x.Create()
	if err != nil || got != path {
		t.Fatalf("create = %q, %v", got, err)
	}
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.RequireSchema(); err != nil {
		t.Fatalf("created schema: %v", err)
	}
	if _, err := x.Create(); !errors.Is(err, ErrIndexExists) {
		t.Fatalf("second create = %v, want ErrIndexExists", err)
	}
}

func TestUpgradeInitialisesExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.sqlite")
	if err := os.WriteFile(path, nil, 0o644); err != nil { // a file `fileindex init` never ran on
		t.Fatal(err)
	}
	x := testIndex(t, path)
	_, err := x.DB()
	if !NeedsUpgrade(err) {
		t.Fatalf("open = %v, want an error NeedsUpgrade accepts", err)
	}
	if _, err := x.Upgrade(); err != nil {
		t.Fatal(err)
	}
	if _, err := x.DB(); err != nil {
		t.Fatalf("after upgrade = %v", err)
	}
	if !NeedsUpgrade(fmt.Errorf("wrapped: %w", db.ErrOutdatedSchema)) || NeedsUpgrade(db.ErrNewerSchema) || NeedsUpgrade(ErrNoIndex) {
		t.Fatal("NeedsUpgrade must accept older or uninitialised schemas only")
	}
	if _, err := testIndex(t, filepath.Join(t.TempDir(), "absent.sqlite")).Upgrade(); !errors.Is(err, ErrNoIndex) {
		t.Fatalf("upgrade of a missing database = %v", err)
	}
}
