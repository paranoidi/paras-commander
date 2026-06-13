package uiscrollbar

// Metrics holds scroll geometry for painting a vertical indicator.
type Metrics struct {
	Total, Visible, Offset int
	ThumbStart, ThumbSize  int
	// ThumbDotRow is the StyleThumb indicator row mapped across the full track height.
	ThumbDotRow int
}

// ComputeMetrics returns scroll metrics for total items, visible rows, and scroll offset.
// ok is false when no indicator is needed (nothing to scroll).
func ComputeMetrics(total, visible, offset int) (Metrics, bool) {
	if visible <= 0 || total <= visible {
		return Metrics{}, false
	}
	if offset < 0 {
		offset = 0
	}
	maxOffset := total - visible
	if offset > maxOffset {
		offset = maxOffset
	}
	thumbSize := visible * visible / total
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > visible {
		thumbSize = visible
	}
	thumbStart := 0
	if maxOffset > 0 {
		thumbStart = offset * (visible - thumbSize) / maxOffset
	}
	if thumbStart < 0 {
		thumbStart = 0
	}
	if thumbStart+thumbSize > visible {
		thumbStart = visible - thumbSize
	}
	dotRow := 0
	if maxOffset > 0 {
		dotRow = offset * (visible - 1) / maxOffset
	}
	if dotRow < 0 {
		dotRow = 0
	}
	if dotRow >= visible {
		dotRow = visible - 1
	}
	return Metrics{
		Total:       total,
		Visible:     visible,
		Offset:      offset,
		ThumbStart:  thumbStart,
		ThumbSize:   thumbSize,
		ThumbDotRow: dotRow,
	}, true
}
