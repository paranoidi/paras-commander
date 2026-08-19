package prefetch

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func writeSolidPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestPriorityGroupPrioritizesDirectionOfTravel(t *testing.T) {
	cases := []struct {
		name     string
		offset   int
		dir      int
		pageSize int
		window   int
		want     int
	}{
		{"caret entry always first", 0, 1, 0, 5, 0},
		{"caret entry first even with no dir", 0, 0, 0, 5, 0},
		{"near neighbor within radius", 2, 0, 0, 5, 1},
		{"near neighbor within radius, negative", -2, 0, 0, 5, 1},
		{"no dir bias: ahead same group as behind", 3, 0, 0, 5, 3},
		{"no dir bias: behind same group as ahead", -3, 0, 0, 5, 3},
		{"forward dir: ahead prioritized", 3, 1, 0, 5, 3},
		{"forward dir: behind deprioritized", -3, 1, 0, 5, 4},
		{"backward dir: behind prioritized", -3, -1, 0, 5, 3},
		{"backward dir: ahead deprioritized", 3, -1, 0, 5, 4},
		{"exact forward landing offset", 20, 0, 20, 5, 2},
		{"exact backward landing offset", -20, 0, 20, 5, 2},
		{"landing vicinity, outside cursor window", 22, 0, 20, 5, 5},
		{"landing vicinity, outside cursor window, backward", -22, 0, 20, 5, 5},
		{"no pageSize: far offset stays in dir-based tier", 20, 0, 0, 5, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := priorityGroup(c.offset, c.dir, c.pageSize, c.window); got != c.want {
				t.Errorf("priorityGroup(%d, %d, %d, %d) = %d, want %d", c.offset, c.dir, c.pageSize, c.window, got, c.want)
			}
		})
	}
}

func TestScheduleOrdersPendingByDirectionThenDistance(t *testing.T) {
	e := &Engine{cache: NewCache(1<<20, 0, 1<<20, "")}
	e.cond = sync.NewCond(&e.mu)
	items := []Item{
		{Path: "far-behind", Kind: KindImage, Offset: -3},
		{Path: "near-ahead", Kind: KindImage, Offset: 1},
		{Path: "caret", Kind: KindImage, Offset: 0},
		{Path: "near-behind", Kind: KindImage, Offset: -1},
		{Path: "far-ahead", Kind: KindImage, Offset: 3},
	}
	e.Schedule(items, 1, 0, 5, nil) // caret moving forward, window covers all offsets
	want := []string{"caret", "near-ahead", "near-behind", "far-ahead", "far-behind"}
	if len(e.pending) != len(want) {
		t.Fatalf("pending len = %d, want %d", len(e.pending), len(want))
	}
	for i, p := range want {
		if e.pending[i].Path != p {
			t.Errorf("pending[%d] = %q, want %q", i, e.pending[i].Path, p)
		}
	}
}

func TestScheduleOrdersLandingRowsBeforeRestOfWindow(t *testing.T) {
	e := &Engine{cache: NewCache(1<<20, 0, 1<<20, "")}
	e.cond = sync.NewCond(&e.mu)
	items := []Item{
		{Path: "window-far-ahead", Kind: KindImage, Offset: 5},
		{Path: "landing-ahead", Kind: KindImage, Offset: 20},
		{Path: "caret", Kind: KindImage, Offset: 0},
		{Path: "near-ahead", Kind: KindImage, Offset: 2},
		{Path: "landing-vicinity-ahead", Kind: KindImage, Offset: 22},
	}
	e.Schedule(items, 0, 20, 5, nil) // pageSize 20, window 5, no dir bias
	// landing-vicinity-ahead (tier 5) is dropped: near-cursor work (caret, near-ahead) is
	// outstanding in this same call, so the hard gate excludes it entirely.
	want := []string{"caret", "near-ahead", "landing-ahead", "window-far-ahead"}
	if len(e.pending) != len(want) {
		t.Fatalf("pending len = %d, want %d", len(e.pending), len(want))
	}
	for i, p := range want {
		if e.pending[i].Path != p {
			t.Errorf("pending[%d] = %q, want %q", i, e.pending[i].Path, p)
		}
	}
	for _, it := range e.pending {
		if it.Path == "landing-vicinity-ahead" {
			t.Errorf("landing-vicinity-ahead should be gated out while near-cursor work is outstanding")
		}
	}
}

