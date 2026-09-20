package preview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFakeBin(t *testing.T, dir, name, script string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func prependPATH(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writeOversizedPayload(t *testing.T, dir string, n int) string {
	t.Helper()
	path := filepath.Join(dir, "payload")
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFfprobeJSONRejectsOversizedStdout(t *testing.T) {
	dir := t.TempDir()
	payload := writeOversizedPayload(t, dir, maxFFprobeJSONBytes+1)
	writeFakeBin(t, dir, "ffprobe", fmt.Sprintf(`cat %q`, payload))
	prependPATH(t, dir)

	_, err := ffprobeJSON(context.Background(), filepath.Join(dir, "clip.mp4"))
	if err == nil {
		t.Fatal("expected error for oversized ffprobe JSON")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("err = %v, want too large", err)
	}
}

func TestFfmpegFramePNGRejectsOversizedStdout(t *testing.T) {
	dir := t.TempDir()
	payload := writeOversizedPayload(t, dir, maxFFmpegFramePNGBytes+1)
	writeFakeBin(t, dir, "ffmpeg", fmt.Sprintf(`cat %q`, payload))
	prependPATH(t, dir)

	got, err := ffmpegFramePNG(context.Background(), filepath.Join(dir, "clip.mp4"), 1.0, 0)
	if err == nil {
		t.Fatal("expected error for oversized ffmpeg PNG")
	}
	if got != nil {
		t.Fatalf("got %d bytes, want nil (truncated PNG must not be returned for decode)", len(got))
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("err = %v, want too large", err)
	}
}
