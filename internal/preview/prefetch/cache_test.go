package prefetch

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func TestMemoryLRUEvictsByByteBudget(t *testing.T) {
	m := newMemoryLRU(100)
	pngA := bytes.Repeat([]byte("a"), 60)
	pngB := bytes.Repeat([]byte("b"), 60)
	pngC := bytes.Repeat([]byte("c"), 60)
	m.put("a", pngA, "meta-a")
	m.put("b", pngB, "meta-b")
	if m.has("a") {
		t.Fatal("expected a evicted after b filled budget")
	}
	if !m.has("b") {
		t.Fatal("expected b present")
	}
	m.put("c", pngC, "meta-c")
	if m.has("b") {
		t.Fatal("expected b evicted after c")
	}
	got, meta, ok := m.get("c")
	if !ok || meta != "meta-c" || len(got) != 60 {
		t.Fatalf("get c = ok=%v meta=%q len=%d", ok, meta, len(got))
	}
}

func TestDiskCacheRoundTripAndEvict(t *testing.T) {
	dir := t.TempDir()
	d := newDiskCache(dir, 80)
	key := videoDiskKey("/tmp/movie.mp4", 1, 2, 1024, 2, 2)
	payload := bytes.Repeat([]byte("x"), 50)
	if err := d.put(key, payload); err != nil {
		t.Fatal(err)
	}
	got, ok := d.get(key)
	if !ok || !bytes.Equal(got, payload) {
		t.Fatalf("get mismatch ok=%v", ok)
	}
	key2 := videoDiskKey("/tmp/other.mp4", 1, 2, 1024, 2, 2)
	if err := d.put(key2, bytes.Repeat([]byte("y"), 50)); err != nil {
		t.Fatal(err)
	}
	// Cap 80: one of the ~50-byte files should be gone after second put.
	_, ok1 := d.get(key)
	_, ok2 := d.get(key2)
	if ok1 && ok2 {
		t.Fatal("expected disk eviction of older entry")
	}
	if !ok2 {
		t.Fatal("expected newest key2 to remain")
	}
}

func TestCacheLoadStillSingleflight(t *testing.T) {
	c := NewCache(1024*1024, 1024*1024, 1024*1024, "")
	calls := 0
	load := func(context.Context) ([]byte, string, error) {
		calls++
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "META", nil
	}
	png1, meta1, err := c.LoadStill(context.Background(), "/a.png", 1, 10, 64, load)
	if err != nil || meta1 != "META" || len(png1) == 0 {
		t.Fatalf("first load: err=%v meta=%q", err, meta1)
	}
	png2, meta2, err := c.LoadStill(context.Background(), "/a.png", 1, 10, 64, load)
	if err != nil || meta2 != "META" || !bytes.Equal(png1, png2) {
		t.Fatalf("second load: err=%v", err)
	}
	if calls != 1 {
		t.Fatalf("load calls = %d, want 1", calls)
	}
}