func TestScheduleIncludesLandingVicinityOnlyWhenNearWorkDone(t *testing.T) {
	t.Run("no near-cursor work: landing vicinity is scheduled", func(t *testing.T) {
		e := &Engine{cache: NewCache(1<<20, 0, 1<<20, "")}
		e.cond = sync.NewCond(&e.mu)
		items := []Item{
			{Path: "landing-vicinity-ahead", Kind: KindImage, Offset: 22},
		}
		e.Schedule(items, 0, 20, 5, nil) // pageSize 20, window 5
		if len(e.pending) != 1 || e.pending[0].Path != "landing-vicinity-ahead" {
			t.Fatalf("pending = %+v, want landing-vicinity-ahead scheduled (nothing to gate behind)", e.pending)
		}
	})

	t.Run("near-cursor work present: landing vicinity is dropped", func(t *testing.T) {
		e := &Engine{cache: NewCache(1<<20, 0, 1<<20, "")}
		e.cond = sync.NewCond(&e.mu)
		items := []Item{
			{Path: "near-ahead", Kind: KindImage, Offset: 2},
			{Path: "landing-vicinity-ahead", Kind: KindImage, Offset: 22},
		}
		e.Schedule(items, 0, 20, 5, nil) // pageSize 20, window 5
		want := []string{"near-ahead"}
		if len(e.pending) != len(want) {
			t.Fatalf("pending = %+v, want %v", e.pending, want)
		}
		if e.pending[0].Path != "near-ahead" {
			t.Errorf("pending[0] = %q, want near-ahead", e.pending[0].Path)
		}
	})
}

func TestWithinPrefetchRange(t *testing.T) {
	cases := []struct {
		name     string
		offset   int
		window   int
		pageSize int
		want     bool
	}{
		{"inside cursor window", 3, 5, 20, true},
		{"outside cursor window, no landing band", 10, 5, 0, false},
		{"inside forward landing window", 21, 5, 20, true},
		{"inside backward landing window", -18, 5, 20, true},
		{"far from both cursor and landing windows", 40, 5, 20, false},
		{"exactly at cursor window edge", 5, 5, 20, true},
		{"exactly at landing window edge", 25, 5, 20, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := withinPrefetchRange(c.offset, c.window, c.pageSize); got != c.want {
				t.Errorf("withinPrefetchRange(%d, %d, %d) = %v, want %v", c.offset, c.window, c.pageSize, got, c.want)
			}
		})
	}
}

// prefetchTestWords is a small pool of random English words used to build synthetic filenames,
// per this repo's testing convention of not basing test data on real filenames.
var prefetchTestWords = []string{
	"meadow", "harbor", "lantern", "orchard", "granite", "willow", "compass", "thistle",
	"ember", "quarry", "falcon", "cedar", "marble", "brindle", "copper", "pebble",
	"juniper", "hollow", "beacon", "walnut", "cinder", "ridge", "amber", "clover",
	"basalt", "raven", "timber", "lagoon", "spruce", "canyon", "birch", "meridian",
	"tundra", "opal", "sable", "vellum", "quartz", "hazel", "cascade", "flint",
	"maple", "prairie", "onyx", "cobalt", "linden", "saffron", "delta", "auburn",
	"pinnacle", "ivory",
}

// TestScheduleRequeuesStillWarmItemWhenRenderColdForBox guards against the bug where an item
// whose still-PNG tier warmed for a box that was unavailable/different at the time (e.g. quick
// view opened before layout metrics were ready) got dropped from Schedule forever, even though
// its render-payload tier was never warmed for the box actually in use. Schedule must keep
// offering the item so a worker can eagerly warm LoadRender for the current box.
func TestScheduleRequeuesStillWarmItemWhenRenderColdForBox(t *testing.T) {
	c := NewCache(1<<20, 0, 1<<20, "")
	e := &Engine{cache: c, cfg: Config{ImageMaxEdgePx: 64}}
	e.cond = sync.NewCond(&e.mu)

	const path = "lantern.png"
	const mtime, size = 1, 10
	// Pre-warm only the still-PNG tier, not the render tier.
	if _, _, err := c.LoadStill(context.Background(), path, mtime, size, 64,
		func(context.Context) ([]byte, string, error) { return []byte("still"), "", nil }); err != nil {
		t.Fatal(err)
	}

	box := &RenderBox{Proto: previewpanel.ImageProtocolKitty, MaxPxW: 20, MaxPxH: 20}
	e.Schedule([]Item{{Path: path, Kind: KindImage, Mtime: mtime, Size: size}}, 0, 0, 5, box)

	if len(e.pending) != 1 || e.pending[0].Path != path {
		t.Fatalf("pending = %+v, want %q scheduled (still warm but render cold for this box)", e.pending, path)
	}
}

