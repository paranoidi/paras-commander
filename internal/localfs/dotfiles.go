package localfs

import (
	"fmt"
	"os"
	"path/filepath"
)

// DirHasDotfileNames reports whether dir contains at least one dot-prefixed name in ReadDir results.
func DirHasDotfileNames(path string) (bool, error) {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("resolve directory %q: %w", path, err)
	}
	dirEntries, err := os.ReadDir(cleanPath)
	if err != nil {
		return false, fmt.Errorf("read directory %q: %w", cleanPath, err)
	}
	for _, dirEntry := range dirEntries {
		name := dirEntry.Name()
		if len(name) > 0 && name[0] == '.' {
			return true, nil
		}
	}
	return false, nil
}
