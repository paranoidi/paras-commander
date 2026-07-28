package prefetch

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
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
	c := NewCache(1024*1024, 1024*1024, "")
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
