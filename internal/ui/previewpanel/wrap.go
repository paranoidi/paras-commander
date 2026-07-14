package previewpanel

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

func styleCacheKey(s tcell.Style) uint64 {
	fg, bg, attrs := s.Decompose()
	h := uint64(attrs)
	h = h*31 + uint64(fg)
	h = h*31 + uint64(bg)
	return h
}

func (st *State) wrapCacheValid(textWidth int, base tcell.Style) bool {
	if st.wrappedLines == nil || st.wrapWidth != textWidth || st.wrapStyleKey != styleCacheKey(base) || st.wrapSource != st.Source {
		return false
	}
	if st.wrapGutterWidth != st.GutterWidth || st.wrapSearchKey != st.Search.cacheKey() {
		return false
	}
	switch st.Source {
	case SourceInternalHighlighted:
		key := st.highlightedCacheKey()
		return st.wrapCellsLen == len(st.HighlightedCells) && st.wrapHighlightKey == key
	default:
		return st.wrapCombinedText == st.CombinedText
	}
}

// bodyCellsRaw returns the flat pre-wrap body cell stream for st.Source, with no search
// highlighting applied. For SourceInternalHighlighted this returns HighlightedCells BY
// REFERENCE (same backing array reused across frames) — callers must not mutate it.
func (st *State) bodyCellsRaw(base tcell.Style) []AnsiCell {
	switch st.Source {
	case SourceInternalHighlighted:
		return st.HighlightedCells
	default:
		return AnsiStyledCells(st.CombinedText, base)
	}
}

// bodyCells returns the flat pre-wrap body cell stream with search-match highlighting
// baked into each matched cell's style, so wrapping (which may insert gutter/indent
// cells) can never misalign a rune-offset-based overlay applied after the fact.
func (st *State) bodyCells(base tcell.Style) []AnsiCell {
	return st.applySearchOverlay(st.bodyCellsRaw(base))
}

// applySearchOverlay returns cells with matched ranges restyled, always as a copy —
// bodyCellsRaw can return HighlightedCells by reference, and mutating it in place would
// permanently corrupt the stored Chroma highlighting even after search ends.
func (st *State) applySearchOverlay(cells []AnsiCell) []AnsiCell {
	if !st.Search.Active || len(st.Search.Matches) == 0 {
		return cells
	}
	out := make([]AnsiCell, len(cells))
	copy(out, cells)
	for mi, m := range st.Search.Matches {
		style := st.Search.MatchStyle
		if mi == st.Search.Current {
			style = st.Search.CurrentStyle
		}
		for i := m.Start; i < m.End && i < len(out); i++ {
			out[i].St = style
		}
	}
	return out
}

// WrapCacheSnapshot copies the wrapped-line cache from src when it matches this state's body.
func (st *State) WrapCacheSnapshot(src State) {
	if src.wrapSearchKey != st.Search.cacheKey() {
		return
	}
	switch st.Source {
	case SourceInternalHighlighted:
		if src.wrapCellsLen != len(st.HighlightedCells) || src.wrapSource != st.Source || src.wrapHighlightKey != st.highlightedCacheKey() {
			return
		}
	default:
		if src.wrapCombinedText != st.CombinedText || src.wrapSource != st.Source {
			return
		}
	}
	st.wrappedLines = src.wrappedLines
	st.wrapWidth = src.wrapWidth
	st.wrapStyleKey = src.wrapStyleKey
	st.wrapSource = src.wrapSource
	st.wrapCombinedText = src.wrapCombinedText
	st.wrapCellsLen = src.wrapCellsLen
	st.wrapGutterWidth = src.wrapGutterWidth
	st.wrapHighlightKey = src.wrapHighlightKey
	st.wrapSearchKey = src.wrapSearchKey
}

// CachedWrappedLineCount returns len(wrappedLines) when the layout cache matches textWidth.
func (st *State) CachedWrappedLineCount(textWidth int) (count int, ok bool) {
	if st.wrappedLines != nil && st.wrapWidth == textWidth {
		switch st.Source {
		case SourceInternalHighlighted:
			if st.wrapCellsLen == len(st.HighlightedCells) && st.wrapSource == st.Source && st.wrapHighlightKey == st.highlightedCacheKey() {
				return len(st.wrappedLines), true
			}
		default:
			if st.wrapCombinedText == st.CombinedText && st.wrapSource == st.Source {
				return len(st.wrappedLines), true
			}
		}
	}
	return 0, false
}

// EnsureWrappedLines returns wrapped preview lines, reusing a cache when inputs are unchanged.
func (st *State) EnsureWrappedLines(textWidth int, base tcell.Style) [][]AnsiCell {
	if textWidth < 1 {
		textWidth = 1
	}
	if st.wrapCacheValid(textWidth, base) {
		return st.wrappedLines
	}
	cells := st.bodyCells(base)
	if st.Source == SourceInternalHighlighted && st.GutterWidth > 0 {
		st.wrappedLines = WrapAnsiCellsWithGutter(cells, textWidth, st.GutterWidth)
	} else {
		st.wrappedLines = WrapAnsiCells(cells, textWidth)
	}
	st.wrapWidth = textWidth
	st.wrapStyleKey = styleCacheKey(base)
	st.wrapSource = st.Source
	st.wrapCombinedText = st.CombinedText
	st.wrapCellsLen = len(st.HighlightedCells)
	st.wrapGutterWidth = st.GutterWidth
	st.wrapSearchKey = st.Search.cacheKey()
	if st.Source == SourceInternalHighlighted {
		st.highlightCacheKey = highlightCacheKey(st.HighlightedCells)
		st.wrapHighlightKey = st.highlightCacheKey
	}
	return st.wrappedLines
}

// WrappedLineCount returns the number of wrapped preview lines.
func (st *State) WrappedLineCount(textWidth int, base tcell.Style) int {
	if textWidth < 1 {
		textWidth = 1
	}
	if count, ok := st.CachedWrappedLineCount(textWidth); ok {
		return count
	}
	return len(st.EnsureWrappedLines(textWidth, base))
}

func previewWrappedLines(st State, textWidth int, base tcell.Style) [][]AnsiCell {
	if st.wrapCacheValid(textWidth, base) {
		return st.wrappedLines
	}
	cells := st.bodyCells(base)
	if st.Source == SourceInternalHighlighted && st.GutterWidth > 0 {
		return WrapAnsiCellsWithGutter(cells, textWidth, st.GutterWidth)
	}
	return WrapAnsiCells(cells, textWidth)
}

// TotalLines returns wrapped line count for scroll metrics.
func TotalLines(st State, textWidth int, base tcell.Style) int {
	if textWidth < 1 {
		textWidth = 1
	}
	if !hasDrawableBody(st) {
		return 0
	}
	return st.WrappedLineCount(textWidth, base)
}

func hasDrawableBody(st State) bool {
	if st.Source == SourceInternalHighlighted && len(st.HighlightedCells) > 0 {
		return true
	}
	return strings.TrimSpace(st.CombinedText) != ""
}

func highlightCacheKey(cells []AnsiCell) uint64 {
	h := uint64(len(cells)) * 2654435761
	if len(cells) == 0 {
		return h
	}
	stride := 1
	if len(cells) > 4096 {
		stride = len(cells) / 4096
		if stride < 1 {
			stride = 1
		}
	}
	for i := 0; i < len(cells); i += stride {
		h ^= styleCacheKey(cells[i].St)
		h = h*31 + uint64(cells[i].R)
	}
	last := cells[len(cells)-1]
	h ^= styleCacheKey(last.St)
	h = h*31 + uint64(last.R)
	return h
}
