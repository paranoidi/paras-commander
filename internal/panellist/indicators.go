// Package panellist implements shared file-list name-column suffix indicators (job, new, subtree).
package panellist

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// DisplayRune is one glyph in the listing name column; NameIdx is -1 for decorations.
type DisplayRune struct {
	Rune    rune
	NameIdx int
}

// NewFileMarkTier selects new-file suffix coloring (latest vs previous batch).
type NewFileMarkTier int

const (
	NewFileMarkNone NewFileMarkTier = iota
	NewFileMarkLatest
	NewFileMarkPrevious
)

// RowSuffix selects which trailing indicators to reserve and paint on a listing row.
type RowSuffix struct {
	JobGlyph         rune
	NewFileTier      NewFileMarkTier
	SubtreeSelection bool
}

// SuffixDecorationLen returns how many trailing runes are reserved for row suffix indicators.
func SuffixDecorationLen(width int, suffix RowSuffix, entry localfs.Entry, th theme.Theme) int {
	n := 0
	if suffix.JobGlyph != 0 && width > n+2 {
		n += 2
	}
	if suffix.NewFileTier != NewFileMarkNone && width > n+2 {
		n += 2
	}
	subtree := suffix.SubtreeSelection && entry.Type == localfs.EntryDirectory
	if subtree && width > n+2 {
		n += 2
	}
	return n
}

// EntryDisplayRunes builds the display rune slice for an entry name, including decorations and suffix glyphs.
func EntryDisplayRunes(entry localfs.Entry, width int, showFileIcons bool, suffix RowSuffix, th theme.Theme) []DisplayRune {
	subtree := suffix.SubtreeSelection && entry.Type == localfs.EntryDirectory
	suffixLen := SuffixDecorationLen(width, suffix, entry, th)
	innerW := width - suffixLen
	if innerW < 1 {
		innerW = 1
	}

	prefix := " "
	if entry.Type == localfs.EntryDirectory && !showFileIcons {
		prefix = "/"
	}
	entryRunes := []rune(entry.Name)

	var body []DisplayRune
	body = append(body, DisplayRune{Rune: []rune(prefix)[0], NameIdx: -1})
	for i, r := range entryRunes {
		body = append(body, DisplayRune{Rune: r, NameIdx: i})
	}
	if entry.Type == localfs.EntrySymlink {
		body = append(body, DisplayRune{Rune: '@', NameIdx: -1})
	}

	if width <= 0 {
		return nil
	}

	var core []DisplayRune
	if len(body) <= innerW {
		core = body
	} else if innerW <= 3 {
		core = body[:innerW]
	} else {
		prefixWidth := (innerW - 1) / 2
		suffixWidth := innerW - prefixWidth - 1
		truncated := make([]DisplayRune, 0, innerW)
		truncated = append(truncated, body[:prefixWidth]...)
		truncated = append(truncated, DisplayRune{Rune: '~', NameIdx: -1})
		truncated = append(truncated, body[len(body)-suffixWidth:]...)
		core = truncated
	}

	if suffixLen == 0 {
		return core
	}
	out := make([]DisplayRune, 0, len(core)+suffixLen)
	out = append(out, core...)
	used := 0
	if suffix.JobGlyph != 0 && width > used+2 {
		out = append(out, DisplayRune{Rune: ' ', NameIdx: -1}, DisplayRune{Rune: suffix.JobGlyph, NameIdx: -1})
		used += 2
	}
	if suffix.NewFileTier != NewFileMarkNone && width > used+2 {
		out = append(out, DisplayRune{Rune: ' ', NameIdx: -1}, DisplayRune{Rune: th.SymbolFilelistNew(), NameIdx: -1})
		used += 2
	}
	if subtree && width > used+2 {
		out = append(out, DisplayRune{Rune: ' ', NameIdx: -1}, DisplayRune{Rune: th.SymbolFilelistSelectionSubtree(), NameIdx: -1})
	}
	return out
}

// RunesFromDisplay extracts runes from a display slice.
func RunesFromDisplay(display []DisplayRune) []rune {
	runes := make([]rune, len(display))
	for i, dr := range display {
		runes[i] = dr.Rune
	}
	return runes
}

// SuffixSpanStyle returns foreground style for one suffix glyph rune.
func SuffixSpanStyle(r rune, suffix RowSuffix, jobStatus, cursorStyleKey string, th theme.Theme, chromeBlocked bool) (tcell.Style, bool) {
	switch {
	case r == suffix.JobGlyph && suffix.JobGlyph != 0:
		return th.JobsIconStyle(jobStatus), true
	case r == th.SymbolFilelistNew() && suffix.NewFileTier != NewFileMarkNone:
		base := th.PanelRowIndicatorNew
		if suffix.NewFileTier == NewFileMarkPrevious {
			base = th.PanelRowIndicatorNewPrevious
		}
		return tcell.StyleDefault.Foreground(th.PanelRowSuffixIconForeground(cursorStyleKey, base)), true
	case r == th.SymbolFilelistSelectionSubtree() && suffix.SubtreeSelection:
		base := th.PanelRowIndicatorSelectionSubtree
		if chromeBlocked {
			base = th.PanelBlockedRowSelected
		}
		return tcell.StyleDefault.Foreground(th.PanelRowSuffixIconForeground(cursorStyleKey, base)), true
	default:
		return tcell.StyleDefault, false
	}
}

// ListingSuffixSpans returns styled spans for trailing row suffix glyphs.
func ListingSuffixSpans(
	entry localfs.Entry,
	nameWidth int,
	showIcons bool,
	suffix RowSuffix,
	jobStatus string,
	th theme.Theme,
	chromeBlocked bool,
	cursorStyleKey string,
	nameBGAt func(displayIndex int) tcell.Style,
) []primitive.Span {
	subtree := suffix.SubtreeSelection && entry.Type == localfs.EntryDirectory
	if suffix.JobGlyph == 0 && suffix.NewFileTier == NewFileMarkNone && !subtree {
		return nil
	}
	display := EntryDisplayRunes(entry, nameWidth, showIcons, suffix, th)
	suf := SuffixDecorationLen(nameWidth, suffix, entry, th)
	decStart := len(display) - suf
	if decStart < 0 || decStart >= len(display) {
		return nil
	}
	var spans []primitive.Span
	for i := decStart; i < len(display); i++ {
		r := display[i].Rune
		spanStyle, ok := SuffixSpanStyle(r, suffix, jobStatus, cursorStyleKey, th, chromeBlocked)
		if !ok {
			continue
		}
		_, rowBG, _ := nameBGAt(i).Decompose()
		spanFG, _, _ := spanStyle.Decompose()
		spans = append(spans, primitive.Span{
			Start: i,
			End:   i + 1,
			Style: tcell.StyleDefault.Foreground(spanFG).Background(rowBG),
		})
	}
	return spans
}
