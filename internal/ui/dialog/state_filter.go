package dialog

import (
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
)

// File-type preset indexes for FilterDialogState.Types (order matches config [filter.types]).
const (
	FilterTypeVideos = iota
	FilterTypePictures
	FilterTypeArchives
	FilterTypeAudio
	FilterTypeDocuments
	FilterTypeCount
)

// Alt+letter mnemonics shared by the Filter and Select group dialogs: the single source for
// both renderers and key handlers. Invert is Filter-only; FullPath is Select group-only.
const (
	PatternMnemonicShell       = 'S'
	PatternMnemonicRegex       = 'R'
	PatternMnemonicSimple      = 'L'
	PatternMnemonicFilesOnly   = 'Y'
	PatternMnemonicDirsOnly    = 'D'
	PatternMnemonicCase        = 'E'
	PatternMnemonicIncludeMeta = 'M'
	PatternMnemonicOnlyMeta    = 'N'
	PatternMnemonicInvert      = 'I'
	PatternMnemonicFullPath    = 'H'
)

// PatternAltIsMnemonic reports whether r (any case) is an Alt mnemonic of the Filter or Select
// group dialog, so the pattern input leaves Alt+r to the dialog instead of readline editing.
func PatternAltIsMnemonic(r rune) bool {
	r = unicode.ToUpper(r)
	switch r {
	case PatternMnemonicShell, PatternMnemonicRegex, PatternMnemonicSimple, PatternMnemonicFilesOnly,
		PatternMnemonicDirsOnly, PatternMnemonicCase, PatternMnemonicIncludeMeta, PatternMnemonicOnlyMeta,
		PatternMnemonicInvert, PatternMnemonicFullPath:
		return true
	}
	return slices.Contains(FilterTypeMnemonics[:], r)
}

// FilterTypeLabels are the type checkbox labels; FilterTypeMnemonics their Alt+letter shortcuts.
var (
	FilterTypeLabels    = [FilterTypeCount]string{"Videos", "Pictures", "Archives", "Audio", "Documents"}
	FilterTypeMnemonics = [FilterTypeCount]rune{'V', 'P', 'A', 'U', 'T'}
)

// PatternTypesDisabled reports whether the file-type checkboxes render disabled (and ignore
// toggles and matching) in the Filter and Select group dialogs: a directories-only pattern
// can never match a file.
func PatternTypesDisabled(dirsOnly bool) bool { return dirsOnly }

// FilterTypesRegexp OR-joins the [filter.types] regexps of the checked presets into one regexp
// and returns it with a comma-joined label of the checked names. It is the single builder for
// the Filter and Select group dialogs. Returns (nil, "") when nothing is checked; the config
// is validated, so a compile failure also drops the restriction. Types are ignored (nil, "")
// while PatternTypesDisabled(dirsOnly).
func FilterTypesRegexp(dirsOnly bool, checked [FilterTypeCount]bool, t config.FilterTypesConfig) (*regexp.Regexp, string) {
	if PatternTypesDisabled(dirsOnly) {
		return nil, ""
	}
	rx := [FilterTypeCount]string{t.Videos, t.Pictures, t.Archives, t.Audio, t.Documents}
	var alts, labels []string
	for i, on := range checked {
		if on {
			alts = append(alts, "(?:"+rx[i]+")")
			labels = append(labels, FilterTypeLabels[i])
		}
	}
	if len(alts) == 0 {
		return nil, ""
	}
	re, err := regexp.Compile(strings.Join(alts, "|"))
	if err != nil {
		return nil, ""
	}
	return re, strings.Join(labels, ", ")
}

// FilterTarget selects what the Filter dialog narrows.
type FilterTarget int

const (
	FilterTargetPanel   FilterTarget = iota // the active panel's entries
	FilterTargetCompare                     // Compare view result rows (basename match)
	FilterTargetDedup                       // Find duplicates groups (any copy's basename matches)
)

// FilterDialogState is the renderable state for the panel Filter modal, which narrows the
// active panel's visible entries via a shell-glob/regexp/simple-substring pattern (installed as
// a panel.EntryFilter). The hint text shown under the pattern row on an invalid pattern is
// computed live from Text/PatternMode/CaseSensitive on every render rather than cached here.
type FilterDialogState struct {
	Open          bool
	Text          string
	TextCursor    int // rune offset of caret within Text (0..len(runes))
	TextScroll    int // first visible rune offset for horizontal scrolling
	PatternMode   panel.GroupPatternMode
	FilesOnly     bool // mutually exclusive with DirsOnly
	DirsOnly      bool
	CaseSensitive bool
	// MetaColumnCount is the active panel's meta column count at dialog-open time; 0 hides the
	// meta checkbox row entirely.
	MetaColumnCount    int
	IncludeMetaColumns bool
	OnlyMetaColumns    bool
	// Types are the checked file-type presets (indexed FilterTypeVideos..); Invert negates the match.
	Types  [FilterTypeCount]bool
	Invert bool
	// Target is what the dialog filters: the active panel, Compare rows or dedup groups.
	Target FilterTarget
	Focus  int // FilterFocus* constants (filter_hint.go), then OK, Cancel

	// Live match-count preview shown right-aligned on the Pattern row, recomputed on every state
	// change. PreviewShow is false while the pattern is empty or fails to compile.
	PreviewFiles   int
	PreviewFolders int
	PreviewShow    bool
}
