package preview

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
)

// maxFFprobeJSONBytes bounds ffprobe -of json stdout. Format+streams metadata is
// typically a few KB; this is a safety net, not a typical-size target.
const maxFFprobeJSONBytes = 1 << 20

// maxFFmpegFramePNGBytes bounds one extracted PNG frame. Scale-at-extract keeps
// frames near the thumbnail edge; this rejects a runaway encode before decode.
const maxFFmpegFramePNGBytes = 8 << 20

// ffprobeJSON runs ffprobe and returns the JSON document (format + streams). A nil ctx runs
// unbounded (context.Background()).
func ffprobeJSON(ctx context.Context, path string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, "ffprobe", "-show_format", "-show_streams", "-of", "json", path)
	stdout := cmdrun.CappedWriter{Max: maxFFprobeJSONBytes}
	stderr := cmdrun.CappedWriter{Max: maxFFprobeJSONBytes}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if stdout.Trimmed {
		return "", fmt.Errorf("ffprobe metadata too large")
	}
	if err != nil {
		return "", fmt.Errorf("[%s] %w", stderr.Data, err)
	}
	return string(stdout.Data), nil
}

// ffmpegFramePNG seeks to timeSec and writes one PNG frame to stdout. maxEdge, when >= 1,
// scales the frame so its longest side is at most that many pixels (no upscale). A nil ctx
// runs unbounded (context.Background()). Truncated stdout is rejected, not returned.
func ffmpegFramePNG(ctx context.Context, videoPath string, timeSec float64, maxEdge int) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	args := []string{
		"-ss", strconv.FormatFloat(timeSec, 'f', -1, 64),
		"-i", videoPath,
	}
	if maxEdge >= 1 {
		args = append(args, "-vf", fmt.Sprintf(
			"scale=%d:%d:force_original_aspect_ratio=decrease", maxEdge, maxEdge))
	}
	// image2pipe is required for stdout; plain image2 exits 0 with an empty pipe.
	args = append(args, "-f", "image2pipe", "-vframes", "1", "-vcodec", "png", "pipe:")
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout := cmdrun.CappedWriter{Max: maxFFmpegFramePNGBytes}
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if stdout.Trimmed {
		return nil, fmt.Errorf("ffmpeg frame too large")
	}
	if err != nil {
		return nil, err
	}
	return stdout.Data, nil
}
