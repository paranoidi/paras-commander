package preview

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestConvertToPNGViaImageMagickRequiresBinary(t *testing.T) {
	emptyPath := t.TempDir()
	t.Setenv("PATH", emptyPath)

	_, err := convertToPNGViaImageMagick(context.Background(), "whatever.psd")
	if !errors.Is(err, ErrImageMagickRequired) {
		t.Fatalf("err = %v, want ErrImageMagickRequired", err)
	}
}

func TestConvertToPNGViaImageMagickRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("magick"); err != nil {
		if _, err := exec.LookPath("convert"); err != nil {
			t.Skip("neither magick nor convert on PATH")
		}
	}

	dir := t.TempDir()
	fixture := filepath.Join(dir, "meadow.png")
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.Set(x, y, color.RGBA{R: uint8(x * 60), G: uint8(y * 60), B: 200, A: 255})
		}
	}
	f, err := os.Create(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	out, err := convertToPNGViaImageMagick(context.Background(), fixture)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePNGBytes(out)
	if err != nil {
		t.Fatalf("decode converted bytes: %v", err)
	}
	if b := decoded.Bounds(); b.Dx() != 4 || b.Dy() != 4 {
		t.Fatalf("decoded bounds = %v, want 4x4", b)
	}
}
