package fileindex

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
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
	"github.com/paranoidi/fileindex/scan"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// testIndex returns an Index on dbPath that reads no user config.ini.
func testIndex(t *testing.T, dbPath string) *Index {
	t.Helper()
	x := New(config.FileIndexConfig{Database: dbPath, Workers: 2, Hash: true})
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
	if err := d.Init(); err != nil {
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
	x.OnChange = func([]string) { changes++ }
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

// TestScanTagTask: paths the index lacks are recorded and tagged by the scan; one the scan rules
// skip gets a warning instead.
func TestScanTagTask(t *testing.T) {
	x := testIndex(t, initIndex(t))
	root := tree(t)
	harbor, lantern := filepath.Join(root, "harbor.txt"), filepath.Join(root, "lantern")
	// A path the scan cannot record: the index database itself is never indexed.
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	own := d.Path

	res, err := x.EditTags([]string{harbor}, []string{"clover"}, true, false)
	if err != nil || !slices.Equal(res.Unknown, []string{harbor}) {
		t.Fatalf("before the scan = %+v, %v", res, err)
	}
	rep := &recReporter{}
	if err := x.ScanTagTask([]string{harbor, lantern, own}, []string{"clover"}, true).Run(context.Background(), rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.warns) != 1 || !strings.Contains(rep.warns[0], own) {
		t.Fatalf("warnings = %v, want one for the unindexable path", rep.warns)
	}
	entries := []localfs.Entry{
		{Name: "harbor.txt", Path: harbor, Type: localfs.EntryFile},
		{Name: "lantern", Path: lantern, Type: localfs.EntryDirectory},
	}
	v, err := x.Provider(TagsColumn).Values(context.Background(), root, entries)
	if err != nil || v[harbor] != "clover" || v[lantern] != "clover" {
		t.Fatalf("tags = %v, %v", v, err)
	}
	meadow := filepath.Join(lantern, "meadow.txt")
	v, err = x.Provider(TagsColumn).Values(context.Background(), lantern, []localfs.Entry{{Name: "meadow.txt", Path: meadow, Type: localfs.EntryFile}})
	if err != nil || v[meadow] != "clover" {
		t.Fatalf("recursive tags = %v, %v", v, err)
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

// A fast scan hashes only the sample of a larger file, even with hash off, and a plain scan
// finishes it.
func TestScanFastTaskHashesSampleOfLargerFiles(t *testing.T) {
	root := t.TempDir()
	for name, size := range map[string]int{"lantern.bin": 30000, "pebble.bin": 100} {
		if err := os.WriteFile(filepath.Join(root, name), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	x := New(config.FileIndexConfig{Database: initIndex(t), Workers: 2})
	x.configPath = filepath.Join(t.TempDir(), "absent-config.ini")
	t.Cleanup(func() { _ = x.Close() })
	if err := x.ScanFastTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	hashed := func(name string) (full, head bool) {
		t.Helper()
		if err := d.QueryRow("SELECT file_id IS NOT NULL, sha256_sample IS NOT NULL FROM instance WHERE filename = ?", name).Scan(&full, &head); err != nil {
			t.Fatal(err)
		}
		return full, head
	}
	if full, head := hashed("lantern.bin"); full || !head {
		t.Fatalf("large file after fast scan: full %v, head %v", full, head)
	}
	if full, _ := hashed("pebble.bin"); !full {
		t.Fatal("small file after fast scan is not fully hashed")
	}
	x.cfg.Hash = true
	if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	if full, _ := hashed("lantern.bin"); !full {
		t.Fatal("large file after plain scan is not fully hashed")
	}
}

// A fast-hashed file groups with a file of the same size and sample; a same-size file with
// another head stays out, and a plain scan settles the set.
func TestDedupFilesPotentialDuplicates(t *testing.T) {
	root := t.TempDir()
	write := func(name string, head byte) {
		t.Helper()
		b := make([]byte, 30000)
		b[0] = head
		if err := os.WriteFile(filepath.Join(root, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("anchor.bin", 'a')
	write("meadow.bin", 'b')
	x := testIndex(t, initIndex(t))
	if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	write("willow.bin", 'a')
	write("thistle.bin", 'c')
	if err := x.ScanFastTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	files, hashes, err := x.DedupFiles(context.Background(), at(t, root))
	if err != nil {
		t.Fatal(err)
	}
	potential := map[string]bool{}
	for _, f := range files {
		if f.Potential != ([32]byte{}) {
			potential[f.Rel] = true
		}
	}
	if len(potential) != 2 || !potential["anchor.bin"] || !potential["willow.bin"] {
		t.Fatalf("potential = %v, want anchor.bin and willow.bin", potential)
	}
	if _, ok := hashes["willow.bin"]; ok {
		t.Fatal("a fast-hashed file must have no content hash")
	}
	if _, ok := hashes["anchor.bin"]; !ok {
		t.Fatal("a fully hashed member keeps its content hash")
	}
	if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	if files, _, err = x.DedupFiles(context.Background(), at(t, root)); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Potential != ([32]byte{}) {
			t.Fatalf("%s still potential after a full scan", f.Rel)
		}
	}
}

// Only the given fast-hashed files are hashed; done runs and the match is settled.
func TestHashPotentialTask(t *testing.T) {
	root := t.TempDir()
	write := func(name string, head, tail byte) {
		t.Helper()
		b := make([]byte, 30000)
		b[0], b[10000] = head, tail // 10000 lies between the sampled head and middle
		if err := os.WriteFile(filepath.Join(root, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("anchor.bin", 'a', 1)
	write("meadow.bin", 'a', 1)
	write("willow.bin", 'a', 2)
	write("clover.bin", 'c', 1)
	write("thistle.bin", 'c', 1)
	x := testIndex(t, initIndex(t))
	if err := x.ScanFastTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, n := range []string{"anchor.bin", "meadow.bin", "willow.bin"} {
		paths = append(paths, filepath.Join(root, n))
	}
	done := 0
	rep := &recReporter{}
	if err := x.HashPotentialTask(paths, func() { done++ }).Run(context.Background(), rep); err != nil {
		t.Fatal(err)
	}
	if done != 1 || rep.last.DoneFiles != 3 || rep.last.TotalFiles != 3 {
		t.Fatalf("done = %d, progress = %+v", done, rep.last)
	}
	files, hashes, err := x.DedupFiles(context.Background(), at(t, root))
	if err != nil {
		t.Fatal(err)
	}
	if hashes["anchor.bin"] != hashes["meadow.bin"] || hashes["anchor.bin"] == hashes["willow.bin"] || len(hashes) != 3 {
		t.Fatalf("hashes = %v", hashes)
	}
	potential := map[string]bool{}
	for _, f := range files {
		if f.Potential != ([32]byte{}) {
			potential[f.Rel] = true
		}
	}
	if len(potential) != 2 || !potential["clover.bin"] || !potential["thistle.bin"] {
		t.Fatalf("potential = %v, want the files not asked for", potential)
	}
}

func TestNewerSchemaAsksToUpgradePC(t *testing.T) {
	path := initIndex(t)
	d, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("INSERT INTO schema_version (version) VALUES (?)", db.SchemaVersion+1); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = testIndex(t, path).DB()
	if !errors.Is(err, ErrNewerSchema) || NeedsUpgrade(err) {
		t.Fatalf("open = %v, want ErrNewerSchema", err)
	}
	if !strings.Contains(err.Error(), "upgrade paras-commander") || strings.Contains(err.Error(), "upgrade fileindex") {
		t.Fatalf("message = %q", err)
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

// TestCloseIsFinal: after Close the index does not reopen the database, so goroutines still
// asking for it cannot recreate files under a directory being removed.
func TestCloseIsFinal(t *testing.T) {
	x := testIndex(t, initIndex(t))
	x.OnOpenError = func(err error) { t.Fatalf("closing reported %v", err) }
	if _, err := x.DB(); err != nil {
		t.Fatal(err)
	}
	if err := x.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := x.DB(); !errors.Is(err, ErrClosed) {
		t.Fatalf("DB after Close = %v, want ErrClosed", err)
	}
	if x.Available() {
		t.Fatal("closed index reported available")
	}
	if _, err := x.Upgrade(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Upgrade after Close = %v, want ErrClosed", err)
	}
}

// TestScanReporterWorkers: Speed carries the hash worker count into the task progress until
// hashing is done.
func TestScanReporterWorkers(t *testing.T) {
	rep := &recReporter{}
	s := newScanReporter(rep, false)
	s.Speed(3, 0)
	if rep.last.Workers != 3 {
		t.Fatalf("Workers after Speed = %d, want 3", rep.last.Workers)
	}
	s.Hashed(scan.Stats{})
	if rep.last.Workers != 0 {
		t.Fatalf("Workers after Hashed = %d, want 0", rep.last.Workers)
	}
}

// TestScanReporterFileDone: starting a file does not count it, FileDone does.
func TestScanReporterFileDone(t *testing.T) {
	rep := &recReporter{}
	s := newScanReporter(rep, false)
	s.Pruned(scan.Stats{}, 3)
	s.Worker(0, "/vol/orchard/lantern")
	s.Worker(0, "/vol/orchard/meadow")
	if rep.last.DoneFiles != 0 {
		t.Fatalf("after Worker = %d done, want 0", rep.last.DoneFiles)
	}
	s.FileDone()
	s.FileDone()
	if rep.last.DoneFiles != 2 {
		t.Fatalf("after FileDone = %d done, want 2", rep.last.DoneFiles)
	}
}

// TestScanReporterStages: the walk counts committed files with no total, pruning keeps the total
// unknown, hashing restarts the count against the files to hash, and a scan that stops before
// hashing keeps the walk's count.
func TestScanReporterStages(t *testing.T) {
	rep := &recReporter{}
	s := newScanReporter(rep, false)
	s.Scan("/vol/orchard", 4, 4, 0)
	s.Indexed(4)
	if p := rep.last; p.DoneFiles != 4 || p.TotalFiles != 0 || p.CurrentPath != "/vol/orchard" {
		t.Fatalf("walking = %+v, want 4 done, unknown total", p)
	}
	if rep.last.Step != 0 {
		t.Fatalf("walking Step = %d, want 0", rep.last.Step)
	}
	s.Discovered(scan.Stats{Discovered: 6})
	if rep.last.Step != 1 {
		t.Fatalf("pruning Step = %d, want 1", rep.last.Step)
	}
	s.Prune("/vol/orchard/pebble", 2)
	if p := rep.last; p.DoneFiles != 6 || p.TotalFiles != 0 || p.CurrentPath != "/vol/orchard/pebble" {
		t.Fatalf("pruning = %+v, want 6 done, unknown total, current dir", p)
	}
	s.Pruned(scan.Stats{}, 9)
	if rep.last.Step != 2 {
		t.Fatalf("hashing Step = %d, want 2", rep.last.Step)
	}
	if p := rep.last; p.DoneFiles != 0 || p.TotalFiles != 9 {
		t.Fatalf("hashing start = %+v, want 0 of 9", p)
	}
	s.Hashed(scan.Stats{Processed: 9, TotalSize: 40})
	if p := rep.last; p.DoneFiles != 9 || p.TotalFiles != 9 || p.DoneBytes != 40 {
		t.Fatalf("hashed = %+v, want 9 of 9 and 40 bytes", p)
	}

	rep = &recReporter{}
	s = newScanReporter(rep, true)
	s.Indexed(5)
	s.Discovered(scan.Stats{Discovered: 5})
	s.Pruned(scan.Stats{}, 5)
	s.Hashed(scan.Stats{})
	if p := rep.last; p.DoneFiles != 5 || p.TotalFiles != 0 {
		t.Fatalf("skip hash = %+v, want the walk's 5 and no total", p)
	}
}

// TestUnhashedFiles: with [fileindex].hash off a scan leaves files findable but unhashed; they
// count their bytes in subtree sizes, show the unhashed status (changed wins), have no hash or
// copies cell and are left out of duplicate detection. A hashing scan turns them indexed.
func TestUnhashedFiles(t *testing.T) {
	path := initIndex(t)
	x := New(config.FileIndexConfig{Database: path, Workers: 2, Hash: false})
	x.configPath = filepath.Join(t.TempDir(), "absent-config.ini")
	t.Cleanup(func() { _ = x.Close() })
	root := tree(t)
	rep := &recReporter{}
	if err := x.ScanTask([]string{root}).Run(context.Background(), rep); err != nil {
		t.Fatal(err)
	}
	if rep.last.DoneFiles != 3 || rep.last.TotalFiles != 0 {
		t.Fatalf("progress = %+v, want 3 files walked and no total", rep.last)
	}
	ctx := context.Background()
	want := int64(len("anchor and rope") + len("clover") + len("bramble patch"))
	if b, n, err := x.SubtreeSize(ctx, at(t, root).String()); err != nil || b != want || n != 3 {
		t.Fatalf("SubtreeSize = %d bytes %d files %v, want %d 3", b, n, err, want)
	}
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	entries := func() []localfs.Entry {
		var es []localfs.Entry
		for _, n := range []string{"harbor.txt"} {
			fi, err := os.Stat(filepath.Join(root, n))
			if err != nil {
				t.Fatal(err)
			}
			es = append(es, localfs.Entry{Name: n, Path: filepath.Join(root, n), Type: localfs.EntryFile, Size: fi.Size(), ModifiedAt: fi.ModTime()})
		}
		return es
	}
	status := func() string {
		t.Helper()
		st, err := entryStatuses(ctx, d, root, entries(), true, scan.Rules{})
		if err != nil {
			t.Fatal(err)
		}
		return st["harbor.txt"]
	}
	if got := status(); got != StatusUnhashed {
		t.Fatalf("status = %q, want unhashed", got)
	}
	if v, err := x.Provider(HashColumn).Values(ctx, root, entries()); err != nil || len(v) != 0 {
		t.Fatalf("hash column = %v, %v, want empty", v, err)
	}
	if files, hashes, err := x.DedupFiles(ctx, at(t, root)); err != nil || len(files) != 0 || len(hashes) != 0 {
		t.Fatalf("DedupFiles = %v %v %v, want none", files, hashes, err)
	}
	if err := os.WriteFile(filepath.Join(root, "harbor.txt"), []byte("anchor, rope and chain"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := status(); got != StatusChanged {
		t.Fatalf("status after edit = %q, want changed", got)
	}

	hashing := testIndex(t, path)
	if err := hashing.ScanTask([]string{root}).Run(ctx, &recReporter{}); err != nil {
		t.Fatal(err)
	}
	if got := status(); got != StatusIndexed {
		t.Fatalf("status after hashing = %q, want indexed", got)
	}
}

// jpegWithExif is a small JPEG with an APP1 segment holding exif spliced in after SOI.
func jpegWithExif(t *testing.T, exif string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	n := len(exif) + 2
	seg := append([]byte{0xFF, 0xE1, byte(n >> 8), byte(n)}, exif...)
	return append(append(append([]byte{}, b[:2]...), seg...), b[2:]...)
}

func TestOtherCopies(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"harbor/meadow.txt":   []byte("clover and thistle"),
		"orchard/clover.txt":  []byte("clover and thistle"),
		"orchard/bramble.txt": []byte("a lone patch"),
		"harbor/lantern.jpg":  jpegWithExif(t, "camera A"),
		"orchard/pebble.jpg":  jpegWithExif(t, "camera B, longer"),
	}
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	at := func(name string) string { return filepath.Join(root, name) }
	x := testIndex(t, initIndex(t))
	for _, dir := range []string{"harbor", "orchard"} {
		if err := x.ScanTask([]string{at(dir)}).Run(context.Background(), &recReporter{}); err != nil {
			t.Fatal(err)
		}
	}
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	// A file the index knows without a full hash, as after a fast scan or an unfinished scan.
	if _, err := d.Exec("UPDATE instance SET file_id = NULL WHERE filename = ?", "bramble.txt"); err != nil {
		t.Fatal(err)
	}

	check := func(name string, wantState CopiesState, want ...string) {
		t.Helper()
		got, state, err := x.OtherCopies(at(name))
		if err != nil || state != wantState {
			t.Fatalf("OtherCopies(%s) = %v %v %v, want state %v", name, got, state, err, wantState)
		}
		for i := range want {
			want[i] = at(want[i])
		}
		if !slices.Equal(got, want) {
			t.Fatalf("OtherCopies(%s) = %v, want %v", name, got, want)
		}
	}
	check("harbor/meadow.txt", CopiesHashed, "orchard/clover.txt")
	check("orchard/clover.txt", CopiesHashed, "harbor/meadow.txt")
	check("harbor/lantern.jpg", CopiesHashed, "orchard/pebble.jpg") // differs only in metadata
	check("orchard/bramble.txt", CopiesNotHashed)
	check("orchard/willow.txt", CopiesNotIndexed)
	// Unique content: indexed and hashed, no other copy.
	if err := os.WriteFile(at("harbor/reed.txt"), []byte("only one of me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := x.ScanTask([]string{at("harbor")}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	check("harbor/reed.txt", CopiesHashed)
	// Empty files are not copies of each other, hashed or not.
	for _, name := range []string{"harbor/tundra.txt", "orchard/walnut.txt", "orchard/sparrow.txt"} {
		if err := os.WriteFile(at(name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{"harbor", "orchard"} {
		if err := x.ScanTask([]string{at(dir)}).Run(context.Background(), &recReporter{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.Exec("UPDATE instance SET file_id = NULL WHERE filename = ?", "sparrow.txt"); err != nil {
		t.Fatal(err)
	}
	check("harbor/tundra.txt", CopiesEmpty)
	check("orchard/sparrow.txt", CopiesEmpty)
}

// DedupOutside keeps only groups spanning the directory boundary, with all their members.
func TestDedupOutside(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"willow/reed.txt":     "cattail",  // spans: copy outside
		"marsh/sedge.txt":     "cattail",  //
		"willow/rush.txt":     "bulrush",  // pair entirely inside
		"willow/bog/rush.txt": "bulrush",  //
		"marsh/fern.txt":      "bracken",  // pair entirely outside
		"heath/fern.txt":      "bracken",  //
		"willow/moss.txt":     "sphagnum", // unique
	} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	x := testIndex(t, initIndex(t))
	if err := x.ScanTask([]string{filepath.Join(root, "willow"), filepath.Join(root, "marsh"), filepath.Join(root, "heath")}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	// Added after the full scan, so this pair spanning the boundary is only fast-hashed.
	for _, name := range []string{"willow/fast.bin", "marsh/fast.bin"} {
		b := make([]byte, 30000)
		b[0] = 'z'
		if err := os.WriteFile(filepath.Join(root, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := x.ScanFastTask([]string{filepath.Join(root, "willow"), filepath.Join(root, "marsh")}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}

	files, hashes, err := x.DedupOutside(context.Background(), filepath.Join(root, "willow"))
	if err != nil {
		t.Fatal(err)
	}
	rel := func(p string) string { return strings.TrimPrefix(filepath.Join(root, p), "/") }
	got := map[string]bool{}
	for _, f := range files {
		got[f.Rel] = true
		if f.Abs.String() != "fileindex:///"+f.Rel {
			t.Fatalf("abs %s does not match rel %s", f.Abs, f.Rel)
		}
	}
	for _, want := range []string{"willow/reed.txt", "marsh/sedge.txt", "willow/fast.bin", "marsh/fast.bin"} {
		if !got[rel(want)] {
			t.Errorf("missing %s in %v", want, got)
		}
	}
	for _, name := range []string{"willow/rush.txt", "willow/bog/rush.txt", "marsh/fern.txt", "heath/fern.txt", "willow/moss.txt"} {
		if got[rel(name)] {
			t.Errorf("%s must not be listed", name)
		}
	}
	if hashes[rel("willow/reed.txt")] != hashes[rel("marsh/sedge.txt")] || hashes[rel("willow/reed.txt")] == ([32]byte{}) {
		t.Fatalf("confirmed pair needs one shared key: %v", hashes)
	}
	for _, f := range files {
		_, hashed := hashes[f.Rel]
		isPotential := f.Potential != ([32]byte{})
		if strings.HasSuffix(f.Rel, "fast.bin") != isPotential || hashed == isPotential {
			t.Errorf("%s: potential=%v hashed=%v", f.Rel, isPotential, hashed)
		}
	}
	if none, _, err := x.DedupOutside(context.Background(), filepath.Join(root, "absent")); err != nil || len(none) != 0 {
		t.Fatalf("missing dir = %v, %v", none, err)
	}
	if none, _, err := x.DedupOutside(context.Background(), "/"); err != nil || len(none) != 0 {
		t.Fatalf("index root = %v, %v", none, err)
	}
}

// A local panel inside a symlinked directory finds the indexed duplicates, which are recorded
// under the real path.
func TestDedupFilesThroughSymlinkedDirectory(t *testing.T) {
	real := t.TempDir()
	for _, n := range []string{"anchor.bin", "meadow.bin"} {
		if err := os.WriteFile(filepath.Join(real, n), []byte("lantern"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(t.TempDir(), "harbor")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	x := testIndex(t, initIndex(t))
	if err := x.ScanTask([]string{real}).Run(context.Background(), &recReporter{}); err != nil {
		t.Fatal(err)
	}
	files, hashes, err := x.DedupFiles(context.Background(), at(t, link))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || len(hashes) != 2 || hashes["anchor.bin"] != hashes["meadow.bin"] {
		t.Fatalf("files = %v, hashes = %v, want both copies through the symlink", files, hashes)
	}
	for _, f := range files {
		if f.Rel != "anchor.bin" && f.Rel != "meadow.bin" {
			t.Fatalf("Rel = %q, want relative to the resolved directory", f.Rel)
		}
	}
}

func TestTagStateCountsTargetsAcrossParents(t *testing.T) {
	x := testIndex(t, initIndex(t))
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	left, right := filepath.Join(root, "orchard"), filepath.Join(root, "pasture")
	var batch []db.ScannedFile
	for i, dir := range []string{left, right} {
		sum := strings.Repeat(string(rune('a'+i)), 32)
		batch = append(batch, db.ScannedFile{Dir: dir, Name: "lantern.txt", Size: 7, Mtime: time.Now(), MD5: sum, SHA256: sum + sum})
	}
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CommitScanBatch(batch); err != nil {
		t.Fatal(err)
	}
	a, b := filepath.Join(left, "lantern.txt"), filepath.Join(right, "lantern.txt")
	for _, edit := range []struct {
		paths []string
		tag   string
	}{{[]string{a, b}, "cobalt"}, {[]string{a}, "sienna"}, {[]string{left}, "umber"}} {
		if _, err := x.EditTags(edit.paths, []string{edit.tag}, true, false); err != nil {
			t.Fatal(err)
		}
	}

	st, err := x.TagState([]string{a, b, a, filepath.Join(right, "absent.txt")})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(st.Names, []string{"cobalt", "sienna", "umber"}) || st.Targets != 3 || st.HasDir {
		t.Fatalf("state = %+v", st)
	}
	if st.Counts["cobalt"] != 2 || st.Counts["sienna"] != 1 || st.Counts["umber"] != 0 {
		t.Fatalf("counts = %v", st.Counts)
	}
	// A panel directory reads its own tags through its parent.
	if st, err = x.TagState([]string{left}); err != nil || st.Counts["umber"] != 1 || st.Targets != 1 {
		t.Fatalf("directory state = %+v, %v", st, err)
	}
	// One directory among files is enough: one the index has, gone from disk, or one only on disk.
	if st, err = x.TagState([]string{a, left}); err != nil || !st.HasDir {
		t.Fatalf("indexed directory state = %+v, %v", st, err)
	}
	unindexed := filepath.Join(root, "thicket")
	if err := os.Mkdir(unindexed, 0o755); err != nil {
		t.Fatal(err)
	}
	if st, err = x.TagState([]string{a, unindexed}); err != nil || !st.HasDir {
		t.Fatalf("directory on disk state = %+v, %v", st, err)
	}
}

// TestCopiesColumnEmptyFiles: empty files show no count however many there are, hashed or not,
// and do not change how other files count.
func TestCopiesColumnEmptyFiles(t *testing.T) {
	for _, hash := range []bool{true, false} {
		x := New(config.FileIndexConfig{Database: initIndex(t), Workers: 2, Hash: hash})
		x.configPath = filepath.Join(t.TempDir(), "absent-config.ini")
		t.Cleanup(func() { _ = x.Close() })
		root := t.TempDir()
		contents := map[string]string{"tundra.txt": "", "walnut.txt": "", "sparrow.txt": "quill"}
		var entries []localfs.Entry
		for name, content := range contents {
			path := filepath.Join(root, name)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			entries = append(entries, localfs.Entry{Name: name, Path: path, Type: localfs.EntryFile, Size: int64(len(content))})
		}
		if err := x.ScanTask([]string{root}).Run(context.Background(), &recReporter{}); err != nil {
			t.Fatal(err)
		}
		v, err := x.Provider(CopiesColumn).Values(context.Background(), root, entries)
		if err != nil {
			t.Fatal(err)
		}
		wantSparrow := "1"
		if !hash {
			wantSparrow = ""
		}
		if v[filepath.Join(root, "tundra.txt")] != panel.MetaNoValue || v[filepath.Join(root, "walnut.txt")] != panel.MetaNoValue || v[filepath.Join(root, "sparrow.txt")] != wantSparrow {
			t.Fatalf("hash=%v: copies = %v, want %q for empty files and %q for sparrow", hash, v, panel.MetaNoValue, wantSparrow)
		}
	}
}

func TestCopiesColumnAlerts(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]panel.MetaAlertLevel{
		panel.MetaNoValue: panel.MetaAlertNone,
		"1":               panel.MetaAlertNone,
		"2":               panel.MetaAlertWarn,
		"3":               panel.MetaAlertCritical,
		"9":               panel.MetaAlertCritical,
	} {
		if got := copiesAlerts().Level(value); got != want {
			t.Errorf("Level(%q) = %d, want %d", value, got, want)
		}
	}
}
