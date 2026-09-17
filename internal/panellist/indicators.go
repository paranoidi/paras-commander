// Package panellist implements shared file-list name-column suffix indicators (job, new, subtree).
package panellist

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// DisplayRune is one icon in the listing name column; NameIdx is -1 for decorations.
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
	JobIcon          rune
	NewFileTier      NewFileMarkTier
	RenameMark       bool
	SubtreeSelection bool
	// JobWrite is true when JobIcon marks a job's write (destination) tree rather
	// than its read (source) tree; see Theme.PanelJobMarkStyle.
	JobWrite bool
	// Working marks a directory whose async navigation load has been pending longer than the
	// working-indicator delay; see Theme.IconFilelistWorking.
	Working bool
	// Pinned marks an entry present in the app's pin list; see Theme.IconPin.
	Pinned bool
}

// SuffixDecorationLen returns how many trailing runes are reserved for row suffix indicators.
func SuffixDecorationLen(width int, suffix RowSuffix, entry localfs.Entry, th theme.Theme) int {
	n := 0
	if suffix.JobIcon != 0 && width > n+2 {
		n += 2
	}
	if suffix.NewFileTier != NewFileMarkNone && width > n+2 {
		n += 2
	}
	if suffix.RenameMark && width > n+2 {
		n += 2
	}
	if suffix.Working && width > n+2 {
		n += 2
	}
	if suffix.Pinned && width > n+2 {
		n += 2
	}
	subtree := suffix.SubtreeSelection && entry.Type == localfs.EntryDirectory
	if subtree && width > n+2 {
		n += 2
	}
	if entry.AccessDenied && width > n+2 {
		n += 2
	}
	return n
}

// EntryDisplayRunes builds the display rune slice for an entry name, including decorations and suffix icons.
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
		truncated = append(truncated, DisplayRune{Rune: primitive.Ellipsis, NameIdx: -1})
		truncated = append(truncated, body[len(body)-suffixWidth:]...)
		core = truncated
	}

	if suffixLen == 0 {
		return core
	}
	out := make([]DisplayRune, 0, len(core)+suffixLen)
	out = append(out, core...)
	used := 0
	if suffix.JobIcon != 0 && width > used+2 {
		out = append(out, DisplayRune{Rune: ' ', NameIdx: -1}, DisplayRune{Rune: suffix.JobIcon, NameIdx: -1})
		used += 2
	}
	if suffix.NewFileTier != NewFileMarkNone && width > used+2 {
		out = append(out, DisplayRune{Rune: ' ', NameIdx: -1}, DisplayRune{Rune: th.IconFilelistNew(), NameIdx: -1})
		used += 2
	}
	if suffix.RenameMark && width > used+2 {
		out = append(out, DisplayRune{Rune: ' ', NameIdx: -1}, DisplayRune{Rune: th.IconFilelistRenamed(), NameIdx: -1})
		used += 2
	}
	if suffix.Working && width > used+2 {
		out = append(out, DisplayRune{Rune: ' ', NameIdx: -1}, DisplayRune{Rune: th.IconFilelistWorking(), NameIdx: -1})
		used += 2
	}
	if suffix.Pinned && width > used+2 {
		out = append(out, DisplayRune{Rune: ' ', NameIdx: -1}, DisplayRune{Rune: pinIconRune(th), NameIdx: -1})
		used += 2
	}
	if subtree && width > used+2 {
		out = append(out, DisplayRune{Rune: ' ', NameIdx: -1}, DisplayRune{Rune: th.IconFilelistSelectionSubtree(), NameIdx: -1})
		used += 2
	}
	if entry.AccessDenied && width > used+2 {
		out = append(out, DisplayRune{Rune: ' ', NameIdx: -1}, DisplayRune{Rune: th.IconFilelistNoPermission(), NameIdx: -1})
	}
	return out
}

// pinIconRune returns th.IconPinRune() for row-suffix icon slots, which are always one
// rune wide (IconPin itself is a string since it is shared with the multi-rune-capable
// menubar pin badge).
func pinIconRune(th theme.Theme) rune {
	return th.IconPinRune()
}

// RunesFromDisplay extracts runes from a display slice.
func RunesFromDisplay(display []DisplayRune) []rune {
	runes := make([]rune, len(display))
	for i, dr := range display {
		runes[i] = dr.Rune
	}
	return runes
}

// SuffixSpanStyle returns foreground style for one suffix icon rune.
func SuffixSpanStyle(r rune, suffix RowSuffix, entry localfs.Entry, jobStatus, cursorStyleKey string, th theme.Theme, chromeBlocked bool) (tcell.Style, bool) {
	switch {
	case r == suffix.JobIcon && suffix.JobIcon != 0:
		base := th.PanelJobMarkStyle(jobStatus, suffix.JobWrite)
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, base)), true
	case r == th.IconFilelistNew() && suffix.NewFileTier != NewFileMarkNone:
		base := th.PanelRowMarkNew
		if suffix.NewFileTier == NewFileMarkPrevious {
			base = th.PanelRowMarkNewPrevious
		}
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, base)), true
	case r == th.IconFilelistRenamed() && suffix.RenameMark:
		base := th.PanelRowMarkRenamed
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, base)), true
	case r == th.IconFilelistWorking() && suffix.Working:
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, th.PanelIconFolderScanning)), true
	case r == pinIconRune(th) && suffix.Pinned:
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, th.PanelRowMarkPinned)), true
	case r == th.IconFilelistSelectionSubtree() && suffix.SubtreeSelection:
		base := th.PanelRowMarkSelectionSubtree
		if chromeBlocked {
			base = th.PanelBlockedRowSelected
		}
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, base)), true
	case r == th.IconFilelistNoPermission() && entry.AccessDenied:
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, th.PanelRowMarkNoPermission)), true
	default:
		return tcell.StyleDefault, false
	}
}

// ListingSuffixSpans returns styled spans for trailing row suffix icons.
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
	if suffix.JobIcon == 0 && suffix.NewFileTier == NewFileMarkNone && !suffix.RenameMark && !subtree &&
		!suffix.Working && !suffix.Pinned && !entry.AccessDenied {
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
		spanStyle, ok := SuffixSpanStyle(r, suffix, entry, jobStatus, cursorStyleKey, th, chromeBlocked)
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
