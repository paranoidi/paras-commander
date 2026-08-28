package localfs

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PreviewSniffBytes is how many leading bytes are read to decide if a file is previewable as text.
const PreviewSniffBytes = 32768

// ErrFilePreviewBinary indicates the file prefix looks binary (contains a NUL byte).
var ErrFilePreviewBinary = errors.New("not a text file")

// ErrFilePreviewIsDir indicates preview was requested for a directory.
var ErrFilePreviewIsDir = errors.New("is a directory")

// ErrFilePreviewImage indicates the path is an image extension eligible for sixel preview.
var ErrFilePreviewImage = errors.New("image file")

// ErrFilePreviewMedia indicates the path is a video/audio extension eligible for media preview.
var ErrFilePreviewMedia = errors.New("media file")

// imageMagickExtensions lists image extensions Go's stdlib image package cannot decode
// natively. These route through ImageMagick conversion (see internal/preview/imagemagick.go)
// before the normal decode/fit/encode pipeline. Add an extension here to support it — no
// other pipeline change needed.
var imageMagickExtensions = map[string]bool{
	".psd": true,
}

// IsImageMagickPath reports whether path needs ImageMagick conversion before it can be
// decoded (see imageMagickExtensions).
func IsImageMagickPath(path string) bool {
	return imageMagickExtensions[strings.ToLower(filepath.Ext(path))]
}

// IsImagePath reports whether path has a supported image extension (case-insensitive).
// Single source of truth for image preview eligibility by name.
func IsImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tif", ".tiff":
		return true
	default:
		return IsImageMagickPath(path)
	}
}

// IsVideoPath reports whether path has a supported video extension (case-insensitive).
// Used for thumbnail generation / prefetch (audio is media but has no video thumbs).
func IsVideoPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mkv", ".mp4", ".m4v", ".webm", ".avi", ".mov", ".mpg", ".mpeg", ".ts", ".ogv":
		return true
	default:
		return false
	}
}

// IsMediaPath reports whether path has a supported video/audio extension (case-insensitive).
// Single source of truth for media preview eligibility by name.
func IsMediaPath(path string) bool {
	if IsVideoPath(path) {
		return true
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".flac", ".ogg", ".opus", ".m4a", ".wav", ".aac":
		return true
	default:
		return false
	}
}

// CheckFilePreviewable returns nil if path is a non-directory regular file whose first
// PreviewSniffBytes bytes contain no NUL byte (the same heuristic git uses for binary
// detection). Non-UTF-8 text (e.g. legacy DOS/Windows codepages) is still previewable —
// internal/preview's decodeSource transcodes it via a Windows-1252 fallback at render time.
// Image paths return ErrFilePreviewImage without opening the file.
// Media paths return ErrFilePreviewMedia without opening the file.
func CheckFilePreviewable(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return ErrFilePreviewIsDir
	}
	if IsImagePath(path) {
		return ErrFilePreviewImage
	}
	if IsMediaPath(path) {
		return ErrFilePreviewMedia
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, PreviewSniffBytes)
	n, err := io.ReadFull(f, buf)
	if err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			return err
		}
	}
	buf = buf[:n]
	if bytes.IndexByte(buf, 0) >= 0 {
		return ErrFilePreviewBinary
	}
	return nil
}

// IsGraphicalPreviewPath reports whether path is previewed through the terminal graphics path
// (sixel/Kitty) rather than as text. Single source of truth for that branch.
func IsGraphicalPreviewPath(path string) bool {
	return IsImagePath(path) || IsMediaPath(path)
}
