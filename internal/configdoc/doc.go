// Package configdoc merges built-in documentation headers into config stub files
// while preserving user configuration byte-for-byte.
package configdoc

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DocEndSentinel marks the end of the built-in documentation block in stub files.
const DocEndSentinel = "# --- end of documentation ---"

// SplitUserBody returns the config payload after the leading documentation block.
// When DocEndSentinel is present, everything after that line (inclusive of the
// sentinel in the doc prefix) is the user body. Otherwise blank lines and #
// comments at the top are treated as documentation; the user body starts at the
// first remaining line.
func SplitUserBody(content []byte) []byte {
	text := string(content)
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if strings.TrimSpace(line) == DocEndSentinel {
			rest := strings.Join(lines[i+1:], "\n")
			return []byte(rest)
		}
	}

	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		start = i
		break
	}
	if start < 0 {
		return nil
	}
	return []byte(strings.Join(lines[start:], "\n"))
}

// MergeDocumentation combines canonicalDoc with the preserved user body.
func MergeDocumentation(canonicalDoc string, userBody []byte) []byte {
	doc := normalizeCanonicalDoc(canonicalDoc)
	body := bytes.TrimLeft(userBody, "\n")
	if len(body) == 0 {
		return []byte(doc)
	}
	merged := make([]byte, 0, len(doc)+1+len(body))
	merged = append(merged, doc...)
	merged = append(merged, '\n')
	merged = append(merged, body...)
	return merged
}

func normalizeCanonicalDoc(doc string) string {
	doc = strings.TrimRight(doc, "\n")
	return doc + "\n"
}

// RefreshDocumentation updates the documentation prefix on disk. It returns
// changed=true when the file was rewritten.
func RefreshDocumentation(path, canonicalDoc string) (changed bool, err error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return false, errors.New("config doc refresh path is required")
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	userBody := SplitUserBody(original)
	merged := MergeDocumentation(canonicalDoc, userBody)
	if bytes.Equal(merged, original) {
		return false, nil
	}
	if err := AtomicWrite(path, merged, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// AtomicWrite replaces dest with data using a temp file in the same directory.
func AtomicWrite(dest string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(dest)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(dest)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := temp.Name()
	_, copyErr := temp.Write(data)
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		return errors.Join(copyErr, syncErr, closeErr)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
