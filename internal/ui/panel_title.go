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
	// A URI keeps its scheme part ("sftp://host", " ://") whole; only the path after it shortens.
	if i := strings.Index(s, "://"); i > 0 {
		head, rest := s[:i+3], s[i+3:]
		if slash := strings.Index(rest, "/"); slash > 0 {
			head, rest = head+rest[:slash], rest[slash:] // sftp host part
		}
		p, segs, ok := splitPanelTitlePath(rest)
		return head + p, segs, ok
	}
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
	return prefix + strings.Join(segs, "/")
}
