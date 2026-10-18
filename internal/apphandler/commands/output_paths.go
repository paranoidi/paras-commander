package commands

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// maxOutputPathStats caps the os.Lstat calls spent on one finished output.
// ponytail: output can reach cmdrun.MaxStreamBytes; stop detecting past the cap. Raise it or
// stat on demand per visible line if big listings need full coverage.
const maxOutputPathStats = 5000

// maxPathTokens is the most whitespace-separated words one mid-line path candidate may span.
// ponytail: names with more than 6 space-separated words are missed mid-line (whole-line names
// still work). Raise it if needed; cost is up to this many lstats per token, cached per call.
const maxPathTokens = 6

// detectOutputPaths finds existing filesystem paths in lines. Per line the whole trimmed line is
// tried first (names with spaces). Otherwise, at each whitespace token the longest run of up to
// maxPathTokens consecutive tokens is tried first, then shorter runs (names with spaces mid-line),
// each with surrounding quotes/brackets, trailing ",;" and a ":<digits>..." suffix (grep -n style)
// removed. The first existing candidate wins and scanning resumes after it. Relative candidates
// resolve against workDir. Columns are cell indexes of the ANSI-stripped line.
func detectOutputPaths(lines []string, workDir string) []dialog.OutputPathMatch {
	var out []dialog.OutputPathMatch
	stats := 0
	cache := map[string]os.FileInfo{} // nil = does not exist
	exists := func(cand string) (string, os.FileInfo) {
		if cand == "" || cand == "." || cand == ".." {
			return "", nil
		}
		abs := cand
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(workDir, abs)
		}
		abs = filepath.Clean(abs)
		if fi, seen := cache[abs]; seen {
			return abs, fi
		}
		if stats >= maxOutputPathStats {
			return "", nil
		}
		stats++
		fi, _ := os.Lstat(abs)
		cache[abs] = fi
		return abs, fi
	}
	for li, line := range lines {
		if line == "--- stderr ---" {
			continue
		}
		var rs []rune
		for _, c := range previewpanel.AnsiStyledCells(line, tcell.StyleDefault) {
			rs = append(rs, c.R)
		}
		first, last := 0, len(rs)
		for first < last && unicode.IsSpace(rs[first]) {
			first++
		}
		for last > first && unicode.IsSpace(rs[last-1]) {
			last--
		}
		if first == last {
			continue
		}
		if abs, fi := exists(string(rs[first:last])); fi != nil {
			out = append(out, dialog.OutputPathMatch{Line: li, Start: first, End: last, Path: abs, IsDir: fi.IsDir()})
			continue
		}
		var toks [][2]int
		for i := first; i < last; {
			if unicode.IsSpace(rs[i]) {
				i++
				continue
			}
			j := i
			for j < last && !unicode.IsSpace(rs[j]) {
				j++
			}
			toks = append(toks, [2]int{i, j})
			i = j
		}
		for i := 0; i < len(toks); {
			next := i + 1
			for j := min(i+maxPathTokens-1, len(toks)-1); j >= i; j-- {
				s, e := trimTokenSpan(rs, toks[i][0], toks[j][1])
				if abs, fi := exists(string(rs[s:e])); fi != nil {
					out = append(out, dialog.OutputPathMatch{Line: li, Start: s, End: e, Path: abs, IsDir: fi.IsDir()})
					next = j + 1
					break
				}
			}
			i = next
		}
	}
	return out
}

// trimTokenSpan narrows rs[s:e] to the path part of a token: leading quotes/brackets and
// trailing quotes/brackets/",;" are dropped and a ":<digit>" suffix is cut.
func trimTokenSpan(rs []rune, s, e int) (int, int) {
	for s < e && strings.ContainsRune(`"'`+"`"+`([{<`, rs[s]) {
		s++
	}
	for k := s; k+1 < e; k++ {
		if rs[k] == ':' && unicode.IsDigit(rs[k+1]) {
			e = k
			break
		}
	}
	for e > s && strings.ContainsRune(`"'`+"`"+`)]}>,;`, rs[e-1]) {
		e--
	}
	return s, e
}

// nextPathLine returns the index of the first match on the next line after paths[cur]'s that
// has matches, or cur when there is none.
func nextPathLine(paths []dialog.OutputPathMatch, cur int) int {
	for i := cur + 1; i < len(paths); i++ {
		if paths[i].Line != paths[cur].Line {
			return i
		}
	}
	return cur
}

// prevPathLine returns the index of the first match on the closest earlier line with matches,
// or cur when there is none.
func prevPathLine(paths []dialog.OutputPathMatch, cur int) int {
	i := cur
	for i >= 0 && paths[i].Line == paths[cur].Line {
		i--
	}
	if i < 0 {
		return cur
	}
	for i > 0 && paths[i-1].Line == paths[i].Line {
		i--
	}
	return i
}
