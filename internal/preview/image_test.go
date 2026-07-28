package preview

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func writeTestPNG(t *testing.T, path string, w, h int) {
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

func TestRunImageEncodesSixelWithinBudget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "garden.png")
	writeTestPNG(t, path, 40, 30)

	res := Run(context.Background(), Request{
		Path:          path,
		Preview:       config.PreviewConfig{Images: true},
		Image:         true,
		ImageMaxPxW:   20,
		ImageMaxPxH:   15,
		ImageProtocol: previewpanel.ImageProtocolSixel,
	})
	if res.ErrorMsg != "" {
		t.Fatalf("ErrorMsg = %q", res.ErrorMsg)
	}
	if !strings.HasPrefix(res.ImagePayload, "\x1bP") {
		t.Fatalf("ImagePayload prefix = %q, want \\x1bP…", res.ImagePayload[:min(8, len(res.ImagePayload))])
	}
	if res.ImageProtocol != previewpanel.ImageProtocolSixel {
		t.Fatalf("ImageProtocol = %v, want Sixel", res.ImageProtocol)
	}
	if res.ImagePxW > 20 || res.ImagePxH > 15 {
		t.Fatalf("scaled = %d×%d, want ≤ 20×15", res.ImagePxW, res.ImagePxH)
	}
	if res.CombinedText != "" {
		t.Fatalf("CombinedText = %q, want empty on successful still-image encode", res.CombinedText)
	}
}

// writeNoisyTestPNG writes a high-entropy (per-pixel random color) PNG — the worst case for
// sixel's run-length encoding, since adjacent pixels are almost never identical.
func writeNoisyTestPNG(t *testing.T, path string, w, h int) {
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

func TestRunImageSixelUnderTmuxUsesSmallerPalette(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noisy.png")
	writeNoisyTestPNG(t, path, 300, 300)

	base := Request{
		Path:          path,
		Preview:       config.PreviewConfig{Images: true},
		Image:         true,
		ImageMaxPxW:   300,
		ImageMaxPxH:   300,
		ImageProtocol: previewpanel.ImageProtocolSixel,
	}

	direct := Run(context.Background(), base)
	if direct.ErrorMsg != "" || direct.ImagePayload == "" {
		t.Fatalf("direct: ErrorMsg = %q, payload len = %d", direct.ErrorMsg, len(direct.ImagePayload))
	}

	tmuxReq := base
	tmuxReq.ImageInTmux = true
	underTmux := Run(context.Background(), tmuxReq)
	if underTmux.ErrorMsg != "" || underTmux.ImagePayload == "" {
		t.Fatalf("under tmux: ErrorMsg = %q, payload len = %d", underTmux.ErrorMsg, len(underTmux.ImagePayload))
	}

	// A 64-color palette compresses a high-entropy image meaningfully better than the default
	// 256-color one, via longer runs of repeated palette indices.
	if len(underTmux.ImagePayload) >= len(direct.ImagePayload) {
		t.Fatalf("tmux payload len = %d, want < direct payload len = %d", len(underTmux.ImagePayload), len(direct.ImagePayload))
	}
}

func TestRunImageSixelUnderTmuxFallsBackWhenTooLargeForBuffer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge-noisy.png")
	// Large enough and noisy enough that even the reduced 64-color tmux palette can't bring
	// the encoded payload under config.DefaultPreviewTmuxSixelMaxBytes.
	writeNoisyTestPNG(t, path, 700, 700)

	res := Run(context.Background(), Request{
		Path:          path,
		Preview:       config.PreviewConfig{Images: true},
		Image:         true,
		ImageMaxPxW:   700,
		ImageMaxPxH:   700,
		ImageProtocol: previewpanel.ImageProtocolSixel,
		ImageInTmux:   true,
	})
	if res.ErrorMsg != "" {
		t.Fatalf("ErrorMsg = %q", res.ErrorMsg)
	}
	if res.ImagePayload != "" {
		t.Fatalf("ImagePayload len = %d, want empty (metadata fallback)", len(res.ImagePayload))
	}
	if !strings.Contains(res.CombinedText, "too large for tmux") {
		t.Fatalf("CombinedText = %q, want %q", res.CombinedText, "too large for tmux")
	}
}

