package preview_test

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	previewrun "github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/preview/prefetch"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func writeGradientPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 80, A: 255})
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

func writeNoisyPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	rng := rand.New(rand.NewSource(1))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(rng.Intn(256)), G: uint8(rng.Intn(256)), B: uint8(rng.Intn(256)), A: 255,
			})
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

// TestRunImageServesRepeatVisitsFromRenderCache proves that a second Run() call against an
// identical Request/Cache is served entirely from the render cache: it corrupts the source
// file's pixel data (keeping length and mtime unchanged, so cache keys still match) between
// calls, then confirms the second call still succeeds with a byte-identical payload — which
// would be impossible if it re-decoded the now-corrupt on-disk bytes.
func TestRunImageServesRepeatVisitsFromRenderCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "orchard.png")
	writeGradientPNG(t, path, 60, 45)

	cache := prefetch.NewCache(1<<20, 0, 1<<20, "")
	req := previewrun.Request{
		Path:          path,
		Preview:       config.PreviewConfig{Images: true},
		Image:         true,
		ImageMaxPxW:   30,
		ImageMaxPxH:   22,
		ImageProtocol: previewpanel.ImageProtocolSixel,
		Cache:         cache,
	}

	res1 := previewrun.Run(context.Background(), req)
	if res1.ErrorMsg != "" || res1.ImagePayload == "" {
		t.Fatalf("first Run: ErrorMsg=%q payload len=%d", res1.ErrorMsg, len(res1.ImagePayload))
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	origMtime := fi.ModTime()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := append([]byte(nil), raw...)
	// Flip bytes in the middle of the file (inside the IDAT compressed stream, well clear of
	// the IHDR header at the front and the IEND trailer at the back) so any fresh PNG decode of
	// this file would fail, without changing the file's length.
	start := len(corrupt) / 3
	end := start + 20
	if end > len(corrupt)-12 {
		end = len(corrupt) - 12
	}
	for i := start; i < end; i++ {
		corrupt[i] ^= 0xFF
	}
	if err := os.WriteFile(path, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, origMtime, origMtime); err != nil {
		t.Fatal(err)
	}

	// Sanity check: confirm the corruption is actually effective — a fresh (uncached) decode of
	// the now-corrupt file must not silently succeed with the same payload as before, or this
	// test wouldn't be proving anything.
	freshReq := req
	freshReq.Cache = prefetch.NewCache(1<<20, 0, 1<<20, "")
	resFresh := previewrun.Run(context.Background(), freshReq)
	if resFresh.ErrorMsg == "" && resFresh.ImagePayload == res1.ImagePayload {
		t.Fatal("corruption had no effect on a fresh decode — test setup invalid")
	}

	res2 := previewrun.Run(context.Background(), req)
	if res2.ErrorMsg != "" {
		t.Fatalf("second Run: ErrorMsg = %q, want success served from cache despite corrupted file", res2.ErrorMsg)
	}
	if res2.ImagePayload != res1.ImagePayload {
		t.Fatal("second Run payload differs from first — expected an identical cached render payload")
	}
	if res2.ImagePxW != res1.ImagePxW || res2.ImagePxH != res1.ImagePxH {
		t.Fatalf("second Run box = %dx%d, want %dx%d (cached)", res2.ImagePxW, res2.ImagePxH, res1.ImagePxW, res1.ImagePxH)
	}
}

