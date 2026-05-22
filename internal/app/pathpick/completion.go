package pathpick

import (
	"os"
	"path/filepath"
	"strings"
)

// Completion is a filesystem name completion suffix at the query caret.
type Completion struct {
	Suffix string // runes to insert at cursor (exclusive of trailing slash)
	IsDir  bool
}

// SuggestAtCursor returns a directory-entry completion when raw is path-shaped,
// the caret is at the end of the line, and the parent directory of the final
// path segment exists. Mid-line caret positions do not show suggestions.
func SuggestAtCursor(panelPath, home, raw string, cursor int, showHidden bool) (Completion, bool) {
	if !QueryLooksPathlike(raw) {
		return Completion{}, false
	}
	runes := []rune(raw)
	if cursor != len(runes) {
		return Completion{}, false
	}
	dirRaw, partial := splitPathAtCursor(raw, cursor)
	if dirRaw == "" && partial == "" {
		return Completion{}, false
	}
	resolvedDir := ResolveQuery(panelPath, home, dirRaw)
	entries, err := os.ReadDir(resolvedDir)
	if err != nil {
		return Completion{}, false
	}
	var matches []struct {
		name  string
		isDir bool
	}
	for _, ent := range entries {
		name := ent.Name()
		if name == "." || name == ".." {
			continue
		}
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if !strings.HasPrefix(name, partial) {
			continue
		}
		isDir := ent.IsDir()
		if !isDir {
			if info, err := ent.Info(); err == nil {
				isDir = info.IsDir()
			}
		}
		matches = append(matches, struct {
			name  string
			isDir bool
		}{name: name, isDir: isDir})
	}
	if len(matches) == 0 {
		return Completion{}, false
	}
	var fullName string
	var isDir bool
	switch len(matches) {
	case 1:
		fullName = matches[0].name
		isDir = matches[0].isDir
	default:
		fullName = longestCommonPrefixName(matches)
		isDir = completionIsDirForName(matches, fullName)
	}
	if len(fullName) <= len(partial) {
		return Completion{}, false
	}
	return Completion{
		Suffix: fullName[len(partial):],
		IsDir:  isDir,
	}, true
}

func splitPathAtCursor(raw string, cursor int) (dirRaw, partial string) {
	runes := []rune(raw)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	prefix := string(runes[:cursor])
	sep := '/'
	if filepath.Separator != '/' {
		sep = filepath.Separator
	}
	lastSep := strings.LastIndex(prefix, string(sep))
	if lastSep < 0 {
		return "", prefix
	}
	return prefix[:lastSep+1], prefix[lastSep+1:]
}

func longestCommonPrefixName(matches []struct {
	name  string
	isDir bool
}) string {
	if len(matches) == 0 {
		return ""
	}
	common := matches[0].name
	for _, m := range matches[1:] {
		common = commonPrefix(common, m.name)
		if common == "" {
			break
		}
	}
	return common
}

func completionIsDirForName(matches []struct {
	name  string
	isDir bool
}, fullName string) bool {
	var exact *bool
	for i := range matches {
		if matches[i].name != fullName {
			continue
		}
		d := matches[i].isDir
		if exact == nil {
			exact = &d
			continue
		}
		if *exact != d {
			return false
		}
	}
	return exact != nil && *exact
}

func commonPrefix(a, b string) string {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	i := 0
	for i < limit && a[i] == b[i] {
		i++
	}
	return a[:i]
}