// TestCacheLoadVideoBroadcastsProgressToJoiningCaller guards against a bug where a caller
// joining an already-running LoadVideo flight (e.g. the foreground preview panel opening a
// video the background prefetch engine already started generating thumbnails for) never saw
// progress updates, because only the flight leader's own load closure ran.
func TestCacheLoadVideoBroadcastsProgressToJoiningCaller(t *testing.T) {
	c := NewCache(1024*1024, 1024*1024, 1024*1024, "")
	key := videoKey("/clip.mp4", 1, 10, 64, 2, 2)

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	pngBytes := buf.Bytes()

	proceed := make(chan struct{})
	leaderLoad := func(ctx context.Context, notify func(done, total int)) ([]byte, error) {
		<-proceed
		notify(1, 2)
		notify(2, 2)
		return pngBytes, nil
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := c.LoadVideo(context.Background(), "/clip.mp4", 1, 10, 64, 2, 2, nil, leaderLoad); err != nil {
			t.Errorf("leader LoadVideo: %v", err)
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	waitFor := func(cond func() bool) {
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatal("timed out waiting for flight state")
			}
			time.Sleep(time.Millisecond)
		}
	}
	waitFor(func() bool {
		c.mu.Lock()
		_, ok := c.flights[key]
		c.mu.Unlock()
		return ok
	})

	var mu sync.Mutex
	var joinerProgress [][2]int
	go func() {
		defer wg.Done()
		joinerLoad := func(context.Context, func(done, total int)) ([]byte, error) {
			t.Error("joiner must not run its own load closure while the leader's flight is in progress")
			return nil, nil
		}
		onProgress := func(done, total int) {
			mu.Lock()
			joinerProgress = append(joinerProgress, [2]int{done, total})
			mu.Unlock()
		}
		if _, err := c.LoadVideo(context.Background(), "/clip.mp4", 1, 10, 64, 2, 2, onProgress, joinerLoad); err != nil {
			t.Errorf("joiner LoadVideo: %v", err)
		}
	}()

	waitFor(func() bool {
		c.mu.Lock()
		call, ok := c.flights[key]
		c.mu.Unlock()
		if !ok {
			return true
		}
		call.listenersMu.Lock()
		n := len(call.listeners)
		call.listenersMu.Unlock()
		return n >= 1
	})
	close(proceed)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	want := [][2]int{{1, 2}, {2, 2}}
	if len(joinerProgress) != len(want) || joinerProgress[0] != want[0] || joinerProgress[1] != want[1] {
		t.Fatalf("joinerProgress = %v, want %v", joinerProgress, want)
	}
}

// TestCacheLoadStillCachesFailure guards against a decode-fail-reschedule loop: a
// permanently corrupt file must only be attempted once, and HasStill must report it as
// warm afterward so the prefetch engine's Schedule() stops re-queuing it forever.
func TestCacheLoadStillCachesFailure(t *testing.T) {
	c := NewCache(1024*1024, 1024*1024, 1024*1024, "")
	calls := 0
	failLoad := func(context.Context) ([]byte, string, error) {
		calls++
		return nil, "", fmt.Errorf("corrupt png")
	}
	if _, _, err := c.LoadStill(context.Background(), "/bad.png", 1, 10, 64, failLoad); err == nil {
		t.Fatal("expected error from first load")
	}
	if _, _, err := c.LoadStill(context.Background(), "/bad.png", 1, 10, 64, failLoad); err == nil {
		t.Fatal("expected error from second load")
	}
	if calls != 1 {
		t.Fatalf("load calls = %d, want 1 (failure must be cached, not retried)", calls)
	}
	if !c.HasStill("/bad.png", 1, 10, 64) {
		t.Fatal("expected HasStill to report warm for a permanently failed decode")
	}
}