// TestScheduleDropsItemWhenBothTiersWarmForBox is the counterpart: once both the still-PNG and
// the render-payload tier are warm for the exact box in use, Schedule must drop the item as before.
func TestScheduleDropsItemWhenBothTiersWarmForBox(t *testing.T) {
	c := NewCache(1<<20, 0, 1<<20, "")
	e := &Engine{cache: c, cfg: Config{ImageMaxEdgePx: 64}}
	e.cond = sync.NewCond(&e.mu)

	const path = "cinder.png"
	const mtime, size = 1, 10
	box := &RenderBox{Proto: previewpanel.ImageProtocolKitty, MaxPxW: 20, MaxPxH: 20}
	if _, _, err := c.LoadStill(context.Background(), path, mtime, size, 64,
		func(context.Context) ([]byte, string, error) { return []byte("still"), "", nil }); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := c.LoadRender(context.Background(), path, mtime, size,
		box.Proto, box.UnicodePlaceholder, box.InTmux, box.MaxPxW, box.MaxPxH,
		func(context.Context) ([]byte, int, int, error) { return []byte("render"), 20, 20, nil }); err != nil {
		t.Fatal(err)
	}

	e.Schedule([]Item{{Path: path, Kind: KindImage, Mtime: mtime, Size: size}}, 0, 0, 5, box)

	if len(e.pending) != 0 {
		t.Fatalf("pending = %+v, want empty (both tiers warm for this box)", e.pending)
	}
}

// TestIsEntryWarmChecksRenderTierForBox guards against the debug icon-tint bug where an entry
// showed "warm" based only on the still-PNG decode tier, while its render payload for the current
// box was still cold (and a worker was actively computing it) — a contradictory white-icon-with-
// spinner state.
func TestIsEntryWarmChecksRenderTierForBox(t *testing.T) {
	c := NewCache(1<<20, 0, 1<<20, "")
	e := &Engine{cache: c, cfg: Config{ImageMaxEdgePx: 64}}
	e.cond = sync.NewCond(&e.mu)

	const path = "willow.png"
	const mtime, size = 1, 10
	ent := localfs.Entry{Path: path, ModifiedAt: time.Unix(0, mtime), Size: size}
	box := &RenderBox{Proto: previewpanel.ImageProtocolKitty, MaxPxW: 20, MaxPxH: 20}

	if e.IsEntryWarm(ent, box) {
		t.Fatal("IsEntryWarm before any warming: want false")
	}

	if _, _, err := c.LoadStill(context.Background(), path, mtime, size, 64,
		func(context.Context) ([]byte, string, error) { return []byte("still"), "", nil }); err != nil {
		t.Fatal(err)
	}

	// Still-PNG tier warm but render tier cold for this box: must report cold, not warm.
	if e.IsEntryWarm(ent, box) {
		t.Fatal("IsEntryWarm with still warm but render cold for box: want false")
	}

	// box == nil: only the decode tier matters (matches pre-fix behavior for that case).
	if !e.IsEntryWarm(ent, nil) {
		t.Fatal("IsEntryWarm with still warm and box == nil: want true")
	}

	if _, _, _, err := c.LoadRender(context.Background(), path, mtime, size,
		box.Proto, box.UnicodePlaceholder, box.InTmux, box.MaxPxW, box.MaxPxH,
		func(context.Context) ([]byte, int, int, error) { return []byte("render"), 20, 20, nil }); err != nil {
		t.Fatal(err)
	}

	// Both tiers warm for this exact box: now reports warm.
	if !e.IsEntryWarm(ent, box) {
		t.Fatal("IsEntryWarm with both tiers warm for box: want true")
	}
}

