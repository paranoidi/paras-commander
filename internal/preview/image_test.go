package preview

import (
	"context"
	"image"
	"image/color"
	"image/png"
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
	if !strings.Contains(res.CombinedText, "PNG") || !strings.Contains(res.CombinedText, "40 × 30") {
		t.Fatalf("CombinedText = %q, want PNG metadata with native dims", res.CombinedText)
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
