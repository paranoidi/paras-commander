package fileindex

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// addFile records a hashed instance of content at the absolute path (no file on disk).
func addFile(t testing.TB, d *db.DB, path, content string) {
	t.Helper()
	sum, sha := md5.Sum([]byte(content)), sha256.Sum256([]byte(content))
	if _, err := d.Exec(`INSERT OR IGNORE INTO file (md5, sha256, filesize) VALUES (?, ?, ?)`,
		hex.EncodeToString(sum[:]), hex.EncodeToString(sha[:]), len(content)); err != nil {
		t.Fatal(err)
	}
	var fileID int64
	if err := d.QueryRow(`SELECT id FROM file WHERE md5 = ?`, hex.EncodeToString(sum[:])).Scan(&fileID); err != nil {
		t.Fatal(err)
	}
	insertInstance(t, d, path, sql.NullInt64{Int64: fileID, Valid: true}, len(content))
}

// addUnhashed records an instance with no content yet: its tags are pending.
func addUnhashed(t testing.TB, d *db.DB, path string, size int) {
	t.Helper()
	insertInstance(t, d, path, sql.NullInt64{}, size)
}

func insertInstance(t testing.TB, d *db.DB, path string, fileID sql.NullInt64, size int) {
	t.Helper()
	dir, err := db.GetOrCreatePath(d, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`INSERT INTO instance (file_id, dir_id, filename, filename_lower, size, mtime) VALUES (?, ?, ?, ?, ?, '2024-01-02 03:04:05')`,
		fileID, *dir, filepath.Base(path), db.Lower(filepath.Base(path)), size); err != nil {
		t.Fatal(err)
	}
}

func tag(t testing.TB, x *Index, path string, tags ...string) {
	t.Helper()
	res, err := x.EditTags([]string{"fileindex://" + path}, tags, true, false)
	if err != nil || res.Targeted != 1 {
		t.Fatalf("tag %s: %+v, %v", path, res, err)
	}
}

func query(t testing.TB, flat bool, expr string, tags ...string) pathloc.TagQuery {
	t.Helper()
	q := pathloc.TagQuery{Flat: flat, Any: expr == "any", Tags: tags}
	view, err := pathloc.TagView(q, "/")
	if err != nil {
		t.Fatal(err)
	}
	q, _ = view.TagQuery()
	return q
}

