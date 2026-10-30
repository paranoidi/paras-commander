package panellist

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// Name shortening for the listing name column. One ordered list of steps is planned per name,
// independent of width, and a width applies as many steps as it needs, so a narrower column
// always shows a subset of what a wider one shows (no flicker while resizing).
const (
	shortenSoftAbbrev = 5 // words are abbreviated to this many runes before any is dropped
	shortenMinAbbrev  = 4 // after dropping, the remaining words go down to this many runes
	shortenMinCut     = 3 // a word's first abbreviation removes at least this many runes
	shortenMaxExt     = 5 // longest extension (runes after the last dot) kept whole
)

// nameTok is a word or a separator run of the name body (extension excluded).
type nameTok struct {
	start, end int // rune range in the name
	sep        bool
	minLen     int // shortest abbreviation allowed; == len when the word is never abbreviated
	digits     bool
	cur        int // runes shown
	dropped    bool
}

func (t *nameTok) len() int { return t.end - t.start }

type shortenStep struct {
	tok  *nameTok
	cut  int // runes removed; 0 for a drop
	drop bool
}

func (s shortenStep) apply() {
	if s.drop {
		s.tok.dropped = true
		return
	}
	s.tok.cur -= s.cut
}

func isNameSep(r rune) bool { return r == '.' || r == '-' || r == '_' || r == ' ' }

// nameExtStart returns the rune index of a short alphanumeric extension's dot, or len(name).
func nameExtStart(name []rune) int {
	for i := len(name) - 1; i > 0; i-- {
		if name[i] != '.' {
			if !unicode.IsLetter(name[i]) && !unicode.IsDigit(name[i]) {
				return len(name)
			}
			continue
		}
		if n := len(name) - 1 - i; n >= 1 && n <= shortenMaxExt {
			return i
		}
		return len(name)
	}
	return len(name)
}

func tokenizeName(name []rune, end int) (toks, words []*nameTok) {
	for i := 0; i < end; {
		sep := isNameSep(name[i])
		j := i + 1
		for j < end && isNameSep(name[j]) == sep {
			j++
		}
		t := &nameTok{start: i, end: j, sep: sep, cur: j - i}
		toks = append(toks, t)
		if !sep {
			t.minLen = min(t.cur, shortenMinAbbrev)
			for _, r := range name[i:j] {
				t.digits = t.digits || unicode.IsDigit(r)
			}
			if t.digits { // a cut number reads as a different number: drop it whole
				t.minLen = t.cur
			}
			words = append(words, t)
		}
		i = j
	}
	return toks, words
}

// planShorten returns the width-independent step order: abbreviate (longest word first) down to
// shortenSoftAbbrev, drop inner words as one gap growing from the middle, abbreviate the rest
// down to shortenMinAbbrev, drop the last word, then abbreviate the first word down to one rune.
func planShorten(words []*nameTok) []shortenStep {
	var steps []shortenStep
	lens := make([]int, len(words))
	for i, t := range words {
		lens[i] = t.cur
	}
	dropped := make([]bool, len(words))
	cut := func(i, floor int) int {
		t := words[i]
		floor = max(t.minLen, floor)
		switch {
		case dropped[i] || lens[i] <= floor:
			return 0
		case lens[i] < t.len():
			return 1
		case lens[i]-shortenMinCut >= floor:
			return shortenMinCut
		}
		return 0
	}
	abbreviate := func(floor, only int) {
		for {
			best := -1
			for i := range words {
				if (only < 0 || i == only) && cut(i, floor) > 0 && (best < 0 || lens[i] >= lens[best]) {
					best = i
				}
			}
			if best < 0 {
				return
			}
			c := cut(best, floor)
			lens[best] -= c
			steps = append(steps, shortenStep{tok: words[best], cut: c})
		}
	}
	drop := func(i int) {
		dropped[i] = true
		steps = append(steps, shortenStep{tok: words[i], drop: true})
	}
	abbreviate(shortenSoftAbbrev, -1)
	n := len(words)
	if n >= 3 {
		lo, hi := n/2, n/2 // the gap grows from just right of centre
		drop(lo)
		for lo > 1 || hi < n-2 {
			if hi < n-2 {
				hi++
				drop(hi)
			}
			if lo > 1 {
				lo--
				drop(lo)
			}
		}
	}
	abbreviate(shortenMinAbbrev, -1)
	if n >= 2 {
		drop(n - 1)
	}
	if n >= 1 && !words[0].digits {
		words[0].minLen = 1
		abbreviate(1, 0)
	}
	return steps
}

// renderShortened lays out toks plus the extension from extStart. A run of dropped words and the
// separators around it becomes one Ellipsis; an abbreviated word's last rune and every Ellipsis
// are marked Cut.
func renderShortened(name []rune, toks []*nameTok, extStart int) []DisplayRune {
	out := make([]DisplayRune, 0, len(name))
	gap := false
	for i, t := range toks {
		switch {
		case t.sep:
			if !gap && (i+1 >= len(toks) || !toks[i+1].dropped) {
				for k := t.start; k < t.end; k++ {
					out = append(out, DisplayRune{Rune: name[k], NameIdx: k})
				}
			}
		case t.dropped:
			if !gap {
				out = append(out, DisplayRune{Rune: primitive.Ellipsis, NameIdx: -1, Cut: true})
				gap = true
			}
		default:
			gap = false
			for k := t.start; k < t.start+t.cur; k++ {
				out = append(out, DisplayRune{Rune: name[k], NameIdx: k})
			}
			if t.cur < t.len() {
				out[len(out)-1].Cut = true
			}
		}
	}
	for k := extStart; k < len(name); k++ {
		out = append(out, DisplayRune{Rune: name[k], NameIdx: k})
	}
	return out
}

// shortenName fits name into width cells (width >= 1) word by word; see planShorten.
// withExt keeps a short trailing extension whole (files only).
func shortenName(name []rune, width int, withExt bool) []DisplayRune {
	extStart := len(name)
	if withExt {
		extStart = nameExtStart(name)
		if runewidth.StringWidth(string(name[extStart:]))+shortenMinAbbrev > width {
			extStart = len(name)
		}
	}
	toks, words := tokenizeName(name, extStart)
	out := renderShortened(name, toks, extStart)
	for _, st := range planShorten(words) {
		if displayCells(out) <= width {
			return out
		}
		st.apply()
		out = renderShortened(name, toks, extStart)
	}
	if displayCells(out) <= width {
		return out
	}
	// Last resort: cut on the right.
	keep := out[:takeCellsLeft(out, width-1)]
	if n := len(keep); n > 0 && keep[n-1].NameIdx < 0 {
		keep = keep[:n-1]
	}
	return append(keep, DisplayRune{Rune: primitive.Ellipsis, NameIdx: -1, Cut: true})
}

// ListingCutSpans returns spans recolouring the cells where the row's shortened name was cut
// (theme.CutMarkStyle of the cell's own style). cellStyleAt gives the cell's style by display index.
func ListingCutSpans(display []DisplayRune, cellStyleAt func(displayIndex int) tcell.Style) []primitive.Span {
	var spans []primitive.Span
	for i, dr := range display {
		if dr.Cut {
			spans = append(spans, primitive.Span{Start: i, End: i + 1, Style: theme.CutMarkStyle(cellStyleAt(i))})
		}
	}
	return spans
}