// TestCacheLoadStillDoesNotCacheCancellation ensures a context-cancelled attempt (the
// request was abandoned, not proven broken) is retried rather than marked failed forever.
func TestCacheLoadStillDoesNotCacheCancellation(t *testing.T) {
	c := NewCache(1024*1024, 1024*1024, 1024*1024, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	load := func(context.Context) ([]byte, string, error) {
		calls++
		return nil, "", ctx.Err()
	}
	if _, _, err := c.LoadStill(ctx, "/cancelled.png", 1, 10, 64, load); err == nil {
		t.Fatal("expected error from cancelled load")
	}
	if c.HasStill("/cancelled.png", 1, 10, 64) {
		t.Fatal("cancellation must not be recorded as a permanent failure")
	}
}

func TestCacheLoadRenderCachesHit(t *testing.T) {
	c := NewCache(1024*1024, 1024*1024, 1024*1024, "")
	calls := 0
	load := func(context.Context) ([]byte, int, int, error) {
		calls++
		return []byte("payload-lighthouse"), 12, 8, nil
	}
	payload1, w1, h1, err := c.LoadRender(context.Background(), "/lighthouse.png", 1, 10,
		previewpanel.ImageProtocolKitty, false, false, 100, 100, load)
	if err != nil || w1 != 12 || h1 != 8 || string(payload1) != "payload-lighthouse" {
		t.Fatalf("first load: payload=%q w=%d h=%d err=%v", payload1, w1, h1, err)
	}
	payload2, w2, h2, err := c.LoadRender(context.Background(), "/lighthouse.png", 1, 10,
		previewpanel.ImageProtocolKitty, false, false, 100, 100, load)
	if err != nil || w2 != 12 || h2 != 8 || !bytes.Equal(payload1, payload2) {
		t.Fatalf("second load: payload=%q w=%d h=%d err=%v", payload2, w2, h2, err)
	}
	if calls != 1 {
		t.Fatalf("load calls = %d, want 1 (second call should be a cache hit)", calls)
	}
}

func TestCacheLoadRenderKeysDoNotCollideAcrossBoxes(t *testing.T) {
	c := NewCache(1024*1024, 1024*1024, 1024*1024, "")
	loadFor := func(w, h int) func(context.Context) ([]byte, int, int, error) {
		return func(context.Context) ([]byte, int, int, error) {
			return []byte(fmt.Sprintf("payload-%dx%d", w, h)), w, h, nil
		}
	}
	pA, wA, hA, err := c.LoadRender(context.Background(), "/harbor.png", 1, 10,
		previewpanel.ImageProtocolKitty, false, false, 50, 50, loadFor(50, 40))
	if err != nil || wA != 50 || hA != 40 {
		t.Fatalf("box A load: payload=%q w=%d h=%d err=%v", pA, wA, hA, err)
	}
	pB, wB, hB, err := c.LoadRender(context.Background(), "/harbor.png", 1, 10,
		previewpanel.ImageProtocolKitty, false, false, 100, 100, loadFor(100, 80))
	if err != nil || wB != 100 || hB != 80 {
		t.Fatalf("box B load: payload=%q w=%d h=%d err=%v", pB, wB, hB, err)
	}
	if bytes.Equal(pA, pB) {
		t.Fatalf("expected distinct payloads for distinct boxes, got %q for both", pA)
	}
}

func TestCacheHasRenderReportsWarmthForExactBoxOnly(t *testing.T) {
	c := NewCache(1024*1024, 1024*1024, 1024*1024, "")
	if c.HasRender("/orchard.png", 1, 10, previewpanel.ImageProtocolKitty, false, false, 100, 100) {
		t.Fatal("expected HasRender false before LoadRender")
	}
	load := func(context.Context) ([]byte, int, int, error) {
		return []byte("payload-orchard"), 12, 8, nil
	}
	if _, _, _, err := c.LoadRender(context.Background(), "/orchard.png", 1, 10,
		previewpanel.ImageProtocolKitty, false, false, 100, 100, load); err != nil {
		t.Fatal(err)
	}
	if !c.HasRender("/orchard.png", 1, 10, previewpanel.ImageProtocolKitty, false, false, 100, 100) {
		t.Fatal("expected HasRender true after LoadRender")
	}
	// A different box (unrelated to the one just warmed) must still report cold.
	if c.HasRender("/orchard.png", 1, 10, previewpanel.ImageProtocolKitty, false, false, 200, 200) {
		t.Fatal("expected HasRender false for a different box")
	}
}

func TestDefaultDirUsesUserCache(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)
	// os.UserCacheDir on Linux uses XDG_CACHE_HOME
	dir, err := DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "pc", "video-thumbs")
	if dir != want {
		// Some platforms ignore XDG_CACHE_HOME; still require …/pc/video-thumbs suffix.
		if filepath.Base(filepath.Dir(dir)) != "pc" || filepath.Base(dir) != "video-thumbs" {
			t.Fatalf("DefaultDir = %q, want under pc/video-thumbs (got want %q)", dir, want)
		}
	}
	_ = os.MkdirAll(dir, 0o755)
}
