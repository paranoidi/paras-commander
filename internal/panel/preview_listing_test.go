package panel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// primeCarouselParentCache simulates the async parent-snapshot apply (internal/app's
// applyCarouselSnapshot) so SnapshotParent's pure-cache-read behavior can be tested without a real
// async round trip.
func primeCarouselParentCache(t *testing.T, state *State, viewportRows int) {
	t.Helper()
	target, ok := state.CarouselParentPreviewTarget()
	if !ok {
		t.Fatal("no parent preview target")
	}
	snap, err := state.buildListingSnapshot(state.Path.Parent(), state.Path.Base(), noIndexCursorFallback, viewportRows, false)
	if err != nil {
		t.Fatal(err)
	}
	state.CarouselSideCache.Parent = snap
	state.CarouselSideCache.ParentOK = true
	state.CarouselSideCache.ParentSourceDir = target
}

// primeCarouselChildCache simulates the async child-snapshot apply (internal/app's
// applyCarouselSnapshot) so SnapshotChild's pure-cache-read behavior can be tested without a real
// async round trip.
func primeCarouselChildCache(t *testing.T, state *State, viewportRows int) {
	t.Helper()
	target, ok := state.CarouselChildPreviewTarget()
	if !ok {
		t.Fatal("no child preview target")
	}
	child, err := pathloc.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	selectedName, indexFallback, centerRecalled := state.recalledCursorFor(target)
	snap, err := state.buildListingSnapshot(child, selectedName, indexFallback, viewportRows, centerRecalled)
	if err != nil {
		t.Fatal(err)
	}
	state.CarouselSideCache.Child = snap
	state.CarouselSideCache.ChildOK = true
	state.CarouselSideCache.ChildCursorDir = target
}

func TestSnapshotParentFalseAtRoot(t *testing.T) {
	state, err := New("/")
	if err != nil {
		t.Skip("cannot open /:", err)
	}
	if _, ok := state.SnapshotParent(); ok {
		t.Fatal("SnapshotParent at filesystem root = true, want false")
	}
}

func TestSnapshotParentHighlightsChildDir(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "walnut")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	state, err := New(child)
	if err != nil {
		t.Fatal(err)
	}
	primeCarouselParentCache(t, &state, 10)
	snap, ok := state.SnapshotParent()
	if !ok {
		t.Fatal("SnapshotParent = false, want true")
	}
	if snap.Path.String() != root {
		t.Fatalf("parent path = %q, want %q", snap.Path.String(), root)
	}
	if snap.Cursor < 0 || snap.Cursor >= len(snap.Entries) || snap.Entries[snap.Cursor].Name != "walnut" {
		t.Fatalf("cursor index %d entries = %v, want walnut", snap.Cursor, snap.Entries)
	}
}

func TestSnapshotChildFalseOnFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "cedar.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range state.Entries {
		if e.Name == "cedar.txt" {
			state.Cursor = i
			break
		}
	}
	if _, ok := state.SnapshotChild(); ok {
		t.Fatal("SnapshotChild on file = true, want false")
	}
}

