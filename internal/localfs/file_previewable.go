package localfs

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// PreviewSniffBytes is how many leading bytes are read to decide if a file is previewable as text.
const PreviewSniffBytes = 32768

// ErrFilePreviewBinary indicates the file prefix looks binary (NUL or invalid UTF-8).
var ErrFilePreviewBinary = errors.New("not a text file")

// ErrFilePreviewIsDir indicates preview was requested for a directory.
var ErrFilePreviewIsDir = errors.New("is a directory")

// ErrFilePreviewImage indicates the path is an image extension eligible for sixel preview.
var ErrFilePreviewImage = errors.New("image file")

// ErrFilePreviewMedia indicates the path is a video/audio extension eligible for media preview.
var ErrFilePreviewMedia = errors.New("media file")

// IsImagePath reports whether path has a supported image extension (case-insensitive).
// Single source of truth for image preview eligibility by name.
func IsImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

// IsMediaPath reports whether path has a supported video/audio extension (case-insensitive).
// Single source of truth for media preview eligibility by name.
func IsMediaPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mkv", ".mp4", ".m4v", ".webm", ".avi", ".mov", ".mpg", ".mpeg", ".ts", ".ogv",
		".mp3", ".flac", ".ogg", ".opus", ".m4a", ".wav", ".aac":
		return true
	default:
		return false
	}
}

// CheckFilePreviewable returns nil if path is a non-directory regular file whose first
// PreviewSniffBytes bytes contain no NUL and are valid UTF-8.
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
	if len(buf) > 0 && !utf8.Valid(buf) {
		return ErrFilePreviewBinary
	}
	return nil
}
