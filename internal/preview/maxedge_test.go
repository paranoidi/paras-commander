package preview

import (
	"image"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
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