func viewAt(t testing.TB, q pathloc.TagQuery, host string) pathloc.Path {
	t.Helper()
	p, err := pathloc.TagView(q, host)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func listNames(t *testing.T, b *Backend, loc pathloc.Path) []string {
	t.Helper()
	entries, err := b.List(context.Background(), loc)
	if err != nil {
		t.Fatalf("list %s: %v", loc, err)
	}
	return names(entries)
}

// holiday builds a small index: orchard holds a beach file, a beach+holiday file and an untagged one;
// cedar a holiday file and a copy of the beach file's content.
func holiday(t *testing.T) (*Index, *Backend, *db.DB) {
	t.Helper()
	x := testIndex(t, initIndex(t))
	d, err := x.DB()
	if err != nil {
		t.Fatal(err)
	}
	addFile(t, d, "/orchard/anchor.txt", "anchor")
	addFile(t, d, "/orchard/rope.txt", "rope")
	addFile(t, d, "/orchard/plain.txt", "plain")
	addFile(t, d, "/cedar/lantern/sail.txt", "sail")
	addFile(t, d, "/cedar/anchor-copy.txt", "anchor")
	tag(t, x, "/orchard/anchor.txt", "beach")
	tag(t, x, "/orchard/rope.txt", "beach", "holiday")
	tag(t, x, "/cedar/lantern/sail.txt", "holiday")
	return x, NewBackend(x), d
}

func TestTagViewAllAndAny(t *testing.T) {
	_, b, _ := holiday(t)
	flat := func(expr string, tags ...string) []string {
		return listNames(t, b, viewAt(t, query(t, true, expr, tags...), "/"))
	}
	// A copy of tagged content carries the tag too (tags live on the content).
	if got := flat("any", "beach"); !slices.Equal(got, []string{"anchor-copy.txt", "anchor.txt", "rope.txt"}) {
		t.Fatalf("beach = %v", got)
	}
	if got := flat("all", "beach", "holiday"); !slices.Equal(got, []string{"rope.txt"}) {
		t.Fatalf("beach&holiday = %v", got)
	}
	if got := flat("any", "beach", "holiday"); !slices.Equal(got, []string{"anchor-copy.txt", "sail.txt", "anchor.txt", "rope.txt"}) {
		t.Fatalf("beach|holiday = %v", got)
	}
}

func TestTagViewUnknownTags(t *testing.T) {
	_, b, _ := holiday(t)
	list := func(expr string, tags ...string) []string {
		return listNames(t, b, viewAt(t, query(t, true, expr, tags...), "/"))
	}
	if got := list("all", "beach", "meadow"); len(got) != 0 {
		t.Fatalf("all with an unknown tag = %v, want nothing", got)
	}
	if got := list("any", "beach", "meadow"); len(got) != 3 {
		t.Fatalf("any with an unknown tag = %v, want the beach matches", got)
	}
	if got := list("any", "meadow", "thistle"); len(got) != 0 {
		t.Fatalf("only unknown tags = %v, want nothing", got)
	}
}

func TestTagViewPendingTags(t *testing.T) {
	x := testIndex(t, initIndex(t))
	d, _ := x.DB()
	addUnhashed(t, d, "/harbor/pebble.txt", 5)
	addUnhashed(t, d, "/harbor/gravel.txt", 7)
	tag(t, x, "/harbor/pebble.txt", "beach")
	b := NewBackend(x)
	if got := listNames(t, b, viewAt(t, query(t, true, "all", "beach"), "/")); !slices.Equal(got, []string{"pebble.txt"}) {
		t.Fatalf("pending = %v", got)
	}
}

func TestTagViewTaggedDirInsideTaggedDir(t *testing.T) {
	x := testIndex(t, initIndex(t))
	d, _ := x.DB()
	addFile(t, d, "/orchard/lantern/inner/meadow.txt", "meadow")
	addFile(t, d, "/orchard/lantern/clover.txt", "clover")
	tag(t, x, "/orchard/lantern", "beach")
	tag(t, x, "/orchard/lantern/inner", "beach")
	tag(t, x, "/orchard/lantern/inner/meadow.txt", "beach")
	b := NewBackend(x)
	q := query(t, true, "all", "beach")
	entries, err := b.List(context.Background(), viewAt(t, q, "/"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Loc.String())
	}
	want := []string{
		"fileindex@(beach):///orchard/lantern",
		"fileindex@(beach):///orchard/lantern/inner",
		"fileindex@(beach):///orchard/lantern/inner/meadow.txt",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if entries[0].Type != fsbackend.EntryDirectory || entries[2].Type != fsbackend.EntryFile || entries[2].Size != int64(len("meadow")) || entries[2].ModifiedAt.IsZero() {
		t.Fatalf("entries = %+v", entries)
	}
	// Structure: the outer directory lists everything below it, tagged or not.
	s := query(t, false, "all", "beach")
	if got := listNames(t, b, viewAt(t, s, "/orchard/lantern")); !slices.Equal(got, []string{"inner", "clover.txt"}) {
		t.Fatalf("covered = %v", got)
	}
	if got := listNames(t, b, viewAt(t, s, "/orchard/lantern/inner")); !slices.Equal(got, []string{"meadow.txt"}) {
		t.Fatalf("inside = %v", got)
	}
}

func TestTagViewStructureListing(t *testing.T) {
	_, b, _ := holiday(t)
	q := query(t, false, "any", "beach", "holiday")
	for host, want := range map[string][]string{
		"/":                   {"cedar", "orchard"},
		"/orchard":            {"anchor.txt", "rope.txt"}, // not plain.txt
		"/cedar":              {"lantern", "anchor-copy.txt"},
		"/cedar/lantern":      {"sail.txt"},
		"/cedar/lantern/sail": nil,
	} {
		loc := viewAt(t, q, host)
		entries, err := b.List(context.Background(), loc)
		if host == "/cedar/lantern/sail" {
			if !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("list %s = %v, want not exist", host, err)
			}
			continue
		}
		if err != nil || !slices.Equal(names(entries), want) {
			t.Fatalf("list %s = %v, %v; want %v", host, names(entries), err, want)
		}
		for _, e := range entries {
			if wantLoc, _ := loc.Join(e.Name); !e.Loc.Equal(wantLoc) {
				t.Fatalf("loc of %s = %s, want %s", e.Name, e.Loc, wantLoc)
			}
		}
	}
}

func TestTagViewEmptyAndMissingDirectories(t *testing.T) {
	x := testIndex(t, initIndex(t))
	b := NewBackend(x)
	q := query(t, false, "all", "beach")
	if got := listNames(t, b, viewAt(t, q, "/")); len(got) != 0 {
		t.Fatalf("root of an empty index = %v", got)
	}
	d, _ := x.DB()
	addFile(t, d, "/orchard/plain.txt", "plain")
	if got := listNames(t, b, viewAt(t, q, "/orchard")); len(got) != 0 {
		t.Fatalf("dir with no match = %v, want empty", got)
	}
	if _, err := b.List(context.Background(), viewAt(t, q, "/pebble")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing dir = %v, want not exist", err)
	}
	if got := listNames(t, b, viewAt(t, query(t, true, "all", "beach"), "/")); len(got) != 0 {
		t.Fatalf("flat with no matches = %v", got)
	}
}

func TestTagViewFlatRowsWithEqualNames(t *testing.T) {
	x := testIndex(t, initIndex(t))
	d, _ := x.DB()
	addFile(t, d, "/orchard/notes.txt", "one")
	addFile(t, d, "/cedar/notes.txt", "two")
	tag(t, x, "/orchard/notes.txt", "beach")
	tag(t, x, "/cedar/notes.txt", "beach")
	b := NewBackend(x)
	entries, err := b.List(context.Background(), viewAt(t, query(t, true, "all", "beach"), "/"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("rows = %+v, %v", entries, err)
	}
	if entries[0].Name != "notes.txt" || entries[1].Name != "notes.txt" ||
		entries[0].Loc.String() != "fileindex@(beach):///cedar/notes.txt" || entries[1].Loc.String() != "fileindex@(beach):///orchard/notes.txt" {
		t.Fatalf("rows = %v %v, want two notes.txt told apart by their paths, sorted", entries[0].Loc, entries[1].Loc)
	}
}

func TestTagViewFlatInnerListing(t *testing.T) {
	x := testIndex(t, initIndex(t))
	d, _ := x.DB()
	addFile(t, d, "/orchard/lantern/clover.txt", "clover")
	addFile(t, d, "/orchard/lantern/sub/thistle.txt", "thistle")
	tag(t, x, "/orchard/lantern", "beach")
	b := NewBackend(x)
	q := query(t, true, "all", "beach")
	row := viewAt(t, q, "/orchard/lantern//")
	entries, err := b.List(context.Background(), row)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(names(entries), []string{"sub", "clover.txt"}) || entries[0].Loc.String() != "fileindex@(beach):///orchard/lantern//sub" ||
		entries[1].Loc.String() != "fileindex@(beach):///orchard/lantern//clover.txt" {
		t.Fatalf("inner = %v", entries)
	}
	if got := listNames(t, b, entries[0].Loc); !slices.Equal(got, []string{"thistle.txt"}) {
		t.Fatalf("deeper = %v", got)
	}
}

func TestTagViewStart(t *testing.T) {
	x, _, d := holiday(t)
	ctx := context.Background()
	for _, c := range []struct {
		q       pathloc.TagQuery
		host    string
		matches int
	}{
		{query(t, false, "all", "beach", "holiday"), "/orchard", 1},
		{query(t, false, "all", "holiday"), "/", 2},
		{query(t, false, "all", "meadow"), "/", 0},
	} {
		host, n, err := x.TagViewStart(ctx, c.q)
		if err != nil || host != c.host || n != c.matches {
			t.Fatalf("start of %v = %q, %d, %v; want %q, %d", c.q.Tags, host, n, err, c.host, c.matches)
		}
	}
	addFile(t, d, "/orchard/lantern/deep/rose.txt", "rose")
	addFile(t, d, "/orchard/lantern/deep/pink.txt", "pink")
	tag(t, x, "/orchard/lantern/deep/rose.txt", "garden")
	tag(t, x, "/orchard/lantern/deep/pink.txt", "garden")
	if host, n, err := x.TagViewStart(ctx, query(t, false, "all", "garden")); err != nil || host != "/orchard/lantern/deep" || n != 2 {
		t.Fatalf("two files in one dir = %q, %d, %v", host, n, err)
	}
	tag(t, x, "/orchard/lantern", "garden")
	if host, n, err := x.TagViewStart(ctx, query(t, false, "all", "garden")); err != nil || host != "/orchard" || n != 3 {
		t.Fatalf("a tagged dir opens in its parent = %q, %d, %v", host, n, err)
	}
}

func TestTagNames(t *testing.T) {
	x, _, _ := holiday(t)
	got, err := x.TagNames(context.Background())
	if err != nil || !slices.Equal(got, []string{"beach", "holiday"}) {
		t.Fatalf("names = %v, %v", got, err)
	}
}

func TestTagViewRefusalsKeepRecords(t *testing.T) {
	x, b, d := holiday(t)
	addFile(t, d, "/orchard/lantern/clover.txt", "clover")
	tag(t, x, "/orchard/lantern", "garden")
	count := func() (n int) {
		if err := d.QueryRow(`SELECT (SELECT COUNT(*) FROM instance) + (SELECT COUNT(*) FROM dir)`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()
	ctx := context.Background()
	q := query(t, false, "any", "beach", "garden")
	flat := query(t, true, "any", "beach", "garden")
	other := viewAt(t, q, "/elsewhere")
	for _, loc := range []pathloc.Path{viewAt(t, q, "/orchard"), viewAt(t, q, "/"), viewAt(t, flat, "/")} {
		var nt *NotTaggedError
		if err := b.RemoveTree(ctx, loc); !errors.As(err, &nt) {
			t.Fatalf("RemoveTree(%s) = %v", loc, err)
		}
		if err := b.Remove(ctx, loc); !errors.As(err, &nt) {
			t.Fatalf("Remove(%s) = %v", loc, err)
		}
		if err := b.Rename(ctx, loc, other); !errors.As(err, &nt) {
			t.Fatalf("Rename(%s) = %v", loc, err)
		}
	}
	err := b.Remove(ctx, viewAt(t, q, "/orchard"))
	if err == nil || err.Error() != "Not tagged itself: orchard" {
		t.Fatalf("message = %v", err)
	}
	if count() != before {
		t.Fatal("a refusal changed records")
	}
	// A tagged directory, a file in a view and a flat row are changed as usual.
	if err := b.Rename(ctx, viewAt(t, q, "/orchard/anchor.txt"), viewAt(t, q, "/orchard/anchors.txt")); err != nil {
		t.Fatal(err)
	}
	if err := b.Remove(ctx, viewAt(t, flat, "/orchard/rope.txt")); err != nil {
		t.Fatal(err)
	}
	if err := b.Remove(ctx, viewAt(t, q, "/orchard/lantern/clover.txt")); err != nil {
		t.Fatal(err)
	}
	if err := b.RemoveTree(ctx, viewAt(t, q, "/orchard/lantern")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Stat(ctx, viewAt(t, q, "/orchard/plain.txt")); err != nil {
		t.Fatalf("an untouched record = %v", err)
	}
}

func TestTagViewSubtreeSize(t *testing.T) {
	x := testIndex(t, initIndex(t))
	d, _ := x.DB()
	addFile(t, d, "/orchard/anchor.txt", "anchor")               // 6, tagged file
	addFile(t, d, "/orchard/lantern/clover.txt", "clover")       // 6, in a tagged dir
	addFile(t, d, "/orchard/lantern/inner/rose.txt", "rose red") // 8, tagged file in a tagged dir in a tagged dir
	addFile(t, d, "/orchard/lantern/inner/thorn.txt", "thorn")   // 5, untagged in the inner dir
	addFile(t, d, "/orchard/plain.txt", "plain")                 // untagged
	addFile(t, d, "/cedar/sub/sail.txt", "sail")                 // 4, tagged
	addFile(t, d, "/cedar/sub/mast.txt", "mast pole")            // untagged
	tag(t, x, "/orchard/anchor.txt", "beach")
	tag(t, x, "/orchard/lantern", "beach")
	tag(t, x, "/orchard/lantern/inner", "beach")
	tag(t, x, "/orchard/lantern/inner/rose.txt", "beach")
	tag(t, x, "/cedar/sub/sail.txt", "beach")
	ctx := context.Background()
	s, f := query(t, false, "all", "beach"), query(t, true, "all", "beach")
	for _, c := range []struct {
		loc          pathloc.Path
		bytes, files int64
	}{
		{viewAt(t, f, "/"), 6 + 6 + 8 + 5 + 4, 5}, // anchor, lantern's three files, sail: nothing counted twice
		{viewAt(t, s, "/"), 6 + 6 + 8 + 5 + 4, 5},
		{viewAt(t, s, "/orchard"), 6 + 6 + 8 + 5, 4},
		{viewAt(t, s, "/cedar"), 4, 1},
		{viewAt(t, s, "/orchard/lantern"), 6 + 8 + 5, 3},    // tagged: all of its tree
		{viewAt(t, s, "/orchard/lantern/inner"), 8 + 5, 2},  // inside a tagged dir
		{viewAt(t, f, "/orchard/lantern//inner"), 8 + 5, 2}, // inside a flat row
		{viewAt(t, s, "/orchard/anchor.txt"), 6, 1},         // a file
		{viewAt(t, f, "/orchard/lantern//clover.txt"), 6, 1},
	} {
		bytes, files, err := x.SubtreeSize(ctx, c.loc.String())
		if err != nil || bytes != c.bytes || files != c.files {
			t.Fatalf("SubtreeSize(%s) = %d bytes %d files %v, want %d %d", c.loc, bytes, files, err, c.bytes, c.files)
		}
	}
	if _, _, err := x.SubtreeSize(ctx, viewAt(t, s, "/pebble").String()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing = %v", err)
	}
}

// settle backdates the database files past tagViewSettle, as if the last write were long ago.
func settle(t testing.TB, d *db.DB) {
	t.Helper()
	old := time.Now().Add(-time.Minute)
	for _, p := range []string{d.Path, d.Path + "-wal"} {
		if err := os.Chtimes(p, old, old); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
	}
}

func TestTagViewFreshWriteIsNotCached(t *testing.T) {
	x, _, d := holiday(t)
	now := time.Now()
	if err := os.Chtimes(d.Path, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := x.matches(context.Background(), query(t, false, "all", "beach")); err != nil {
		t.Fatal(err)
	}
	if len(x.tagViews.sets) != 0 {
		t.Fatal("kept a match set read right after a write")
	}
}

func TestTagViewCacheReuseAndDrop(t *testing.T) {
	x, b, d := holiday(t)
	settle(t, d)
	ctx := context.Background()
	q := query(t, false, "all", "beach")
	first, err := x.matches(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := x.matches(ctx, query(t, true, "all", "beach")); again != first {
		t.Fatal("a second listing rebuilt the match set; flat and structure share it")
	}
	if got := listNames(t, b, viewAt(t, query(t, true, "all", "beach"), "/")); len(got) != 3 {
		t.Fatalf("rows = %v", got)
	}
	// Another handle (the fileindex command, say) tags one more file.
	other, err := db.Open(d.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Close() }()
	inst, found, err := other.InstanceID("/orchard/plain.txt")
	if err != nil || !found {
		t.Fatalf("instance = %v, %v, %v", inst, found, err)
	}
	if _, err := other.AddTags(db.TagTarget{Instances: []int64{inst}}, []string{"beach"}); err != nil {
		t.Fatal(err)
	}
	if got := listNames(t, b, viewAt(t, query(t, true, "all", "beach"), "/")); len(got) != 4 {
		t.Fatalf("rows after an outside write = %v, want the new match", got)
	}
	if next, _ := x.matches(ctx, q); next == first {
		t.Fatal("the match set outlived a write")
	}
}

func TestTagViewCacheIsBounded(t *testing.T) {
	x, _, d := holiday(t)
	settle(t, d)
	ctx := context.Background()
	for i := range tagViewMax + 3 {
		if _, err := x.matches(ctx, query(t, false, "all", fmt.Sprintf("meadow%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(x.tagViews.sets); n != tagViewMax || len(x.tagViews.order) != tagViewMax {
		t.Fatalf("cached %d sets, want %d", n, tagViewMax)
	}
}

func TestTagViewCancelIsNotCached(t *testing.T) {
	x, b, _ := holiday(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	loc := viewAt(t, query(t, true, "all", "beach"), "/")
	if _, err := b.List(ctx, loc); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled list = %v", err)
	}
	if len(x.tagViews.sets) != 0 {
		t.Fatal("a failed build was cached")
	}
	if got := listNames(t, b, loc); len(got) != 3 {
		t.Fatalf("list after a cancel = %v", got)
	}
}

func TestTagViewConcurrentListings(t *testing.T) {
	_, b, _ := holiday(t)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, loc := range []pathloc.Path{
				viewAt(t, query(t, i%2 == 0, "any", "beach", "holiday"), "/"),
				viewAt(t, query(t, false, "all", "beach"), "/orchard"),
			} {
				entries, err := b.List(context.Background(), loc)
				if err != nil || len(entries) == 0 {
					t.Errorf("list %s = %v, %v", loc, entries, err)
				}
				slices.Reverse(entries) // the caller owns what it gets
			}
		}()
	}
	wg.Wait()
	if got := listNames(t, b, viewAt(t, query(t, true, "any", "beach", "holiday"), "/")); !slices.Equal(got, []string{"anchor-copy.txt", "sail.txt", "anchor.txt", "rope.txt"}) {
		t.Fatalf("cached rows were modified: %v", got)
	}
}

func TestTagViewStat(t *testing.T) {
	_, b, _ := holiday(t)
	ctx := context.Background()
	q := query(t, true, "all", "beach")
	for _, host := range []string{"/", "/orchard", "/orchard/anchor.txt"} {
		if _, err := b.Stat(ctx, viewAt(t, q, host)); err != nil {
			t.Fatalf("stat %s = %v", host, err)
		}
	}
	if _, err := b.Stat(ctx, viewAt(t, q, "/pebble")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("stat missing = %v", err)
	}
}

// TestTagViewCopyOutKeepsOnlyMatches: copying an intermediate directory of a structure view to
// disk goes through Backend.List, so only the matches arrive, in their structure.
func TestTagViewCopyOutKeepsOnlyMatches(t *testing.T) {
	x := testIndex(t, initIndex(t))
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"orchard/anchor.txt":         "anchor",
		"orchard/plain.txt":          "plain",
		"orchard/lantern/clover.txt": "clover",
		"orchard/lantern/thorn.txt":  "thorn",
		"orchard/lantern/deep/a.txt": "deep one",
		"cedar/sail.txt":             "sail",
	} {
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
	tag(t, x, filepath.Join(root, "orchard/anchor.txt"), "beach")
	tag(t, x, filepath.Join(root, "orchard/lantern/deep"), "beach")
	tag(t, x, filepath.Join(root, "cedar/sail.txt"), "beach")
	prev := current.Load()
	if err := Register(x); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { current.Store(prev) })

	q := query(t, false, "all", "beach")
	dst := t.TempDir()
	src := viewAt(t, q, filepath.Join(root, "orchard"))
	if _, _, err := ops.ExecuteCopy(context.Background(), []pathloc.Path{src}, ops.MustPath(dst), ops.Options{CopyBufferKiB: 4}, ops.ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	var got []string
	_ = filepath.WalkDir(filepath.Join(dst, "orchard"), func(p string, e fs.DirEntry, err error) error {
		if err == nil && !e.IsDir() {
			rel, _ := filepath.Rel(dst, p)
			got = append(got, rel)
		}
		return nil
	})
	want := []string{"orchard/anchor.txt", "orchard/lantern/deep/a.txt"}
	if !slices.Equal(got, want) {
		t.Fatalf("copied %v, want %v", got, want)
	}
}

// BenchmarkTagViewHundredThousand: a 100k-file index with tagged contents; the cold build of the
// match set and a listing served from the cache.
func BenchmarkTagViewHundredThousand(b *testing.B) {
	const files, perDir = 100_000, 20
	path := filepath.Join(b.TempDir(), "index.sqlite")
	d, err := db.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	if err := d.Init(); err != nil {
		b.Fatal(err)
	}
	tx, err := d.Begin()
	if err != nil {
		b.Fatal(err)
	}
	dirs := map[string]int64{}
	dirID := func(p string) int64 {
		if id, ok := dirs[p]; ok {
			return id
		}
		id, err := db.GetOrCreatePath(tx, p)
		if err != nil {
			b.Fatal(err)
		}
		dirs[p] = *id
		return *id
	}
	var tagID int64
	if err := tx.QueryRow(`INSERT INTO tag (name) VALUES ('beach') RETURNING id`).Scan(&tagID); err != nil {
		b.Fatal(err)
	}
	for i := range files {
		dir := dirID(fmt.Sprintf("/orchard/shelf%d/row%d", i/perDir/50, i/perDir))
		hash := sha256.Sum256(fmt.Appendf(nil, "content %d", i))
		sha := hex.EncodeToString(hash[:])
		var fileID int64
		if err := tx.QueryRow(`INSERT INTO file (md5, sha256, filesize) VALUES (?, ?, 10) RETURNING id`, sha[:32], sha).Scan(&fileID); err != nil {
			b.Fatal(err)
		}
		name := fmt.Sprintf("meadow%d.txt", i)
		if _, err := tx.Exec(`INSERT INTO instance (file_id, dir_id, filename, filename_lower, size, mtime) VALUES (?, ?, ?, ?, 10, '2024-01-02 03:04:05')`, fileID, dir, name, name); err != nil {
			b.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO content_tag (tag_id, sha256) VALUES (?, ?)`, tagID, sha); err != nil {
			b.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	_ = d.Close()
	x := New(config.FileIndexConfig{Database: path})
	defer func() { _ = x.Close() }()
	ctx := context.Background()
	be := NewBackend(x)
	settle(b, d)
	q := query(b, true, "all", "beach")
	b.Run("cold build", func(b *testing.B) {
		for range b.N {
			x.tagViews = tagViewCache{}
			if entries, err := be.List(ctx, viewAt(b, q, "/")); err != nil || len(entries) != files {
				b.Fatalf("rows = %d, %v", len(entries), err)
			}
		}
	})
	b.Run("cached flat list", func(b *testing.B) {
		for range b.N {
			if entries, err := be.List(ctx, viewAt(b, q, "/")); err != nil || len(entries) != files {
				b.Fatalf("rows = %d, %v", len(entries), err)
			}
		}
	})
	s := query(b, false, "all", "beach")
	b.Run("cached structure list", func(b *testing.B) {
		for range b.N {
			if entries, err := be.List(ctx, viewAt(b, s, "/orchard/shelf1")); err != nil || len(entries) == 0 {
				b.Fatalf("rows = %d, %v", len(entries), err)
			}
		}
	})
}

func TestDirectoryColumn(t *testing.T) {
	x := testIndex(t, initIndex(t))
	x.home = "/home/willow"
	d, _ := x.DB()
	addFile(t, d, "/home/willow/orchard/notes.txt", "one")
	addFile(t, d, "/home/willow/cedar/notes.txt", "two")
	addFile(t, d, "/srv/harbor/lantern/thistle/juniper/saffron/notes.txt", "three")
	for _, p := range []string{"/home/willow/orchard/notes.txt", "/home/willow/cedar/notes.txt", "/srv/harbor/lantern/thistle/juniper/saffron/notes.txt"} {
		tag(t, x, p, "beach")
	}
	b := NewBackend(x)
	rows, err := b.List(context.Background(), viewAt(t, query(t, true, "all", "beach"), "/"))
	if err != nil || len(rows) != 3 {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
	// The meta handler calls the provider once per directory of the list, with that group's rows.
	got := map[string]string{}
	for _, r := range rows {
		dir := r.Loc.Parent()
		v, err := x.Provider(DirectoryColumn).Values(context.Background(), dir.String(), []localfs.Entry{
			{Name: r.Name, Path: r.Loc.String()},
			{Name: "..", Path: dir.String()},
		})
		if err != nil || len(v) != 1 {
			t.Fatalf("values = %v, %v, want the row only (no parent row)", v, err)
		}
		got[r.Loc.String()] = v[r.Loc.String()]
	}
	if got["fileindex@(beach):///home/willow/cedar/notes.txt"] != "~/cedar" || got["fileindex@(beach):///home/willow/orchard/notes.txt"] != "~/orchard" {
		t.Fatalf("directories = %v, want the two equal names told apart, with ~ for home", got)
	}
	long := got["fileindex@(beach):///srv/harbor/lantern/thistle/juniper/saffron/notes.txt"]
	if runewidth.StringWidth(long) > panel.MetaCellMaxWidth || !strings.HasSuffix(long, "saffron") {
		t.Fatalf("long directory = %q, want at most %d cells with its last name whole", long, panel.MetaCellMaxWidth)
	}
}

func TestDirectoryColumnLocalPanel(t *testing.T) {
	x, root := scanned(t)
	v, err := x.Provider(DirectoryColumn).Values(context.Background(), root, []localfs.Entry{{Name: "harbor.txt", Path: filepath.Join(root, "harbor.txt")}})
	if err != nil || v[filepath.Join(root, "harbor.txt")] == "" {
		t.Fatalf("values = %v, %v, want the directory on a local panel", v, err)
	}
}
