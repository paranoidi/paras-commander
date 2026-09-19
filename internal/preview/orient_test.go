package preview

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"
	"time"
)

// testOrientColor gives every pixel of the 2x3 fixture image a distinct, easily-verified color.
func testOrientColor(x, y int) color.RGBA {
	return color.RGBA{R: uint8(x*100 + 10), G: uint8(y*80 + 5), B: 1, A: 255}
}

func newOrientTestImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 2, 3)) // w=2, h=3
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			img.SetRGBA(x, y, testOrientColor(x, y))
		}
	}
	return img
}

func TestApplyExifOrientation(t *testing.T) {
	type corner struct{ ox, oy, sx, sy int }
	cases := []struct {
		orientation int
		dw, dh      int
		corners     []corner
	}{
		{1, 2, 3, []corner{{0, 0, 0, 0}, {1, 0, 1, 0}, {0, 2, 0, 2}, {1, 2, 1, 2}}},
		{2, 2, 3, []corner{{0, 0, 1, 0}, {1, 0, 0, 0}, {0, 2, 1, 2}, {1, 2, 0, 2}}},
		{3, 2, 3, []corner{{0, 0, 1, 2}, {1, 0, 0, 2}, {0, 2, 1, 0}, {1, 2, 0, 0}}},
		{4, 2, 3, []corner{{0, 0, 0, 2}, {1, 0, 1, 2}, {0, 2, 0, 0}, {1, 2, 1, 0}}},
		{5, 3, 2, []corner{{0, 0, 0, 0}, {2, 0, 0, 2}, {0, 1, 1, 0}, {2, 1, 1, 2}}},
		{6, 3, 2, []corner{{0, 0, 0, 2}, {2, 0, 0, 0}, {0, 1, 1, 2}, {2, 1, 1, 0}}},
		{7, 3, 2, []corner{{0, 0, 1, 2}, {2, 0, 1, 0}, {0, 1, 0, 2}, {2, 1, 0, 0}}},
		{8, 3, 2, []corner{{0, 0, 1, 0}, {2, 0, 1, 2}, {0, 1, 0, 0}, {2, 1, 0, 2}}},
	}

	for _, tc := range cases {
		src := newOrientTestImage()
		got := applyExifOrientation(src, tc.orientation)
		b := got.Bounds()
		if b.Dx() != tc.dw || b.Dy() != tc.dh {
			t.Fatalf("orientation %d: dims = %dx%d, want %dx%d", tc.orientation, b.Dx(), b.Dy(), tc.dw, tc.dh)
		}
		for _, c := range tc.corners {
			want := testOrientColor(c.sx, c.sy)
			gotColor := color.RGBAModel.Convert(got.At(c.ox, c.oy)).(color.RGBA)
			if gotColor != want {
				t.Fatalf("orientation %d: pixel (%d,%d) = %+v, want %+v (source (%d,%d))",
					tc.orientation, c.ox, c.oy, gotColor, want, c.sx, c.sy)
			}
		}
	}
}

func TestApplyExifOrientationUnknownReturnsUnchanged(t *testing.T) {
	src := newOrientTestImage()
	if got := applyExifOrientation(src, 1); got != image.Image(src) {
		t.Fatalf("orientation 1 should return the same image unchanged")
	}
	if got := applyExifOrientation(src, 0); got != image.Image(src) {
		t.Fatalf("orientation 0 (missing/invalid) should return the same image unchanged")
	}
	if got := applyExifOrientation(src, 9); got != image.Image(src) {
		t.Fatalf("orientation 9 (out of range) should return the same image unchanged")
	}
}

func TestExifOrientation(t *testing.T) {
	dir := t.TempDir()

	rotated := filepath.Join(dir, "rotated.jpg")
	writeTestJPEGWithOrientation(t, rotated, 8, 6, 6)
	if got := exifOrientation(rotated); got != 6 {
		t.Fatalf("exifOrientation(orientation 6 fixture) = %d, want 6", got)
	}

	plain := filepath.Join(dir, "plain.png")
	writeTestPNG(t, plain, 8, 6)
	if got := exifOrientation(plain); got != 1 {
		t.Fatalf("exifOrientation(no exif) = %d, want 1 (normal)", got)
	}
}

func TestDecodeStillMaxEdgePNGAppliesOrientation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rotated.jpg")
	writeTestJPEGWithOrientation(t, path, 8, 6, 6) // w=8, h=6 stored; orientation 6 -> displayed 6x8

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pngBytes, _, err := DecodeStillMaxEdgePNG(ctx, path, 0, "off")
	if err != nil {
		t.Fatalf("DecodeStillMaxEdgePNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 6 || b.Dy() != 8 {
		t.Fatalf("decoded bounds = %dx%d, want 6x8 (transposed by orientation 6)", b.Dx(), b.Dy())
	}
}
