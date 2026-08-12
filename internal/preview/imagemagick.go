package preview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// ErrImageMagickRequired is returned when a preview needs ImageMagick to convert an
// unsupported image format and no magick/convert binary is found on PATH.
var ErrImageMagickRequired = errors.New("ImageMagick required to preview this file type")

func imageMagickBinary() (string, error) {
	for _, name := range []string{"magick", "convert"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", ErrImageMagickRequired
}

// convertToPNGViaImageMagick shells out to ImageMagick to rasterize path's first
// layer/page to PNG bytes. Formats routed here (see localfs.IsImageMagickPath) are ones
// Go's image package cannot decode natively.
func convertToPNGViaImageMagick(ctx context.Context, path string) ([]byte, error) {
	bin, err := imageMagickBinary()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, path+"[0]", "png:-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("imagemagick convert: %w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}
