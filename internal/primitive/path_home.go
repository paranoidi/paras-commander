package primitive

import (
	"path/filepath"
	"strings"
)

// PathWithHomeTilde returns absPath formatted for display with the user's home directory
// replaced by ~/ when applicable. homeDir should be filepath.Clean(os.UserHomeDir()); when
// empty, paths are returned cleaned with slashes normalized only.
func PathWithHomeTilde(absPath, homeDir string) string {
	path := filepath.ToSlash(filepath.Clean(absPath))
	if homeDir == "" {
		return path
	}
	home := filepath.ToSlash(filepath.Clean(homeDir))
	if home == "." || home == "" {
		return path
	}
	if path == home {
		return "~/"
	}
	prefix := home + "/"
	if !strings.HasPrefix(path, prefix) {
		return path
	}
	return "~" + path[len(home):]
}
