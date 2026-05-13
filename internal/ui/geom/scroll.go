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
