package preview

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"

	"github.com/mattn/go-sixel"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
	xdraw "golang.org/x/image/draw"

	_ "image/gif"  // register GIF decoder (first frame)
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder

	_ "golang.org/x/image/bmp"  // register BMP decoder
	_ "golang.org/x/image/tiff" // register TIFF decoder
	_ "golang.org/x/image/webp" // register WebP decoder
)

const kittyChunkSize = 4096

func runImage(req Request) Result {
	fi, err := os.Stat(req.Path)
	if err != nil {
		return Result{ErrorMsg: err.Error()}
	}
	f, err := os.Open(req.Path)
	if err != nil {
		return Result{ErrorMsg: err.Error()}
	}
	defer func() { _ = f.Close() }()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return Result{ErrorMsg: err.Error()}
	}
	meta := formatImageMeta(format, cfg.Width, cfg.Height, fi.Size())
	metaResult := Result{
		Source:       previewpanel.SourceExternalANSI,
		CombinedText: meta,
	}

	megapixels := int64(cfg.Width) * int64(cfg.Height)
	maxPixels := int64(config.DefaultPreviewImageMaxDecodeMegapixels) * 1_000_000
	if megapixels > maxPixels {
		metaResult.CombinedText = meta + " / too large"
		return metaResult
	}
	if !req.Preview.Images || req.ImageMaxPxW < 1 || req.ImageMaxPxH < 1 ||
		req.ImageProtocol == previewpanel.ImageProtocolNone {
		return metaResult
	}

	if _, err := f.Seek(0, 0); err != nil {
		return Result{ErrorMsg: err.Error()}
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return Result{ErrorMsg: err.Error()}
	}

	scaled := fitImage(img, req.ImageMaxPxW, req.ImageMaxPxH)
	bounds := scaled.Bounds()
	payload, err := encodeImagePayload(scaled, req.ImageProtocol)
	if err != nil || payload == "" {
		return metaResult
	}
	return Result{
		Source:        previewpanel.SourceExternalANSI,
		CombinedText:  meta,
		ImagePayload:  payload,
		ImagePxW:      bounds.Dx(),
		ImagePxH:      bounds.Dy(),
		ImageProtocol: req.ImageProtocol,
	}
}

func encodeImagePayload(img image.Image, proto previewpanel.ImageProtocol) (string, error) {
	switch proto {
	case previewpanel.ImageProtocolKitty:
		return encodeKittyAPC(img)
	case previewpanel.ImageProtocolSixel:
		var buf bytes.Buffer
		if err := sixel.NewEncoder(&buf).Encode(img); err != nil {
			return "", err
		}
		return buf.String(), nil
	default:
		return "", fmt.Errorf("unsupported image protocol %d", proto)
	}
}

// encodeKittyAPC builds a chunked Kitty graphics transmit+place sequence (f=100 PNG).
func encodeKittyAPC(img image.Image) (string, error) {
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(pngBuf.Bytes())
	if encoded == "" {
		return "", fmt.Errorf("empty png")
	}

	var out strings.Builder
	first := true
	for len(encoded) > 0 {
		n := kittyChunkSize
		if n > len(encoded) {
			n = len(encoded)
		}
		// Non-final chunks must be a multiple of 4 bytes.
		if n < len(encoded) {
			n -= n % 4
			if n == 0 {
				n = len(encoded)
			}
		}
		chunk := encoded[:n]
		encoded = encoded[n:]
		more := 1
		if len(encoded) == 0 {
			more = 0
		}
		if first {
			fmt.Fprintf(&out, "\x1b_Ga=T,f=100,i=%d,C=1,q=2,m=%d;%s\x1b\\",
				previewpanel.KittyGraphicsImageID, more, chunk)
			first = false
		} else {
			fmt.Fprintf(&out, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
		}
	}
	return out.String(), nil
}

func formatImageMeta(format string, w, h int, size int64) string {
	name := strings.ToUpper(strings.TrimSpace(format))
	if name == "" {
		name = "IMAGE"
	}
	return fmt.Sprintf("%s image / %d × %d px / %s", name, w, h, formatImageBytes(size))
}

func formatImageBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	const (
		kb = 1000
		mb = 1000 * kb
		gb = 1000 * mb
	)
	switch {
	case n < kb:
		return fmt.Sprintf("%d B", n)
	case n < mb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	case n < gb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	default:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(gb))
	}
}

// fitImage scales src down to fit maxW×maxH, preserving aspect ratio. Never upscales.
func fitImage(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 || maxW < 1 || maxH < 1 {
		return src
	}
	if w <= maxW && h <= maxH {
		return src
	}
	scale := float64(maxW) / float64(w)
	if s := float64(maxH) / float64(h); s < scale {
		scale = s
	}
	nw := int(float64(w) * scale)
	nh := int(float64(h) * scale)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}