func TestRunImageEncodesKittyAPC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "harbor.png")
	writeTestPNG(t, path, 24, 18)

	res := Run(context.Background(), Request{
		Path:          path,
		Preview:       config.PreviewConfig{Images: true},
		Image:         true,
		ImageMaxPxW:   100,
		ImageMaxPxH:   100,
		ImageProtocol: previewpanel.ImageProtocolKitty,
	})
	if res.ErrorMsg != "" {
		t.Fatalf("ErrorMsg = %q", res.ErrorMsg)
	}
	if !strings.HasPrefix(res.ImagePayload, "\x1b_G") {
		t.Fatalf("ImagePayload prefix = %q, want \\x1b_G…", res.ImagePayload[:min(8, len(res.ImagePayload))])
	}
	if !strings.Contains(res.ImagePayload, "a=T") || !strings.Contains(res.ImagePayload, "f=100") {
		t.Fatalf("ImagePayload missing a=T/f=100: %q", res.ImagePayload[:min(80, len(res.ImagePayload))])
	}
	if res.ImageProtocol != previewpanel.ImageProtocolKitty {
		t.Fatalf("ImageProtocol = %v, want Kitty", res.ImageProtocol)
	}
}

func TestRunImageEncodesKittyAPCWithUnicodePlaceholder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quarry.png")
	writeTestPNG(t, path, 24, 18)

	res := Run(context.Background(), Request{
		Path:                    path,
		Preview:                 config.PreviewConfig{Images: true},
		Image:                   true,
		ImageMaxPxW:             100,
		ImageMaxPxH:             100,
		ImageProtocol:           previewpanel.ImageProtocolKitty,
		ImageUnicodePlaceholder: true,
	})
	if res.ErrorMsg != "" {
		t.Fatalf("ErrorMsg = %q", res.ErrorMsg)
	}
	head := res.ImagePayload[:min(120, len(res.ImagePayload))]
	if !strings.Contains(res.ImagePayload, "U=1") {
		t.Fatalf("ImagePayload missing U=1: %q", head)
	}
	if strings.Contains(res.ImagePayload, "C=1") {
		t.Fatalf("ImagePayload has cursor-relative C=1 alongside U=1: %q", head)
	}
	if !strings.Contains(res.ImagePayload, "a=T") || !strings.Contains(res.ImagePayload, "f=100") {
		t.Fatalf("ImagePayload missing a=T/f=100: %q", head)
	}
}

func TestRunImageDisabledReturnsMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meadow.png")
	writeTestPNG(t, path, 16, 12)

	res := Run(context.Background(), Request{
		Path:          path,
		Preview:       config.PreviewConfig{Images: false},
		Image:         true,
		ImageMaxPxW:   100,
		ImageMaxPxH:   100,
		ImageProtocol: previewpanel.ImageProtocolSixel,
	})
	if res.ErrorMsg != "" {
		t.Fatalf("ErrorMsg = %q", res.ErrorMsg)
	}
	if res.ImagePayload != "" {
		t.Fatalf("ImagePayload = %q, want empty when images disabled", res.ImagePayload)
	}
	if !strings.Contains(res.CombinedText, "PNG") || !strings.Contains(res.CombinedText, "16 × 12") {
		t.Fatalf("CombinedText = %q, want format+dims", res.CombinedText)
	}
}

func TestRunImageProtocolNoneReturnsMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stone.png")
	writeTestPNG(t, path, 8, 8)

	res := Run(context.Background(), Request{
		Path:          path,
		Preview:       config.PreviewConfig{Images: true},
		Image:         true,
		ImageMaxPxW:   100,
		ImageMaxPxH:   100,
		ImageProtocol: previewpanel.ImageProtocolNone,
	})
	if res.ImagePayload != "" {
		t.Fatalf("ImagePayload = %q, want empty for None protocol", res.ImagePayload)
	}
}