// TestScheduleExcludesInFlightItem guards against the bug where an item currently being warmed by
// one worker got re-queued by every subsequent Schedule call (since its render tier was still cold
// for the box), letting a different idle worker dequeue the same path and block inside LoadRender's
// doFlight singleflight wait instead of doing other work. Schedule must drop in-flight items instead
// of re-queuing them; the in-flight worker's own completion is what makes the next Schedule call
// re-evaluate the item correctly.
func TestScheduleExcludesInFlightItem(t *testing.T) {
	c := NewCache(1<<20, 0, 1<<20, "")
	e := &Engine{cache: c, cfg: Config{ImageMaxEdgePx: 64}}
	e.cond = sync.NewCond(&e.mu)

	const path = "compass.png"
	const mtime, size = 1, 10
	box := &RenderBox{Proto: previewpanel.ImageProtocolKitty, MaxPxW: 20, MaxPxH: 20}
	item := Item{Path: path, Kind: KindImage, Mtime: mtime, Size: size}

	release := make(chan struct{})
	loadDone := make(chan struct{})
	go func() {
		defer close(loadDone)
		_, _, _ = c.LoadStill(context.Background(), path, mtime, size, 64,
			func(context.Context) ([]byte, string, error) {
				<-release
				return []byte("still"), "", nil
			})
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !c.InFlight(path) {
		time.Sleep(time.Millisecond)
	}
	if !c.InFlight(path) {
		t.Fatal("timed out waiting for LoadStill to become in-flight")
	}

	e.Schedule([]Item{item}, 0, 0, 5, box)
	if len(e.pending) != 0 {
		t.Fatalf("pending = %+v, want empty while item is in-flight", e.pending)
	}

	close(release)
	<-loadDone
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && c.InFlight(path) {
		time.Sleep(time.Millisecond)
	}
	if c.InFlight(path) {
		t.Fatal("timed out waiting for in-flight to clear")
	}

	// Still-PNG tier now warm, but render tier for this box was never loaded: item must be
	// re-included, same as TestScheduleRequeuesStillWarmItemWhenRenderColdForBox.
	e.Schedule([]Item{item}, 0, 0, 5, box)
	if len(e.pending) != 1 || e.pending[0].Path != path {
		t.Fatalf("pending = %+v, want %q re-included now that in-flight has cleared and render tier is still cold", e.pending, path)
	}
}

func TestScheduleFromListingExcludesDeadZoneBetweenCursorAndLandingWindows(t *testing.T) {
	e := &Engine{cache: NewCache(1<<20, 0, 1<<20, "")}
	e.cond = sync.NewCond(&e.mu)

	const total = 50
	const cursorIdx = 25
	const window = 2
	const pageSize = 20
	entries := make([]localfs.Entry, total)
	for i := 0; i < total; i++ {
		entries[i] = localfs.Entry{
			Path: prefetchTestWords[i%len(prefetchTestWords)] + ".png",
			Type: localfs.EntryFile,
		}
	}
	e.ScheduleFromListing(entries, cursorIdx, window, 0, pageSize, nil)

	got := make(map[int]bool, len(e.pending))
	for _, it := range e.pending {
		got[it.Offset] = true
	}
	for offset := -window; offset <= window; offset++ {
		if !got[offset] {
			t.Errorf("expected cursor window offset %d to be scheduled", offset)
		}
	}
	// Exact landing offsets (tier 2) are always scheduled alongside near-cursor work.
	for _, landing := range []int{pageSize, -pageSize} {
		if !got[landing] {
			t.Errorf("expected exact landing offset %d to be scheduled", landing)
		}
	}
	deadZoneOffsets := []int{window + 1, pageSize - window - 1, -(window + 1), -(pageSize - window - 1)}
	for _, offset := range deadZoneOffsets {
		if got[offset] {
			t.Errorf("dead-zone offset %d should not be scheduled", offset)
		}
	}
	// Landing-vicinity offsets (tier 5, excluding the exact landing offset itself) are gated out
	// here since near-cursor work (the cursor window) is outstanding in this same Schedule call.
	for _, landing := range []int{pageSize, -pageSize} {
		for offset := landing - window; offset <= landing+window; offset++ {
			if offset == landing {
				continue
			}
			if got[offset] {
				t.Errorf("landing-vicinity offset %d should be gated out while near-cursor work is outstanding", offset)
			}
		}
	}
}

func TestScheduleFromListingWithoutPageSizeDoesNotPanic(t *testing.T) {
	e := &Engine{cache: NewCache(1<<20, 0, 1<<20, "")}
	e.cond = sync.NewCond(&e.mu)

	entries := []localfs.Entry{
		{Path: prefetchTestWords[0] + ".png", Type: localfs.EntryFile},
		{Path: prefetchTestWords[1] + ".png", Type: localfs.EntryFile},
		{Path: prefetchTestWords[2] + ".png", Type: localfs.EntryFile},
	}
	// pageSize 0: no landing band, only the cursor window applies.
	e.ScheduleFromListing(entries, 1, 5, 0, 0, nil)
	if len(e.pending) != 3 {
		t.Fatalf("pending len = %d, want 3 with pageSize 0", len(e.pending))
	}

	// pageSize landing band past both ends of the listing: yields no extra items, no panic.
	e.ScheduleFromListing(entries, 1, 1, 0, 1000, nil)
	if len(e.pending) != 3 {
		t.Fatalf("pending len = %d, want 3 with out-of-range landing band", len(e.pending))
	}
}

// waitRenderWarm polls until key is present in the render cache (or fails the test on timeout),
// proving runJob's KindImage case warmed Cache.LoadRender in the background rather than the
// caller having to trigger it synchronously.
func waitRenderWarm(t *testing.T, e *Engine, key string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, ok := e.cache.render.get(key); ok {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for render cache to warm for key %q", key)
}

func TestScheduleWithRenderBoxWarmsRenderCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meadow.png")
	writeSolidPNG(t, path, 40, 40)
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	mtime, size := fi.ModTime().UnixNano(), fi.Size()

	e := NewEngine(context.Background(), Config{Workers: 1, ImageMaxEdgePx: 64})
	t.Cleanup(e.Close)

	box := &RenderBox{Proto: previewpanel.ImageProtocolKitty, MaxPxW: 20, MaxPxH: 20}
	e.Schedule([]Item{{Path: path, Kind: KindImage, Mtime: mtime, Size: size}}, 0, 0, 5, box)

	key := renderKey(path, mtime, size, box.Proto, box.UnicodePlaceholder, box.InTmux, box.MaxPxW, box.MaxPxH)
	waitRenderWarm(t, e, key)

	// Prove it's actually served from cache: a load callback that fails the test if invoked.
	payload, _, _, err := e.cache.LoadRender(context.Background(), path, mtime, size,
		box.Proto, box.UnicodePlaceholder, box.InTmux, box.MaxPxW, box.MaxPxH,
		func(context.Context) ([]byte, int, int, error) {
			t.Fatal("LoadRender load callback invoked; expected the prefetch worker to have warmed the cache already")
			return nil, 0, 0, nil
		})
	if err != nil || len(payload) == 0 {
		t.Fatalf("LoadRender after warm: payload len=%d err=%v", len(payload), err)
	}
}

func TestScheduleWithNilRenderBoxSkipsRenderWarming(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "harbor.png")
	writeSolidPNG(t, path, 40, 40)
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	mtime, size := fi.ModTime().UnixNano(), fi.Size()

	e := NewEngine(context.Background(), Config{Workers: 1, ImageMaxEdgePx: 64})
	t.Cleanup(e.Close)

	e.Schedule([]Item{{Path: path, Kind: KindImage, Mtime: mtime, Size: size}}, 0, 0, 5, nil)

	// Wait for the still (maxEdge PNG) cache to warm, which happens unconditionally — a stand-in
	// for the job having finished, since there's nothing render-side to poll for here.
	stillDeadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(stillDeadline) && !e.cache.HasStill(path, mtime, size, 64) {
		time.Sleep(time.Millisecond)
	}
	if !e.cache.HasStill(path, mtime, size, 64) {
		t.Fatal("timed out waiting for still cache to warm")
	}

	key := renderKey(path, mtime, size, previewpanel.ImageProtocolKitty, false, false, 20, 20)
	if _, _, ok := e.cache.render.get(key); ok {
		t.Fatal("render cache warmed despite nil RenderBox; expected eager warming to be skipped")
	}
}
