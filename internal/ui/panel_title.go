package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
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
	// A display prefix ("<icon> " for fileindex://) stays whole; only the path after it shortens.
	if i := strings.Index(s, " "); i > 0 && !strings.ContainsAny(s[:i], "/~") &&
		(strings.HasPrefix(s[i+1:], "/") || strings.HasPrefix(s[i+1:], "~/")) {
		p, segs, ok := splitPanelTitlePath(s[i+1:])
		return s[:i+1] + p, segs, ok
	}
	// A URI keeps its scheme part ("sftp://host") whole; only the path after it shortens.
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

// PanelTitleSegment is one clickable path element in the panel title: the cells it occupies and
// how many directories above the panel path it points to (0 is the panel directory itself).
type PanelTitleSegment struct {
	X, Width, Depth int
}

// Contains reports whether column x lies on the segment.
func (s PanelTitleSegment) Contains(x int) bool {
	return x >= s.X && x < s.X+s.Width
}

// panelTitleSegments lays out the path elements PanelTitlePath draws for absPath from column
// startX: the root token ("/", "~", "sftp://host") and each component, without the "/" separators
// or a display prefix ahead of the root. Abbreviated components keep their depth; components cut
// off by truncation are clipped to the cells left before the ellipsis.
func panelTitleSegments(absPath, homeDir string, startX, maxRunes int) []PanelTitleSegment {
	if maxRunes <= 0 {
		return nil
	}
	display := abbreviatePanelTitlePath(primitive.PathWithHomeTilde(absPath, homeDir), maxRunes)
	prefix, segs, ok := splitPanelTitlePath(display)
	if !ok {
		return nil
	}
	limit := maxRunes
	if runewidth.StringWidth(display) > maxRunes {
		limit = maxRunes - 1 // the ellipsis cell
	}
	var out []PanelTitleSegment
	add := func(col, width, depth int) {
		if width = min(width, limit-col); width > 0 {
			out = append(out, PanelTitleSegment{X: startX + col, Width: width, Depth: depth})
		}
	}
	// The root token follows any display prefix ("<icon> ") and drops its trailing separator.
	tokenStart := strings.LastIndex(prefix, " ") + 1
	if token := prefix[tokenStart:]; token != "" {
		if token != "/" {
			token = strings.TrimSuffix(token, "/")
		}
		add(runewidth.StringWidth(prefix[:tokenStart]), runewidth.StringWidth(token), len(segs))
	}
	col := runewidth.StringWidth(prefix)
	for i, seg := range segs {
		w := runewidth.StringWidth(seg)
		add(col, w, len(segs)-1-i)
		col += w + 1
	}
	return out
}
