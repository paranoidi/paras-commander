package dialog

import "github.com/paranoidi/paras-commander/internal/panel"

// File-type preset indexes for FilterDialogState.Types (order matches config [filter.types]).
const (
	FilterTypeVideos = iota
	FilterTypePictures
	FilterTypeArchives
	FilterTypeAudio
	FilterTypeDocuments
	FilterTypeCount
)

// FilterTypeLabels are the type checkbox labels; FilterTypeMnemonics their Alt+letter shortcuts.
var (
	FilterTypeLabels    = [FilterTypeCount]string{"Videos", "Pictures", "Archives", "Audio", "Documents"}
	FilterTypeMnemonics = [FilterTypeCount]rune{'V', 'P', 'A', 'U', 'T'}
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
	// Compare targets the Compare view's result rows (basename match) instead of the active panel.
	Compare bool
	Focus   int // FilterFocus* constants (filter_hint.go), then OK, Cancel

	// Live match-count preview shown right-aligned on the Pattern row, recomputed on every state
	// change. PreviewShow is false while the pattern is empty or fails to compile.
	PreviewFiles   int
	PreviewFolders int
	PreviewShow    bool
}
