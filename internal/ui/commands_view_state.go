package ui

// commandsEntryLines is the list-row height of e: the command line plus one line per macro.
func commandsEntryLines(e CommandRunEntry) int {
	return 1 + len(e.Macros)
}

// commandsMaxListScroll returns the smallest first-row index that still fills visibleRows
// lines down to the last entry.
func commandsMaxListScroll(entries []CommandRunEntry, visibleRows int) int {
	used := 0
	for i := len(entries) - 1; i >= 0; i-- {
		used += commandsEntryLines(entries[i])
		if used > visibleRows {
			return min(i+1, len(entries)-1)
		}
	}
	return 0
}

// EnsureSelectionVisible clamps selection and list scroll for the commands list, whose rows
// are one or more lines tall (see commandsEntryLines).
func (s *CommandsViewState) EnsureSelectionVisible(entries []CommandRunEntry, visibleRows int) {
	ensureSelectionVisible(&s.Selected, &s.ListScroll, len(entries), 0)
	if len(entries) == 0 || visibleRows <= 0 {
		return
	}
	s.ListScroll = max(0, min(s.ListScroll, s.Selected))
	used := 0
	for i := s.ListScroll; i <= s.Selected; i++ {
		used += commandsEntryLines(entries[i])
	}
	for s.ListScroll < s.Selected && used > visibleRows {
		used -= commandsEntryLines(entries[s.ListScroll])
		s.ListScroll++
	}
	s.ListScroll = min(s.ListScroll, commandsMaxListScroll(entries, visibleRows))
}
