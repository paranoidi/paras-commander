package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/primitive"
)

// PanelTitlePath formats a directory path for the panel title bar.
// homeDir should be filepath.Clean(os.UserHomeDir()); when empty, paths are not collapsed to ~/.
// maxRunes is the maximum width in terminal cells (runes) for the path only—the caller
// adds surrounding spaces separately.
func PanelTitlePath(absPath, homeDir string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	display := primitive.PathWithHomeTilde(absPath, homeDir)
	display = abbreviatePanelTitlePath(display, maxRunes)
	if utf8.RuneCountInString(display) <= maxRunes {
		return display
	}
	return primitive.TruncateRight(display, maxRunes)
}

func abbreviatePanelTitlePath(display string, maxRunes int) string {
	for utf8.RuneCountInString(display) > maxRunes {
		prefix, segs, ok := splitPanelTitlePath(display)
		if !ok || len(segs) <= 1 {
			break
		}
		shortened := false
		for i := 0; i < len(segs)-1; i++ {
			r := []rune(segs[i])
			if len(r) > 1 {
				segs[i] = string(r[0])
				shortened = true
				break
			}
		}
		if !shortened {
			break
		}
		display = joinPanelTitlePath(prefix, segs)
	}
	return display
}

func splitPanelTitlePath(s string) (prefix string, segs []string, ok bool) {
	switch {
	case strings.HasPrefix(s, "~/"):
		rest := s[2:]
		if rest == "" {
			return "~/", nil, true
		}
		return "~/", strings.Split(rest, "/"), true
	case strings.HasPrefix(s, "/"):
		rest := strings.TrimPrefix(s, "/")
		if rest == "" {
			return "/", nil, true
		}
		return "/", strings.Split(rest, "/"), true
	case s == "":
		return "", nil, false
	default:
		return "", strings.Split(s, "/"), true
	}
}

func joinPanelTitlePath(prefix string, segs []string) string {
	if len(segs) == 0 {
		return prefix
	}
	switch prefix {
	case "~/":
		return "~/" + strings.Join(segs, "/")
	case "/":
		return "/" + strings.Join(segs, "/")
	default:
		return strings.Join(segs, "/")
	}
}
