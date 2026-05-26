package ui

// AdjacentMessageLogIndex moves to the next logical message (skipping wrapped continuation lines).
// delta is +1 for newer (toward index 0) or -1 for older. Returns cur when no further message exists.
func AdjacentMessageLogIndex(entries []MessageLogEntry, cur, delta int) int {
	if len(entries) == 0 {
		return 0
	}
	if cur < 0 {
		cur = 0
	}
	if cur >= len(entries) {
		cur = len(entries) - 1
	}
	if delta > 0 {
		for i := cur + 1; i < len(entries); i++ {
			if entries[i].Time != "" {
				return i
			}
		}
		return cur
	}
	if delta < 0 {
		if entries[cur].Time == "" {
			for i := cur - 1; i >= 0; i-- {
				if entries[i].Time != "" {
					return i
				}
			}
			return 0
		}
		for i := cur - 1; i >= 0; i-- {
			if entries[i].Time != "" {
				return i
			}
		}
		return 0
	}
	return cur
}