func TestSnapshotChildAppliesIdleDiskTotalsSort(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "maple")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	for name := range map[string]int64{"small.bin": 100, "huge.bin": 8000} {
		if err := os.WriteFile(filepath.Join(child, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	state, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	state.Sort.DiskUsageIdleSizeSort = true
	state.IdleDiskTotalsSort = true
	cache := map[string]int64{
		filepath.Join(child, "small.bin"): 100,
		filepath.Join(child, "huge.bin"):  8000,
	}
	state.DiskSorter = func(abs string) (int64, bool) {
		v, ok := cache[filepath.Clean(abs)]
		return v, ok
	}
	if !state.SelectVisibleEntry("maple") {
		t.Fatal("maple not found")
	}
	primeCarouselChildCache(t, &state, 10)
	snap, ok := state.SnapshotChild()
	if !ok {
		t.Fatal("SnapshotChild = false, want true")
	}
	if len(snap.Entries) < 2 {
		t.Fatalf("entries = %v, want at least two files", snap.Entries)
	}
	if snap.Entries[0].Name != "huge.bin" || snap.Entries[1].Name != "small.bin" {
		t.Fatalf("child preview order = %q then %q, want huge.bin before small.bin by disk usage", snap.Entries[0].Name, snap.Entries[1].Name)
	}
}

func TestSnapshotParentIgnoresChildCoalesce(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "walnut")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	state, err := New(child)
	if err != nil {
		t.Fatal(err)
	}
	primeCarouselParentCache(t, &state, 10)
	state.CarouselChildPreviewCoalesce = true
	snap, ok := state.SnapshotParent()
	if !ok {
		t.Fatal("SnapshotParent during child coalesce = false, want true")
	}
	if snap.Path.String() != root {
		t.Fatalf("parent path = %q, want %q", snap.Path.String(), root)
	}
}

func TestSnapshotChildCoalesceUsesCache(t *testing.T) {
	root := t.TempDir()
	maple := filepath.Join(root, "maple")
	oak := filepath.Join(root, "oak")
	for _, dir := range []string{maple, oak} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	state, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !state.SelectVisibleEntry("maple") {
		t.Fatal("maple not found")
	}
	primeCarouselChildCache(t, &state, 10)
	first, ok := state.SnapshotChild()
	if !ok {
		t.Fatal("SnapshotChild = false, want true")
	}
	if first.Path.String() != maple {
		t.Fatalf("child path = %q, want %q", first.Path.String(), maple)
	}
	state.CarouselChildPreviewCoalesce = true
	if !state.SelectVisibleEntry("oak") {
		t.Fatal("oak not found")
	}
	if state.CarouselChildCacheValid() {
		t.Fatal("strict cache valid should be false after cursor moved to oak")
	}
	if !state.CarouselChildCachePaintDuringCoalesce() {
		t.Fatal("coalesce should still paint cached child until flush")
	}
	cached, ok := state.SnapshotChild()
	if !ok {
		t.Fatal("coalesced SnapshotChild = false, want cached paint")
	}
	if cached.Path.String() != maple {
		t.Fatalf("coalesced child path = %q, want cached %q", cached.Path.String(), maple)
	}
}

func TestCarouselChildCacheInvalidatedOnChdir(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "maple")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !state.SelectVisibleEntry("maple") {
		t.Fatal("maple not found")
	}
	primeCarouselChildCache(t, &state, 10)
	if _, ok := state.SnapshotChild(); !ok {
		t.Fatal("SnapshotChild = false, want true")
	}
	if !state.CarouselSideCache.ChildOK {
		t.Fatal("child cache should be warm")
	}
	if _, err := state.Enter(10); err != nil {
		t.Fatal(err)
	}
	if state.CarouselSideCache.ChildOK {
		t.Fatal("child cache should be cleared after entering maple")
	}
}

func TestCarouselChildCachePaintDuringCoalesceOnFileCursor(t *testing.T) {
	root := t.TempDir()
	maple := filepath.Join(root, "maple")
	if err := os.Mkdir(maple, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cedar.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !state.SelectVisibleEntry("maple") {
		t.Fatal("maple not found")
	}
	primeCarouselChildCache(t, &state, 10)
	if _, ok := state.SnapshotChild(); !ok {
		t.Fatal("SnapshotChild = false, want true")
	}
	for i, e := range state.Entries {
		if e.Name == "cedar.txt" {
			state.Cursor = i
			break
		}
	}
	state.CarouselChildPreviewCoalesce = true
	if !state.CarouselChildCachePaintDuringCoalesce() {
		t.Fatal("coalesce should paint cached child while cursor is on a file")
	}
	if _, ok := state.SnapshotChild(); !ok {
		t.Fatal("coalesced SnapshotChild = false, want cached maple listing")
	}
}

func TestCarouselChildCacheValidOnlyOnDirectoryCursor(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cedar.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range state.Entries {
		if e.Name == "cedar.txt" {
			state.Cursor = i
			break
		}
	}
	state.CarouselSideCache.ChildOK = true
	state.CarouselSideCache.ChildCursorDir = filepath.Join(root, "cedar.txt")
	if state.CarouselChildCacheValid() {
		t.Fatal("file under cursor should not validate child cache")
	}
}

func TestSnapshotChildRecallsCursor(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "maple")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "birch.log"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !state.SelectVisibleEntry("maple") {
		t.Fatal("maple not found")
	}
	if _, err := state.Enter(10); err != nil {
		t.Fatal(err)
	}
	if !state.SelectVisibleEntry("birch.log") {
		t.Fatal("birch.log not found in child")
	}
	if err := state.Parent(10); err != nil {
		t.Fatal(err)
	}
	primeCarouselChildCache(t, &state, 10)
	snap, ok := state.SnapshotChild()
	if !ok {
		t.Fatal("SnapshotChild = false, want true")
	}
	if snap.Path.String() != child {
		t.Fatalf("child path = %q, want %q", snap.Path.String(), child)
	}
	if snap.Cursor < 0 || snap.Cursor >= len(snap.Entries) || snap.Entries[snap.Cursor].Name != "birch.log" {
		t.Fatalf("cursor index %d entries = %v, want birch.log", snap.Cursor, snap.Entries)
	}
}