// TestRunImageRenderCacheIsolatesDistinctBoxes confirms two different pixel boxes for the same
// file are cached independently — no key collision producing a payload sized for the wrong box.
func TestRunImageRenderCacheIsolatesDistinctBoxes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meridian.png")
	writeGradientPNG(t, path, 80, 60)

	cache := prefetch.NewCache(1<<20, 0, 1<<20, "")
	base := previewrun.Request{
		Path:          path,
		Preview:       config.PreviewConfig{Images: true},
		Image:         true,
		ImageProtocol: previewpanel.ImageProtocolSixel,
		Cache:         cache,
	}

	small := base
	small.ImageMaxPxW, small.ImageMaxPxH = 20, 15
	resSmall := previewrun.Run(context.Background(), small)
	if resSmall.ErrorMsg != "" || resSmall.ImagePxW > 20 || resSmall.ImagePxH > 15 {
		t.Fatalf("small box: ErrorMsg=%q px=%dx%d", resSmall.ErrorMsg, resSmall.ImagePxW, resSmall.ImagePxH)
	}

	large := base
	large.ImageMaxPxW, large.ImageMaxPxH = 80, 60
	resLarge := previewrun.Run(context.Background(), large)
	if resLarge.ErrorMsg != "" || resLarge.ImagePxW > 80 || resLarge.ImagePxH > 60 {
		t.Fatalf("large box: ErrorMsg=%q px=%dx%d", resLarge.ErrorMsg, resLarge.ImagePxW, resLarge.ImagePxH)
	}

	if resSmall.ImagePayload == resLarge.ImagePayload {
		t.Fatal("expected distinct payloads for distinct boxes, got the same cached payload for both")
	}
	// Re-run the small box: must still return the small box's own cached payload, not the
	// large box's (which would indicate a render-cache key collision between boxes).
	resSmallAgain := previewrun.Run(context.Background(), small)
	if resSmallAgain.ImagePayload != resSmall.ImagePayload {
		t.Fatal("re-running the small box did not return its own cached payload")
	}
}

// TestRunImageRenderCacheKeepsShrunkTmuxPayload confirms the tmux-sixel shrink-retry result —
// not the pre-shrink oversized encode — is what gets cached and re-served on a second visit.
func TestRunImageRenderCacheKeepsShrunkTmuxPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cascade.png")
	// Large and noisy enough that the initial fit-to-panel-budget encode overflows
	// config.DefaultPreviewTmuxSixelMaxBytes, forcing the shrink-retry loop.
	writeNoisyPNG(t, path, 700, 700)

	cache := prefetch.NewCache(1<<20, 0, 1<<20, "")
	req := previewrun.Request{
		Path:          path,
		Preview:       config.PreviewConfig{Images: true},
		Image:         true,
		ImageMaxPxW:   700,
		ImageMaxPxH:   700,
		ImageProtocol: previewpanel.ImageProtocolSixel,
		ImageInTmux:   true,
		Cache:         cache,
	}

	res1 := previewrun.Run(context.Background(), req)
	if res1.ErrorMsg != "" || res1.ImagePayload == "" {
		t.Fatalf("first Run: ErrorMsg=%q payload len=%d", res1.ErrorMsg, len(res1.ImagePayload))
	}
	if len(res1.ImagePayload) >= config.DefaultPreviewTmuxSixelMaxBytes {
		t.Fatalf("first Run payload len = %d, want < %d (shrunk)", len(res1.ImagePayload), config.DefaultPreviewTmuxSixelMaxBytes)
	}
	if res1.ImagePxW >= 700 || res1.ImagePxH >= 700 {
		t.Fatalf("first Run box = %dx%d, want smaller than unshrunk 700x700", res1.ImagePxW, res1.ImagePxH)
	}

	res2 := previewrun.Run(context.Background(), req)
	if res2.ErrorMsg != "" {
		t.Fatalf("second Run: ErrorMsg = %q", res2.ErrorMsg)
	}
	if res2.ImagePayload != res1.ImagePayload {
		t.Fatal("second Run payload differs — expected the cached shrunk payload to be re-served as-is")
	}
	if res2.ImagePxW != res1.ImagePxW || res2.ImagePxH != res1.ImagePxH {
		t.Fatalf("second Run box = %dx%d, want the cached shrunk box %dx%d", res2.ImagePxW, res2.ImagePxH, res1.ImagePxW, res1.ImagePxH)
	}
}
