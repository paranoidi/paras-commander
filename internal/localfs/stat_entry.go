package localfs

import (
	"fmt"
	"os"
	"path/filepath"
)

// EntryFromPath returns a populated Entry by stat'ing path (symlinks are not followed for type/mode).
func EntryFromPath(path string) (Entry, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Entry{}, fmt.Errorf("resolve path %q: %w", path, err)
	}
	abs = filepath.Clean(abs)
	info, err := os.Lstat(abs)
	if err != nil {
		return Entry{}, fmt.Errorf("stat %q: %w", abs, err)
	}
	dev, devOK := entryDevice(info)
	return Entry{
		Name:       filepath.Base(abs),
		Path:       abs,
		Type:       ClassifyMode(info.Mode()),
		Size:       info.Size(),
		Mode:       info.Mode(),
		ModifiedAt: info.ModTime(),
		Dev:        dev,
		DevValid:   devOK,
	}, nil
}
