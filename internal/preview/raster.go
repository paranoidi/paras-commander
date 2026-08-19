package preview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// errImageTooLarge is returned (with meta already annotated) when decode is skipped for size.
var errImageTooLarge = fmt.Errorf("image too large")

// DecodeStillMaxEdgePNG decodes path, clamps the longest edge to maxEdge, and returns PNG bytes
// plus the metadata caption string used by still-image previews.
func DecodeStillMaxEdgePNG(ctx context.Context, path string, maxEdge int) (pngBytes []byte, meta string, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}

	if localfs.IsImageMagickPath(path) {
		converted, err := convertToPNGViaImageMagick(ctx, path)
		if err != nil {
			return nil, "", err
		}
		img, err := DecodePNGBytes(converted)
		if err != nil {
			return nil, "", err
		}
		b := img.Bounds()
		format := strings.ToUpper(strings.TrimPrefix(filepath.Ext(path), "."))
		meta = formatImageMeta(format, b.Dx(), b.Dy(), fi.Size())
		if maxEdge > 0 {
			img = fitImage(img, maxEdge, maxEdge)
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), meta, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = f.Close() }()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, "", err
	}
	meta = formatImageMeta(format, cfg.Width, cfg.Height, fi.Size())

	pixels := int64(cfg.Width) * int64(cfg.Height)
	maxPixels := int64(config.DefaultPreviewImageMaxDecodeMegapixels) * 1_000_000
	if pixels > maxPixels {
		return nil, meta + " / too large", errImageTooLarge
	}

	if _, err := f.Seek(0, 0); err != nil {
		return nil, "", err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, "", err
	}
	if maxEdge > 0 {
		img = fitImage(img, maxEdge, maxEdge)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), meta, nil
}

// BuildVideoThumbMaxEdgePNG extracts and composites a video thumb grid, clamping the grid to
// maxEdge×maxEdge before encoding as PNG.
func BuildVideoThumbMaxEdgePNG(ctx context.Context, path string, durationSec float64, cols, rows, maxEdge int, onFrame func(done, total int)) ([]byte, error) {
	if maxEdge < 1 {
		maxEdge = 1
	}
	grid, err := buildVideoThumbGrid(ctx, path, durationSec, cols, rows, maxEdge, maxEdge, onFrame)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, grid); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodePNGBytes decodes a PNG raster from bytes.
func DecodePNGBytes(b []byte) (image.Image, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("empty png")
	}
	return png.Decode(bytes.NewReader(b))
}
