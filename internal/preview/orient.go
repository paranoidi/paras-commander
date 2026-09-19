package preview

import (
	"image"

	xdraw "golang.org/x/image/draw"
)

// applyExifOrientation returns img transformed per the EXIF Orientation tag (1-8) so it displays
// upright: 1 (normal) or any value outside 2-8 returns img unchanged. Orientations 5-8 swap width
// and height (portrait <-> landscape).
//
// ponytail: plain per-pixel loop over a copied *image.RGBA — fine at preview sizes (this always
// runs after the max-edge clamp); revisit with strided row copies if ever measured slow.
func applyExifOrientation(img image.Image, orientation int) image.Image {
	if orientation < 2 || orientation > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.Copy(src, image.Point{}, img, b, xdraw.Src, nil)

	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))

	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var sx, sy int
			switch orientation {
			case 2: // flip horizontal
				sx, sy = w-1-x, y
			case 3: // rotate 180
				sx, sy = w-1-x, h-1-y
			case 4: // flip vertical
				sx, sy = x, h-1-y
			case 5: // transpose (main diagonal)
				sx, sy = y, x
			case 6: // rotate 90 clockwise
				sx, sy = y, h-1-x
			case 7: // transverse (anti-diagonal): flip horizontal, then rotate 90 clockwise
				sx, sy = w-1-y, h-1-x
			case 8: // rotate 90 counter-clockwise
				sx, sy = w-1-y, x
			}
			dst.SetRGBA(x, y, src.RGBAAt(sx, sy))
		}
	}
	return dst
}
