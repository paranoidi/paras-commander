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

	pixels := int64(cfg.Width) * int64(cfg.Height)
	maxPixels := int64(config.DefaultPreviewImageMaxDecodeMegapixels) * 1_000_000
	if pixels > maxPixels {
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
	payload, err := encodeImagePayload(scaled, req.ImageProtocol, req.ImageUnicodePlaceholder, req.ImageInTmux)
	if err != nil || payload == "" {
		return metaResult
	}
	if req.ImageProtocol == previewpanel.ImageProtocolSixel && req.ImageInTmux &&
		len(payload) >= config.DefaultPreviewTmuxSixelMaxBytes {
		// tmux (through 3.5a) silently discards a single escape sequence beyond its hardcoded
		// ~1MB input buffer rather than forwarding it — sending this would show as the image
		// flickering and vanishing rather than a clean, if lower-quality, preview.
		metaResult.CombinedText = meta + " / too large for tmux"
		return metaResult
	}
	return Result{
		Source:        previewpanel.SourceExternalANSI,
		CombinedText:  meta,
		ImagePayload:  payload,
		ImagePxW:      bounds.Dx(),
		ImagePxH:      bounds.Dy(),
		ImageProtocol: req.ImageProtocol,

		ImageUnicodePlaceholder: req.ImageUnicodePlaceholder,
	}
}

func encodeImagePayload(img image.Image, proto previewpanel.ImageProtocol, unicodePlaceholder, inTmux bool) (string, error) {
	switch proto {
	case previewpanel.ImageProtocolKitty:
		return encodeKittyAPC(img, unicodePlaceholder)
	case previewpanel.ImageProtocolSixel:
		var buf bytes.Buffer
		enc := sixel.NewEncoder(&buf)
		if inTmux {
			// See config.DefaultPreviewTmuxSixelColors: a smaller palette compresses far
			// better under sixel's run-length encoding, keeping the payload under tmux's
			// (pre-3.6) hardcoded input buffer limit for typical previews.
			enc.Colors = config.DefaultPreviewTmuxSixelColors
		}
		if err := enc.Encode(img); err != nil {
			return "", err
		}
		return buf.String(), nil
	default:
		return "", fmt.Errorf("unsupported image protocol %d", proto)
	}
}

// encodeKittyAPC builds a chunked Kitty graphics transmit sequence (f=100 PNG). With
// unicodePlaceholder, the first chunk requests Unicode-placeholder display (U=1) instead of
// the terminal's own cursor-relative auto-display (C=1) — see
// internal/ui/previewpanel/unicode_placeholder.go for why (tmux compatibility) and how the
// placeholder cells that reference this transmitted data get drawn.
func encodeKittyAPC(img image.Image, unicodePlaceholder bool) (string, error) {
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(pngBuf.Bytes())
	if encoded == "" {
		return "", fmt.Errorf("empty png")
	}

	firstChunkParams := fmt.Sprintf("a=T,f=100,i=%d,C=1,q=2", previewpanel.KittyGraphicsImageID)
	if unicodePlaceholder {
		firstChunkParams = fmt.Sprintf("a=T,f=100,i=%d,q=2,U=1", previewpanel.KittyGraphicsImageID)
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
			fmt.Fprintf(&out, "\x1b_G%s,m=%d;%s\x1b\\", firstChunkParams, more, chunk)
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
