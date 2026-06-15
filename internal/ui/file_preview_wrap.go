package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// FilePreviewBodyStyle is the base text style for preview body rows.
func FilePreviewBodyStyle(styles theme.Theme, chromeBlocked bool) tcell.Style {
	bg := auxPanelContentBG(styles, chromeBlocked)
	return auxPanelBodyText(styles, chromeBlocked, bg)
}

func styleCacheKey(s tcell.Style) uint64 {
	fg, bg, attrs := s.Decompose()
	h := uint64(attrs)
	h = h*31 + colorCacheKey(fg)
	h = h*31 + colorCacheKey(bg)
	return h
}

func colorCacheKey(c tcell.Color) uint64 {
	return uint64(c)
}

func (st *FilePreviewState) wrapCacheValid(textWidth int, base tcell.Style) bool {
	return st.wrappedLines != nil &&
		st.wrapWidth == textWidth &&
		st.wrapStyleKey == styleCacheKey(base) &&
		st.wrapCombinedText == st.CombinedText
}

// EnsureWrappedLines returns wrapped preview lines, reusing a cache when text, width, and base style are unchanged.
func (st *FilePreviewState) EnsureWrappedLines(textWidth int, base tcell.Style) [][]AnsiCell {
	if textWidth < 1 {
		textWidth = 1
	}
	if st.wrapCacheValid(textWidth, base) {
		return st.wrappedLines
	}
	cells := AnsiStyledCells(st.CombinedText, base)
	st.wrappedLines = WrapAnsiCells(cells, textWidth)
	st.wrapWidth = textWidth
	st.wrapStyleKey = styleCacheKey(base)
	st.wrapCombinedText = st.CombinedText
	return st.wrappedLines
}

// WrappedLineCount returns the number of wrapped preview lines, using the layout cache when valid.
func (st *FilePreviewState) WrappedLineCount(textWidth int, base tcell.Style) int {
	if textWidth < 1 {
		textWidth = 1
	}
	if st.wrappedLines != nil && st.wrapWidth == textWidth && st.wrapCombinedText == st.CombinedText {
		return len(st.wrappedLines)
	}
	return len(st.EnsureWrappedLines(textWidth, base))
}

// previewWrappedLines returns wrapped lines for drawing, using a pre-warmed cache when possible.
func previewWrappedLines(st FilePreviewState, textWidth int, base tcell.Style) [][]AnsiCell {
	if st.wrapCacheValid(textWidth, base) {
		return st.wrappedLines
	}
	cells := AnsiStyledCells(st.CombinedText, base)
	return WrapAnsiCells(cells, textWidth)
}
