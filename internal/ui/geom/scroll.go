package geom

// ScrollOffset returns the scroll start so that selected is centered in the
// viewport when possible, clamped to valid bounds. total is the number of items,
// visibleRows is the viewport size, and selected is the cursor index.
func ScrollOffset(selected, visibleRows, total int) int {
	if total <= visibleRows {
		return 0
	}
	start := selected - visibleRows/2
	if start < 0 {
		return 0
	}
	lastStart := total - visibleRows
	if start > lastStart {
		return lastStart
	}
	return start
}

// ScrollOffsetEdge returns the scroll offset after edge-margin scrolling.
// currentScroll is preserved when the cursor already has at least margin rows
// above and below within the viewport.
func ScrollOffsetEdge(cursor, currentScroll, visibleRows, total, margin int) int {
	if visibleRows <= 0 || total <= 0 {
		return 0
	}
	if total <= visibleRows {
		return 0
	}
	maxOffset := total - visibleRows
	scroll := currentScroll
	if scroll > maxOffset {
		scroll = maxOffset
	}
	if scroll < 0 {
		scroll = 0
	}
	effMargin := EffectiveEdgeMargin(visibleRows, margin)
	pos := cursor - scroll
	topMargin := pos
	bottomMargin := visibleRows - 1 - pos
	if topMargin >= effMargin && bottomMargin >= effMargin {
		return scroll
	}
	var target int
	if topMargin < effMargin {
		target = cursor - effMargin
	} else {
		target = cursor - visibleRows + effMargin + 1
	}
	if target < 0 {
		target = 0
	}
	if target > maxOffset {
		target = maxOffset
	}
	if cursor < target {
		target = cursor
	}
	if cursor >= target+visibleRows {
		target = cursor - visibleRows + 1
	}
	return target
}

// EffectiveEdgeMargin returns the margin used for edge scrolling after viewport clamping.
func EffectiveEdgeMargin(visibleRows, margin int) int {
	if margin < 0 {
		margin = 0
	}
	if visibleRows <= 1 {
		return 0
	}
	half := (visibleRows - 1) / 2
	if margin > half {
		return half
	}
	return margin
}
