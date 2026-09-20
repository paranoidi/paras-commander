package preview

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func TestCalculateTimeMarks(t *testing.T) {
	marks := calculateTimeMarks(10, 4)
	if len(marks) != 4 {
		t.Fatalf("len = %d, want 4", len(marks))
	}
	// duration/(n+1) = 2; marks at 2,4,6,8
	want := []float64{2, 4, 6, 8}
	for i := range want {
		if marks[i] != want[i] {
			t.Fatalf("marks[%d] = %v, want %v", i, marks[i], want[i])
		}
	}
	// Short duration: clamp below duration.
	short := calculateTimeMarks(1, 4)
	for i, m := range short {
		if m >= 1 {
			t.Fatalf("short[%d] = %v, want < 1", i, m)
		}
	}
	if calculateTimeMarks(0, 4) != nil {
		t.Fatal("zero duration: want nil")
	}
	if calculateTimeMarks(10, 0) != nil {
		t.Fatal("zero n: want nil")
	}
}

func TestFormatMediaMeta(t *testing.T) {
	doc := ffprobeDoc{
		Format: ffprobeFormat{
			Duration: "125.5",
			Size:     "1048576",
			BitRate:  "128000",
		},
		Streams: []ffprobeStream{
			{
				CodecType:  "video",
				CodecName:  "h264",
				Width:      1920,
				Height:     1080,
				RFrameRate: "30/1",
				PixFmt:     "yuv420p",
			},
			{
				CodecType:  "audio",
				CodecName:  "aac",
				SampleRate: "48000",
				Channels:   2,
				BitRate:    "160000",
			},
		},
	}
	got := formatMediaMeta(doc)
	for _, want := range []string{"Media /", "2:05", "Video: h264", "1920×1080", "30.00 fps", "Audio: aac", "48000 Hz"} {
		if !strings.Contains(got, want) {
			t.Fatalf("meta missing %q:\n%s", want, got)
		}
	}
}

func TestParseFrameRate(t *testing.T) {
	if got := parseFrameRate("30000/1001"); got < 29.9 || got > 30.0 {
		t.Fatalf("30000/1001 = %v", got)
	}
	if got := parseFrameRate("25"); got != 25 {
		t.Fatalf("25 = %v", got)
	}
	if parseFrameRate("0/0") != 0 {
		t.Fatal("0/0 want 0")
	}
}

func TestComposeThumbGrid(t *testing.T) {
	// 4 solid 16×9 frames → 2×2 grid; budget 320×200 → scale by min(320/32, 200/18)=10 → 320×180
	frames := make([]image.Image, 4)
	for i := range frames {
		img := image.NewRGBA(image.Rect(0, 0, 16, 9))
		frames[i] = img
	}
	grid := composeThumbGrid(frames, 2, 2, 320, 200)
	b := grid.Bounds()
	if b.Dx() != 320 || b.Dy() != 180 {
		t.Fatalf("grid size %dx%d, want 320x180 (no letterbox)", b.Dx(), b.Dy())
	}
}

func TestComposeThumbGridNoBlackMargins(t *testing.T) {
	frames := make([]image.Image, 4)
	for i := range frames {
		img := image.NewRGBA(image.Rect(0, 0, 40, 30))
		for y := 0; y < 30; y++ {
			for x := 0; x < 40; x++ {
				img.Set(x, y, image.White)
			}
		}
		frames[i] = img
	}
	// Tall budget would have letterboxed under the old cell-fill-with-black approach.
	grid := composeThumbGrid(frames, 2, 2, 200, 400)
	b := grid.Bounds()
	// 80×60 natural → scale min(200/80, 400/60)=2.5 → 200×150
	if b.Dx() != 200 || b.Dy() != 150 {
		t.Fatalf("grid size %dx%d, want 200x150", b.Dx(), b.Dy())
	}
	// Corner pixel of each tile should be white (no black pad).
	for _, pt := range []image.Point{{0, 0}, {100, 0}, {0, 75}, {100, 75}} {
		r, g, bl, _ := grid.At(pt.X, pt.Y).RGBA()
		if r == 0 && g == 0 && bl == 0 {
			t.Fatalf("pixel at %v is black (margin)", pt)
		}
	}
}

// videoPNGCache is a MediaCache that serves a prebuilt video-grid PNG so RunMediaThumbs
// can be tested without ffmpeg.
type videoPNGCache struct {
	png []byte
}

func (c videoPNGCache) LoadStill(context.Context, string, int64, int64, int, func(context.Context) ([]byte, string, error)) ([]byte, string, error) {
	return nil, "", nil
}
func (c videoPNGCache) LoadRender(context.Context, string, int64, int64, previewpanel.ImageProtocol, bool, bool, int, int, func(context.Context) ([]byte, int, int, error)) ([]byte, int, int, error) {
	return nil, 0, 0, nil
}
func (c videoPNGCache) LoadVideo(_ context.Context, _ string, _, _ int64, _, _, _ int, _ func(int, int), _ func(context.Context, func(int, int)) ([]byte, error)) ([]byte, error) {
	return c.png, nil
}
func (c videoPNGCache) HasVideo(string, int64, int64, int, int, int) bool { return true }
func (c videoPNGCache) InFlight(string) bool                             { return false }
func (c videoPNGCache) SnapshotInFlight() []string                       { return nil }

// TestRunMediaThumbsSixelUnderTmuxShrinksToFit covers a high-entropy video grid whose first
// encode exceeds tmux's sixel byte cap: it must shrink (or fall back with an explicit
// "too large for tmux" caption), same as stills — not silently drop the image.
func TestRunMediaThumbsSixelUnderTmuxShrinksToFit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "grid.png")
	writeNoisyTestPNG(t, path, 700, 700)
	pngBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	req := Request{
		Path:          path,
		Preview:       config.PreviewConfig{Images: true, VideoThumbCols: 2, VideoThumbRows: 2, VideoMetadata: true},
		Media:         true,
		ImageMaxPxW:   700,
		ImageMaxPxH:   700,
		ImageCellPxH:  20,
		ImageProtocol: previewpanel.ImageProtocolSixel,
		ImageInTmux:   true,
		Cache:         videoPNGCache{png: pngBytes},
	}
	res := RunMediaThumbs(context.Background(), req, &MediaThumbWork{meta: "Media / test clip", duration: 10}, nil)
	if res.ErrorMsg != "" {
		t.Fatalf("ErrorMsg = %q", res.ErrorMsg)
	}
	if res.ImagePayload == "" {
		if !strings.Contains(res.CombinedText, "too large for tmux") {
			t.Fatalf("ImagePayload empty without explicit fallback, CombinedText = %q", res.CombinedText)
		}
		return
	}
	if len(res.ImagePayload) >= config.DefaultPreviewTmuxSixelMaxBytes {
		t.Fatalf("ImagePayload len = %d, want < %d", len(res.ImagePayload), config.DefaultPreviewTmuxSixelMaxBytes)
	}
	if res.ImagePxW <= 0 || res.ImagePxH <= 0 {
		t.Fatalf("ImagePxW/H = %d/%d, want positive", res.ImagePxW, res.ImagePxH)
	}
	if res.ImagePxW >= 700 || res.ImagePxH >= 700 {
		t.Fatalf("ImagePxW/H = %d/%d, want smaller than the unshrunk 700x700 fit", res.ImagePxW, res.ImagePxH)
	}
}
