package preview

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func TestExtractFramePNGAndMediaThumbs(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not on PATH")
	}
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.mp4")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "testsrc=duration=3:size=160x120:rate=10", "-y", clip)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg generate: %v\n%s", err, out)
	}
	img, err := extractFramePNG(clip, 1.0)
	if err != nil {
		t.Fatalf("extractFramePNG: %v", err)
	}
	if img.Bounds().Dx() < 1 {
		t.Fatal("empty image")
	}

	req := Request{
		Path:          clip,
		Preview:       config.PreviewConfig{Images: true, VideoThumbCols: 2, VideoThumbRows: 2},
		Media:         true,
		ImageMaxPxW:   200,
		ImageMaxPxH:   400,
		ImageCellPxH:  20,
		ImageProtocol: previewpanel.ImageProtocolSixel,
	}
	meta, work := RunMediaMeta(req)
	if meta.ErrorMsg != "" {
		t.Fatalf("RunMediaMeta: %s", meta.ErrorMsg)
	}
	if work == nil {
		t.Fatal("want thumb work")
	}
	if meta.CombinedText == "" {
		t.Fatal("empty meta")
	}
	res := RunMediaThumbs(context.Background(), req, work)
	if res.ErrorMsg != "" {
		t.Fatalf("RunMediaThumbs: %s", res.ErrorMsg)
	}
	if res.ImagePayload == "" {
		t.Fatal("want ImagePayload")
	}
	_ = os.Remove(clip)
}
