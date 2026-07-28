package preview

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
)

// ffprobeJSON runs ffprobe and returns the JSON document (format + streams).
func ffprobeJSON(path string) (string, error) {
	cmd := exec.Command("ffprobe", "-show_format", "-show_streams", "-of", "json", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("[%s] %w", stderr.String(), err)
	}
	return stdout.String(), nil
}

// ffmpegFramePNG seeks to timeSec and writes one PNG frame to stdout.
func ffmpegFramePNG(videoPath string, timeSec float64) ([]byte, error) {
	// image2pipe is required for stdout; plain image2 exits 0 with an empty pipe.
	cmd := exec.Command(
		"ffmpeg",
		"-ss", strconv.FormatFloat(timeSec, 'f', -1, 64),
		"-i", videoPath,
		"-f", "image2pipe",
		"-vframes", "1",
		"-vcodec", "png",
		"pipe:",
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}
