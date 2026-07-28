package preview

import (
	"image"
	"strings"
	"testing"
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
