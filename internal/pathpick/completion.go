package pathpick

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/paranoidi/paras-commander/internal/search"
)

// Candidate is one filesystem-entry completion for the partial final path segment.
type Candidate struct {
	Name   string
	IsDir  bool
	Ranges []search.Range // match ranges within Name, for highlighting
}

// Suggest returns filesystem-entry candidates for the partial path segment at the caret,
// when raw is path-shaped, the caret is at the end of the line, and the parent directory of
// the final segment is readable. Mid-line caret positions return ok=false.
//
// start is the rune index in raw where the partial segment begins (partial == raw[start:cursor]
// as runes). Candidates rank exact case-sensitive prefix matches first (by name), then
// case-insensitive fuzzy matches (by score desc, then name). dirsOnly drops non-directory
// entries (e.g. a copy destination for a directory or a multi-selection).
func Suggest(panelPath, home, raw string, cursor int, showHidden, dirsOnly bool) (start int, items []Candidate, ok bool) {
	if !QueryLooksPathlike(raw) {
		return 0, nil, false
	}
	runes := []rune(raw)
	if cursor != len(runes) {
		return 0, nil, false
	}
	dirRaw, partial := splitPathAtCursor(raw, cursor)
	if dirRaw == "" && partial == "" {
		return 0, nil, false
	}
	start = len([]rune(dirRaw))

	resolvedDir := ResolveQuery(panelPath, home, dirRaw)
	entries, err := os.ReadDir(resolvedDir)
	if err != nil {
		return 0, nil, false
	}

	type scored struct {
		Candidate
		score int
	}
	var prefixed []Candidate
	var fuzzy []scored
	fuzzyOpts := search.Options{CaseInsensitive: true}
	for _, ent := range entries {
		name := ent.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		c := Candidate{Name: name}
		score := 0
		isPrefix := strings.HasPrefix(name, partial)
		if isPrefix {
			if partial != "" {
				c.Ranges = []search.Range{{Start: 0, End: len([]rune(partial))}}
			}
		} else {
			if partial == "" {
				continue
			}
			res := search.Fuzzy(partial, name, fuzzyOpts)
			if !res.Matched {
				continue
			}
			c.Ranges, score = res.Ranges, res.Score
		}
		c.IsDir = ent.IsDir()
		if !c.IsDir && ent.Type()&fs.ModeSymlink != 0 {
			// ponytail: only symlinks need a stat to tell whether they point at a directory.
			if info, err := os.Stat(filepath.Join(resolvedDir, name)); err == nil {
				c.IsDir = info.IsDir()
			}
		}
		if dirsOnly && !c.IsDir {
			continue
		}
		if isPrefix {
			prefixed = append(prefixed, c)
		} else {
			fuzzy = append(fuzzy, scored{Candidate: c, score: score})
		}
	}

	// os.ReadDir returns entries sorted by name, so prefixed is already in order and a stable
	// sort keeps name order among equal fuzzy scores.
	sort.SliceStable(fuzzy, func(i, j int) bool { return fuzzy[i].score > fuzzy[j].score })

	items = make([]Candidate, 0, len(prefixed)+len(fuzzy))
	items = append(items, prefixed...)
	for _, f := range fuzzy {
		items = append(items, f.Candidate)
	}

	if len(items) == 1 && items[0].Name == partial && !items[0].IsDir {
		return start, nil, false
	}
	if len(items) == 0 {
		return start, nil, false
	}
	return start, items, true
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
