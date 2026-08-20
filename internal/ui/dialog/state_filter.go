package dialog

import "github.com/paranoidi/paras-commander/internal/panel"

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
	FilesOnly     bool
	DirsOnly      bool
	CaseSensitive bool
	Focus         int // 0-2=mode radios, 3=pattern, 4=files only, 5=dirs only, 6=case sensitive, then OK, Cancel

	// Live match-count preview shown right-aligned on the Pattern row, recomputed on every state
	// change. PreviewShow is false while the pattern is empty or fails to compile.
	PreviewFiles   int
	PreviewFolders int
	PreviewShow    bool
}
