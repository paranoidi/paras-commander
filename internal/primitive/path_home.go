package primitive

import (
	"maps"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// PathWithHomeTilde returns absPath formatted for display with the user's home directory
// replaced by ~/ when applicable. homeDir should be filepath.Clean(os.UserHomeDir()); when
// empty, paths are returned cleaned with slashes normalized only.
//
// A URI ("scheme://rest") is never cleaned: it is shown as is, or with its scheme prefix replaced
// by the registered display text (SetPathDisplayPrefixes) and the rest shortened as a host path.
func PathWithHomeTilde(absPath, homeDir string) string {
	if i := strings.Index(absPath, "://"); i > 0 {
		canon, rest := absPath[:i+3], absPath[i+3:]
		if shown, ok := pathDisplayPrefix(canon); ok {
			return shown + PathWithHomeTilde(rest, homeDir)
		}
		return absPath
	}
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

// displayPrefixes maps a canonical URI prefix ("fileindex://") to the text shown in its place.
// The app sets it from the theme; render code on any goroutine reads it.
var displayPrefixes atomic.Pointer[map[string]string]

// SetPathDisplayPrefixes replaces the URI prefixes PathWithHomeTilde shows differently (canonical
// prefix -> display text). Prefixes absent from m are shown as is.
func SetPathDisplayPrefixes(m map[string]string) {
	cp := maps.Clone(m)
	displayPrefixes.Store(&cp)
}

func pathDisplayPrefix(canon string) (string, bool) {
	m := displayPrefixes.Load()
	if m == nil {
		return "", false
	}
	shown, ok := (*m)[canon]
	return shown, ok
}
