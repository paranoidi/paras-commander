// Package panellist implements shared file-list name-column suffix indicators (job, new, subtree).
package panellist

import (
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// DisplayRune is one icon in the listing name column; NameIdx is -1 for decorations. Cut marks
// where a shortened name was cut (an abbreviated word's last rune, or an Ellipsis).
type DisplayRune struct {
	Rune    rune
	NameIdx int
	Cut     bool
}

// NewFileMarkTier selects new-file suffix coloring (latest, previous or older batch).
type NewFileMarkTier int

const (
	NewFileMarkNone NewFileMarkTier = iota
	NewFileMarkLatest
	NewFileMarkPrevious
	NewFileMarkOlder
)

// JobSuffix is the job part of RowSuffix, painted after the name as <job><queued><operation>.
type JobSuffix struct {
	// JobIcon is icons.filelist.job; zero means no active (scanning/running/decision) job is
	// working on the row. A queued or paused job leaves it zero but may still set
	// JobQueuedIcon / JobOpIcon.
	JobIcon rune
	// JobQueuedIcon is the jobs-list queued icon, set while the matched job's status is
	// still queued; zero otherwise.
	JobQueuedIcon rune
	// JobOpIcon is icons.filelist.move (move/flatten, source rows only) or icons.filelist.delete
	// (delete); zero for copy/extract jobs.
	JobOpIcon rune
	// JobOpStyle is the foreground style for JobOpIcon (panel.row.mark.job.move / .delete).
	JobOpStyle tcell.Style
	// JobWrite is true when JobIcon marks a job's write (destination) tree rather
	// than its read (source) tree; see Theme.PanelJobMarkStyle.
	JobWrite bool
}

// RowSuffix selects which trailing indicators to reserve and paint on a listing row.
type RowSuffix struct {
	JobSuffix
	NewFileTier      NewFileMarkTier
	RenameMark       bool
	SubtreeSelection bool
	// Working marks a directory whose async navigation load has been pending longer than the
	// working-indicator delay; see Theme.IconFilelistWorking.
	Working bool
	// Pinned marks an entry present in the app's pin list; see Theme.IconPin.
	Pinned bool
	// RangeAnchor marks the pending range-select start row; see Theme.IconFilelistRangeAnchor.
	RangeAnchor bool
	// OtherMatch marks a row whose name also exists in the other panel's directory;
	// OtherMatchCursor additionally marks the name under the other panel's cursor.
	OtherMatch, OtherMatchCursor bool
}

// SuffixSlotCells is the width of one trailing suffix indicator: a separating space plus the icon.
const SuffixSlotCells = 2

// maxSuffixSlots is how many trailing suffix indicators a row can carry (suffixIcons).
const maxSuffixSlots = 11

// suffixSlots collects a row's trailing suffix icons in display order.
type suffixSlots struct {
	icons [maxSuffixSlots]rune
	n     int
	width int
}

// fits reports whether an indicator that is on gets a slot: each takes SuffixSlotCells and the
// row keeps at least one cell for the name.
func (s *suffixSlots) fits(on bool) bool {
	return on && s.width > (s.n+1)*SuffixSlotCells
}

func (s *suffixSlots) push(icon rune) {
	s.icons[s.n] = icon
	s.n++
}

// suffixIcons returns the row's trailing suffix icons that fit in width, in display order.
func suffixIcons(width int, suffix RowSuffix, entry localfs.Entry, th theme.Theme) suffixSlots {
	s := suffixSlots{width: width}
	for _, icon := range [...]rune{suffix.JobIcon, suffix.JobQueuedIcon, suffix.JobOpIcon} {
		if s.fits(icon != 0) {
			s.push(icon)
		}
	}
	if s.fits(suffix.NewFileTier != NewFileMarkNone) {
		s.push(th.IconFilelistNew())
	}
	if s.fits(suffix.RenameMark) {
		s.push(th.IconFilelistRenamed())
	}
	if s.fits(suffix.Working) {
		s.push(th.IconFilelistWorking())
	}
	if s.fits(suffix.Pinned) {
		s.push(pinIconRune(th))
	}
	if s.fits(suffix.RangeAnchor) {
		s.push(th.IconFilelistRangeAnchor())
	}
	if s.fits(suffix.OtherMatch) {
		s.push(th.IconFilelistOtherMatch())
	}
	if s.fits(suffix.SubtreeSelection && entry.Type == localfs.EntryDirectory) {
		s.push(th.IconFilelistSelectionSubtree())
	}
	if s.fits(entry.AccessDenied) {
		s.push(th.IconFilelistNoPermission())
	}
	return s
}

// SuffixDecorationLen returns how many trailing runes are reserved for row suffix indicators.
func SuffixDecorationLen(width int, suffix RowSuffix, entry localfs.Entry, th theme.Theme) int {
	return suffixIcons(width, suffix, entry, th).n * SuffixSlotCells
}

// EntryDisplayRunes builds the display rune slice for an entry name, including decorations and suffix icons.
func EntryDisplayRunes(entry localfs.Entry, width int, showFileIcons bool, suffix RowSuffix, th theme.Theme) []DisplayRune {
	slots := suffixIcons(width, suffix, entry, th)
	suffixLen := slots.n * SuffixSlotCells
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

	// Widths are terminal cells, not runes: CJK names have 2-cell glyphs.
	var core []DisplayRune
	if displayCells(body) <= innerW {
		core = body
	} else if innerW <= 3 {
		core = body[:takeCellsLeft(body, innerW)]
	} else {
		// Prefix and symlink '@' are one cell each and stay; the name shortens word by word.
		symlink := entry.Type == localfs.EntrySymlink
		nameW := innerW - 1
		if symlink {
			nameW--
		}
		core = make([]DisplayRune, 0, len(body))
		core = append(core, body[0])
		core = append(core, shortenName(entryRunes, nameW, entry.Type != localfs.EntryDirectory)...)
		if symlink {
			core = append(core, body[len(body)-1])
		}
	}

	if suffixLen == 0 {
		return core
	}
	out := make([]DisplayRune, 0, len(core)+suffixLen)
	out = append(out, core...)
	for _, icon := range slots.icons[:slots.n] {
		out = append(out, DisplayRune{Rune: ' ', NameIdx: -1}, DisplayRune{Rune: icon, NameIdx: -1})
	}
	return out
}

func displayCells(d []DisplayRune) int {
	n := 0
	for _, dr := range d {
		n += runewidth.RuneWidth(dr.Rune)
	}
	return n
}

// takeCellsLeft returns how many leading runes of d fit in width cells.
func takeCellsLeft(d []DisplayRune, width int) int {
	used := 0
	for i, dr := range d {
		used += runewidth.RuneWidth(dr.Rune)
		if used > width {
			return i
		}
	}
	return len(d)
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
	case r == suffix.JobQueuedIcon && suffix.JobQueuedIcon != 0:
		fg, _, _ := th.JobsIconStyle("queued").Decompose()
		base := tcell.StyleDefault.Foreground(fg)
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, base)), true
	case r == suffix.JobOpIcon && suffix.JobOpIcon != 0:
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, suffix.JobOpStyle)), true
	case r == th.IconFilelistNew() && suffix.NewFileTier != NewFileMarkNone:
		base := th.PanelRowMarkNew
		switch suffix.NewFileTier {
		case NewFileMarkPrevious:
			base = th.PanelRowMarkNewPrevious
		case NewFileMarkOlder:
			base = th.PanelRowMarkNewOlder
		}
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, base)), true
	case r == th.IconFilelistRenamed() && suffix.RenameMark:
		base := th.PanelRowMarkRenamed
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, base)), true
	case r == th.IconFilelistWorking() && suffix.Working:
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, th.PanelIconFolderScanning)), true
	case r == pinIconRune(th) && suffix.Pinned:
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, th.PanelRowMarkPinned)), true
	case r == th.IconFilelistRangeAnchor() && suffix.RangeAnchor:
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, th.PanelRowMarkRangeAnchor)), true
	case r == th.IconFilelistOtherMatch() && suffix.OtherMatch:
		base := th.PanelRowMarkOtherMatch
		if suffix.OtherMatchCursor {
			base = th.PanelRowMarkOtherMatchCursor
		}
		return tcell.StyleDefault.Foreground(th.PanelRowIconForeground(cursorStyleKey, base)), true
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

// ListingSuffixSpans returns styled spans for trailing row suffix icons. onBar (nil ok) reports
// whether a display cell sits on the disk-usage bar.
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
	onBar func(displayIndex int) bool,
) []primitive.Span {
	subtree := suffix.SubtreeSelection && entry.Type == localfs.EntryDirectory
	if suffix.JobIcon == 0 && suffix.JobQueuedIcon == 0 && suffix.JobOpIcon == 0 &&
		suffix.NewFileTier == NewFileMarkNone && !suffix.RenameMark && !subtree &&
		!suffix.Working && !suffix.Pinned && !suffix.RangeAnchor && !suffix.OtherMatch && !entry.AccessDenied {
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
		// The other-match mark is bright_black, same as the disk-usage bar background.
		if suffix.OtherMatch && onBar != nil && r == th.IconFilelistOtherMatch() && onBar(i) {
			spanFG = tcell.ColorBlack
		}
		spans = append(spans, primitive.Span{
			Start: i,
			End:   i + 1,
			Style: tcell.StyleDefault.Foreground(spanFG).Background(rowBG),
		})
	}
	return spans
}
