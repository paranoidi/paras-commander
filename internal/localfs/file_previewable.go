package localfs

import (
	"bytes"
	"errors"
	"io"
	"os"
	"unicode/utf8"
)

// PreviewSniffBytes is how many leading bytes are read to decide if a file is previewable as text.
const PreviewSniffBytes = 32768

// ErrFilePreviewBinary indicates the file prefix looks binary (NUL or invalid UTF-8).
var ErrFilePreviewBinary = errors.New("not a text file")

// ErrFilePreviewIsDir indicates preview was requested for a directory.
var ErrFilePreviewIsDir = errors.New("is a directory")

// CheckFilePreviewable returns nil if path is a non-directory regular file whose first
// PreviewSniffBytes bytes contain no NUL and are valid UTF-8.
func CheckFilePreviewable(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return ErrFilePreviewIsDir
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
