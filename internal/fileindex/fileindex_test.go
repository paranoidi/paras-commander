package fileindex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

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

// TestSubtreeSize: a directory totals the indexed files under it, the index root everything, a
// file its own size; a path the index lacks does not exist.
func TestSubtreeSize(t *testing.T) {
	x, root := scanned(t)
	ctx := context.Background()
	lantern := int64(len("clover") + len("bramble patch"))
	all := lantern + int64(len("anchor and rope"))
	for _, c := range []struct {
		host         string
		bytes, files int64
	}{
		{filepath.Join(root, "lantern"), lantern, 2},
		{root, all, 3},
		{"/", all, 3},
		{filepath.Join(root, "harbor.txt"), int64(len("anchor and rope")), 1},
	} {
		bytes, files, err := x.SubtreeSize(ctx, at(t, c.host).String())
		if err != nil || bytes != c.bytes || files != c.files {
			t.Fatalf("SubtreeSize(%s) = %d bytes %d files %v, want %d %d", c.host, bytes, files, err, c.bytes, c.files)
		}
	}
	if _, _, err := x.SubtreeSize(ctx, at(t, filepath.Join(root, "pebble")).String()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing path = %v, want not exist", err)
	}
}

func TestBackendIsReadOnlyButRenamesAndRemovesInIndex(t *testing.T) {
	x, root := scanned(t)
	changes := 0
	x.OnChange = func() { changes++ }
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
	if changes != 3 {
		t.Fatalf("OnChange called %d times, want once per rename, remove and remove tree", changes)
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
		v, err := x.Provider(TagsColumn).Values(context.Background(), dir, es)
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

func TestDedupFilesUsesIndexedHashes(t *testing.T) {
	x := testIndex(t, initIndex(t))
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"willow/reed.txt": "cattail", "marsh/sedge.txt": "cattail", "marsh/rush.txt": "bulrush"} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	files, hashes, err := x.DedupFiles(context.Background(), at(t, root))
	if err != nil || len(files) != 3 || len(hashes) != 3 {
		t.Fatalf("files = %v, hashes = %v, err = %v", files, hashes, err)
	}
	for _, f := range files {
		if f.Abs.String() != "fileindex://"+filepath.Join(root, filepath.FromSlash(f.Rel)) {
			t.Fatalf("abs %s does not match rel %s", f.Abs, f.Rel)
		}
	}
	if hashes["willow/reed.txt"] != hashes["marsh/sedge.txt"] || hashes["marsh/rush.txt"] == hashes["marsh/sedge.txt"] {
		t.Fatalf("hashes = %v", hashes)
	}
	if all, _, err := x.DedupFiles(context.Background(), at(t, "/")); err != nil || len(all) != 3 {
		t.Fatalf("index root = %v, %v", all, err)
	}
	if got, _, err := x.DedupFiles(context.Background(), at(t, filepath.Join(root, "pebble"))); err != nil || len(got) != 0 {
		t.Fatalf("missing dir = %v, %v", got, err)
	}
}

func TestAvailableIsQuiet(t *testing.T) {
	x := testIndex(t, filepath.Join(t.TempDir(), "nowhere", "index.sqlite"))
	x.OnOpenError = func(err error) { t.Fatalf("Available reported %v", err) }
	if x.Available() || x.Provider(TagsColumn).Available() {
		t.Fatal("missing index reported available")
	}
	if !testIndex(t, initIndex(t)).Available() {
		t.Fatal("initialised index reported unavailable")
	}
}

func TestColumns(t *testing.T) {
	x, root := scanned(t)
	lanternDir := filepath.Join(root, "lantern")
	// A second copy of meadow.txt's content, scanned too.
	if err := os.WriteFile(filepath.Join(lanternDir, "clover.txt"), []byte("clover"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	values := func(col, dir string, es ...localfs.Entry) map[string]string {
		t.Helper()
		v, err := x.Provider(col).Values(context.Background(), dir, es)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	file := func(dir, name string) localfs.Entry {
		t.Helper()
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			return localfs.Entry{Name: name, Path: filepath.Join(dir, name), Type: localfs.EntryFile}
		}
		typ := localfs.EntryFile
		if fi.IsDir() {
			typ = localfs.EntryDirectory
		}
		return localfs.Entry{Name: name, Path: filepath.Join(dir, name), Type: typ, Size: fi.Size(), ModifiedAt: fi.ModTime()}
	}

	// Last seen / first seen: today, to the minute, for files and directories alike.
	harbor, lantern := file(root, "harbor.txt"), file(root, "lantern")
	today := time.Now().Format("2006-01-02")
	for _, col := range []string{FirstSeenColumn, LastSeenColumn} {
		v := values(col, root, harbor, lantern)
		for _, e := range []localfs.Entry{harbor, lantern} {
			if !strings.HasPrefix(v[e.Path], today) || len(v[e.Path]) != len("2006-01-02 15:04") {
				t.Fatalf("%s of %s = %q, want today to the minute", col, e.Name, v[e.Path])
			}
		}
	}

	meadow, clover, thistle := file(lanternDir, "meadow.txt"), file(lanternDir, "clover.txt"), file(lanternDir, "thistle.md")
	if v := values(CopiesColumn, lanternDir, meadow, clover, thistle); v[meadow.Path] != "2" || v[clover.Path] != "2" || v[thistle.Path] != "1" {
		t.Fatalf("copies = %v, want meadow and clover 2, thistle 1", v)
	}
	if h := values(HashColumn, lanternDir, thistle)[thistle.Path]; len(h) != hashCells {
		t.Fatalf("hash = %q, want %d hex digits", h, hashCells)
	}
	if v := values(TreeFilesColumn, root, lantern, harbor); v[lantern.Path] != "3" || v[harbor.Path] != "" {
		t.Fatalf("tree files = %v, want 3 for lantern and none for a file", v)
	}

	// Status on a local panel: from the listing's size and mtime.
	if err := os.WriteFile(harbor.Path, []byte("anchor, rope and chain"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pebble.txt"), []byte("gravel"), 0o644); err != nil {
		t.Fatal(err)
	}
	harbor, pebble := file(root, "harbor.txt"), file(root, "pebble.txt")
	want := map[string]string{harbor.Path: StatusChanged, lantern.Path: StatusIndexed, pebble.Path: StatusNew}
	if v := values(StatusColumn, root, harbor, lantern, pebble); !maps.Equal(v, want) {
		t.Fatalf("local status = %v, want %v", v, want)
	}
	// On an index panel: stat'ed, and gone from disk is missing.
	if err := os.Remove(meadow.Path); err != nil {
		t.Fatal(err)
	}
	idx := "fileindex://" + lanternDir
	meadowIdx := localfs.Entry{Name: "meadow.txt", Path: idx + "/meadow.txt", Type: localfs.EntryFile}
	thistleIdx := localfs.Entry{Name: "thistle.md", Path: idx + "/thistle.md", Type: localfs.EntryFile}
	want = map[string]string{meadowIdx.Path: StatusMissing, thistleIdx.Path: StatusIndexed}
	if v := values(StatusColumn, idx, meadowIdx, thistleIdx); !maps.Equal(v, want) {
		t.Fatalf("index status = %v, want %v", v, want)
	}

	if got := formatTimestamp("not a time"); got != "not a time" {
		t.Fatalf("unparsed timestamp = %q", got)
	}
}

func TestFind(t *testing.T) {
	x, root := scanned(t)
	rels := func(f []db.Found) []string {
		var out []string
		for _, e := range f {
			rel, _ := filepath.Rel(root, e.Path)
			out = append(out, rel)
		}
		return out
	}
	got, trunc, err := x.Find(root, FindQuery{Text: "ANT"}, 0)
	if err != nil || trunc || !slices.Equal(rels(got), []string{"lantern"}) {
		t.Fatalf("substring = %v trunc=%v err=%v, want the lantern dir", rels(got), trunc, err)
	}
	got, _, _ = x.Find(root, FindQuery{Text: "*.txt"}, 0)
	if want := []string{"harbor.txt", "lantern/meadow.txt"}; !slices.Equal(rels(got), want) {
		t.Fatalf("glob = %v, want %v", rels(got), want)
	}
	got, _, _ = x.Find(root, FindQuery{Text: "t", OnlyDirs: true}, 0)
	if !slices.Equal(rels(got), []string{"lantern"}) {
		t.Fatalf("dirs only = %v", rels(got))
	}
	got, _, _ = x.Find(root, FindQuery{Text: "lantern", OnlyFiles: true}, 0)
	if len(got) != 0 {
		t.Fatalf("files only matched a dir: %v", rels(got))
	}
	got, _, err = x.Find(root, FindQuery{Text: "thistle", Boolean: true}, 0)
	if err != nil || !slices.Equal(rels(got), []string{"lantern/thistle.md"}) {
		t.Fatalf("boolean = %v err=%v", rels(got), err)
	}
	got, trunc, _ = x.Find(root, FindQuery{Text: "*.*"}, 2)
	if len(got) != 2 || !trunc {
		t.Fatalf("cap = %d trunc=%v, want 2 and truncated", len(got), trunc)
	}
	if _, _, err = x.Find(root, FindQuery{Text: `"unbalanced`, Boolean: true}, 0); err == nil {
		t.Fatal("an FTS syntax error must be returned")
	}
}

func TestFindDepthHiddenAndRoots(t *testing.T) {
	x := testIndex(t, initIndex(t))
	root := tree(t)
	for name, body := range map[string]string{".hollow/ghost.txt": "g", "lantern/deep/leaf.txt": "l", ".quiet.txt": "q"} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	count := func(q FindQuery) int {
		got, _, err := x.Find(root, q, 0)
		if err != nil {
			t.Fatal(err)
		}
		return len(got)
	}
	if n := count(FindQuery{Text: "*.txt"}); n != 3 { // harbor, meadow, leaf: hidden ones are left out
		t.Fatalf("visible = %d, want 3", n)
	}
	if n := count(FindQuery{Text: "*.txt", IncludeHidden: true}); n != 5 {
		t.Fatalf("with hidden = %d, want 5", n)
	}
	if n := count(FindQuery{Text: "*.txt", MaxDepth: 1}); n != 1 { // harbor.txt only
		t.Fatalf("depth 1 = %d, want 1", n)
	}
	lantern := filepath.Join(root, "lantern")
	both := FindQuery{Text: "*.txt", Roots: []string{lantern, filepath.Join(lantern, "deep")}}
	if n := count(both); n != 2 { // meadow and leaf, the nested root's leaf not repeated
		t.Fatalf("roots = %d, want 2", n)
	}
}
