package preview

import (
	"image"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func TestFitImageMaxEdgeClamp(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4000, 2000))
	got := fitImage(src, 1024, 1024)
	b := got.Bounds()
	if b.Dx() != 1024 || b.Dy() != 512 {
		t.Fatalf("got %dx%d, want 1024x512", b.Dx(), b.Dy())
	}
}

func TestImageMaxEdgeDefault(t *testing.T) {
	if got := ImageMaxEdge(config.PreviewConfig{}); got != config.DefaultPreviewImageMaxEdgePx {
		t.Fatalf("ImageMaxEdge(empty) = %d, want %d", got, config.DefaultPreviewImageMaxEdgePx)
	}
	if got := ImageMaxEdge(config.PreviewConfig{ImageMaxEdgePx: 512}); got != 512 {
		t.Fatalf("ImageMaxEdge(512) = %d, want 512", got)
	}
}

func TestTmuxSixelMaxEdgeDefault(t *testing.T) {
	if got := TmuxSixelMaxEdge(config.PreviewConfig{}); got != config.DefaultPreviewTmuxSixelMaxEdgePx {
		t.Fatalf("TmuxSixelMaxEdge(empty) = %d, want %d", got, config.DefaultPreviewTmuxSixelMaxEdgePx)
	}
	if got := TmuxSixelMaxEdge(config.PreviewConfig{TmuxSixelMaxEdgePx: 512}); got != 512 {
		t.Fatalf("TmuxSixelMaxEdge(512) = %d, want 512", got)
	}
}

func TestEffectiveStillMaxEdge(t *testing.T) {
	cfg := config.PreviewConfig{ImageMaxEdgePx: 0, TmuxSixelMaxEdgePx: 1024}
	cases := []struct {
		name     string
		protocol previewpanel.ImageProtocol
		inTmux   bool
		want     int
	}{
		{"sixel+tmux uses tmux clamp", previewpanel.ImageProtocolSixel, true, 1024},
		{"sixel outside tmux uses general clamp", previewpanel.ImageProtocolSixel, false, 0},
		{"kitty in tmux uses general clamp", previewpanel.ImageProtocolKitty, true, 0},
		{"kitty outside tmux uses general clamp", previewpanel.ImageProtocolKitty, false, 0},
	}
	for _, c := range cases {
		if got := EffectiveStillMaxEdge(cfg, c.protocol, c.inTmux); got != c.want {
			t.Errorf("%s: EffectiveStillMaxEdge() = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestVideoThumbMaxEdgeFallback(t *testing.T) {
	if got := VideoThumbMaxEdge(config.PreviewConfig{ImageMaxEdgePx: 0}); got != config.DefaultPreviewTmuxSixelMaxEdgePx {
		t.Fatalf("VideoThumbMaxEdge(0) = %d, want %d", got, config.DefaultPreviewTmuxSixelMaxEdgePx)
	}
	if got := VideoThumbMaxEdge(config.PreviewConfig{ImageMaxEdgePx: 512}); got != 512 {
		t.Fatalf("VideoThumbMaxEdge(512) = %d, want 512", got)
	}
}
