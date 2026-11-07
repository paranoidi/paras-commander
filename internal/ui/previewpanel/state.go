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
	// IsDir is true when Path is a directory being previewed by an external
	// [[preview.commands]] rule (see internal/preview.RunRules) rather than a file.
	IsDir  bool
	Source Source
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
	// Selection is the mouse text selection (fullscreen preview only).
	Selection Selection
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
	// ImageInTmux is true when the currently displayed image was requested with $TMUX set.
	// Draw uses it (only alongside ImageProtocol == ImageProtocolSixel) to show the
	// bottom-right "Sixel+Tmux" indicator — decided upstream in the preview request, same
	// rule as ImageUnicodePlaceholder: Draw must not re-derive tmux status from the
	// environment itself.
	ImageInTmux bool
	// ImageCapabilityUncertain is true when the currently displayed image's protocol/placeholder
	// capability was decided by low-confidence guesswork rather than an explicit user
	// confirmation (preview.CapabilityUncertain, decided upstream in the preview request — same
	// rule as ImageInTmux/ImageUnicodePlaceholder). Draw carries it into ImagePlacement so the
	// app layer can show the bottom-left "capability unknown" footer hint without re-deriving it.
	ImageCapabilityUncertain bool
	// ImageFirst, when true, draws the image above the CombinedText caption instead of
	// drawImageBody's default (caption above image, used by pc's own video-thumbnail
	// previews). Set for external [[preview.commands]] rule output whose image came before
	// its trailing text in the command's own stdout, so the rendered order matches the
	// command's actual output order instead of always reordering it.
	ImageFirst bool

	// layout is the row index of the body, immutable once built and shared between draw
	// snapshots of State (see wrapLayout).
	layout *wrapLayout
}

// SetHighlightedCells replaces internal Chroma cells and invalidates the wrap layout.
func (st *State) SetHighlightedCells(cells []AnsiCell) {
	st.HighlightedCells = cells
	st.layout = nil
}

// SourceLineToScrollOffset returns the wrapped-row index where source line lineN begins, or
// the last row when lineN is beyond the content. Used to jump to a diff hunk or search match.
// base is only used for ANSI text; rune widths determine wrapping regardless of color.
func (st State) SourceLineToScrollOffset(lineN, textWidth int, base tcell.Style) int {
	if textWidth < 1 {
		textWidth = 1
	}
	l := st.cachedLayout(textWidth)
	if l == nil {
		l = st.buildLayout(textWidth, base)
	}
	return l.rowOfLine(lineN)
}
