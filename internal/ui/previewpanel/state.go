package previewpanel

import "github.com/gdamore/tcell/v2"

// Phase is the async preview lifecycle.
type Phase int

const (
	PhaseIdle Phase = iota
	PhasePending
	PhaseRunning
	PhaseDone
)

// Source identifies how preview body cells were produced.
type Source int

const (
	// SourceExternalANSI uses CombinedText parsed through AnsiStyledCells.
	SourceExternalANSI Source = iota
	// SourceInternalHighlighted uses HighlightedCells from Chroma.
	SourceInternalHighlighted
)

// State holds scrollable file preview content and wrap cache.
type State struct {
	Open bool
	// Phase is used while Open; draw uses a snapshot each frame.
	Phase Phase
	// Path is the absolute file path being previewed.
	Path string
	// TitleBase is shown in the panel title (usually filepath.Base(Path)).
	TitleBase string
	Source    Source
	// CombinedText is stdout plus optional stderr (external mode); may contain ANSI escapes.
	CombinedText string
	// HighlightedCells is the flat pre-wrap body for internal Chroma highlighting.
	HighlightedCells []AnsiCell
	// GutterWidth is the visual width of the line-number gutter (internal mode, line_numbers on).
	GutterWidth int
	Scroll      int
	// ExitCode is set when Phase == PhaseDone and ErrorMsg == "" (external subprocess).
	ExitCode int
	// ErrorMsg is set for launch/read failures or non-zero external exit.
	ErrorMsg string
	// ChromaStyle is the Chroma style name used to generate HighlightedCells (internal mode only).
	// Empty when content was produced by an external renderer. Used to tint the panel border.
	ChromaStyle string
	// BodyHeld keeps the previous body visible while a new file loads.
	BodyHeld bool
	// IsDiff is true when the preview is showing a git diff instead of file content.
	IsDiff bool
	// IsMarkdown is true when body content was produced by the rendered-markdown formatter
	// (mdformat) rather than raw/diff/chroma text. Borderless (fullscreen) draw adds a
	// 1-space left/right margin when this is set.
	IsMarkdown bool
	// DiffHunkLines holds 0-based source line numbers where each contiguous +/- change
	// run begins (first added/removed line of the chunk). Used by Ctrl+Alt+J/K.
	DiffHunkLines []int
	// GitStatusText is a short git-status label shown next to the title (e.g. "no changes", "ignored").
	GitStatusText string
	// GitStatusThemeKey is the panel.git.* theme key used to color GitStatusText.
	GitStatusThemeKey string
	// Search tracks incremental "/" search state and matches within this preview.
	Search SearchState
	// ImagePayload is a raw sixel DCS or Kitty APC sequence for image previews (empty for text).
	ImagePayload string
	// ImagePxW / ImagePxH are the encoded image dimensions in pixels.
	ImagePxW int
	ImagePxH int
	// ImageProtocol identifies which graphics protocol ImagePayload uses.
	ImageProtocol ImageProtocol
	// ImageUnicodePlaceholder is true when ImagePayload was encoded for Kitty
	// Unicode-placeholder display (U=1): Draw paints placeholder cells instead of
	// recording a cursor-relative placement. Decided upstream in the preview request
	// (tmux + an outer terminal known to support placeholders) — Draw must not
	// re-derive it from the environment.
	ImageUnicodePlaceholder bool

	wrappedLines     [][]AnsiCell
	wrapWidth        int
	wrapStyleKey     uint64
	wrapSource       Source
	wrapCombinedText string
	wrapCellsLen     int
	wrapGutterWidth  int
	wrapHighlightKey uint64
	wrapSearchKey    uint64

	// highlightCacheKey fingerprints HighlightedCells styles; bump when cells are replaced.
	highlightCacheKey uint64
}

// SetHighlightedCells replaces internal Chroma cells and invalidates the wrap cache.
func (st *State) SetHighlightedCells(cells []AnsiCell) {
	st.HighlightedCells = cells
	st.highlightCacheKey = highlightCacheKey(cells)
	st.clearWrapCache()
}

func (st *State) clearWrapCache() {
	st.wrappedLines = nil
	st.wrapWidth = 0
	st.wrapStyleKey = 0
	st.wrapSource = 0
	st.wrapCombinedText = ""
	st.wrapCellsLen = 0
	st.wrapGutterWidth = 0
	st.wrapHighlightKey = 0
	st.wrapSearchKey = 0
}

func (st State) highlightedCacheKey() uint64 {
	if st.highlightCacheKey != 0 {
		return st.highlightCacheKey
	}
	return highlightCacheKey(st.HighlightedCells)
}

// HighlightedCacheKey fingerprints internal Chroma highlight styles for cache invalidation.
func (st State) HighlightedCacheKey() uint64 {
	return st.highlightedCacheKey()
}

// SourceLineToScrollOffset returns the wrapped-line index where source line lineN begins.
// Used to jump to a diff hunk position. base is only used for ANSI text; rune widths
// determine wrapping regardless of color.
func (st State) SourceLineToScrollOffset(lineN, textWidth int, base tcell.Style) int {
	if lineN <= 0 {
		return 0
	}
	var cells []AnsiCell
	switch st.Source {
	case SourceInternalHighlighted:
		cells = st.HighlightedCells
	default:
		cells = AnsiStyledCells(st.CombinedText, base)
	}
	// Walk cells counting \n; stop after lineN newlines to find start of that source line.
	srcNewlines := 0
	endIdx := -1
	for i, c := range cells {
		if c.R == '\n' {
			srcNewlines++
			if srcNewlines == lineN {
				endIdx = i + 1
				break
			}
		}
	}
	gw := 0
	if st.Source == SourceInternalHighlighted {
		gw = st.GutterWidth
	}
	wrapFn := func(prefix []AnsiCell) [][]AnsiCell {
		if gw > 0 {
			return WrapAnsiCellsWithGutter(prefix, textWidth, gw)
		}
		return WrapAnsiCells(prefix, textWidth)
	}
	if endIdx < 0 {
		// lineN is beyond content; return index of last wrapped line.
		wrapped := wrapFn(cells)
		return max(0, len(wrapped)-1)
	}
	wrapped := wrapFn(cells[:endIdx])
	if len(wrapped) == 0 {
		return 0
	}
	return len(wrapped) - 1
}
