package ui

// commandsEntryLines is the list-row height of e: the command line plus one line per macro.
func commandsEntryLines(e CommandRunEntry) int {
	return 1 + len(e.Macros)
}

// CommandsDisplayIndex maps between CommandsList (chronological, append-only) indices and
// display rows (newest first). It is its own inverse, so the list panel, scroll math and
// the handler's cursor moves all share this one mapping. CommandsViewState.Selected is a
// CommandsList index; ListScroll is a display row.
func CommandsDisplayIndex(n, i int) int { return n - 1 - i }

// commandsMaxListScroll returns the smallest first display row that still fills visibleRows
// lines down to the last (oldest) row.
func commandsMaxListScroll(entries []CommandRunEntry, visibleRows int) int {
	n := len(entries)
	used := 0
	for d := n - 1; d >= 0; d-- {
		used += commandsEntryLines(entries[CommandsDisplayIndex(n, d)])
		if used > visibleRows {
			return min(d+1, n-1)
		}
	}
	return 0
}

// EnsureSelectionVisible clamps selection and list scroll for the commands list, whose rows
// are one or more lines tall (see commandsEntryLines) and drawn newest first.
func (s *CommandsViewState) EnsureSelectionVisible(entries []CommandRunEntry, visibleRows int) {
	n := len(entries)
	ensureSelectionVisible(&s.Selected, &s.ListScroll, n, 0)
	if n == 0 || visibleRows <= 0 {
		return
	}
	sel := CommandsDisplayIndex(n, s.Selected)
	s.ListScroll = max(0, min(s.ListScroll, sel))
	used := 0
	for d := s.ListScroll; d <= sel; d++ {
		used += commandsEntryLines(entries[CommandsDisplayIndex(n, d)])
	}
	for s.ListScroll < sel && used > visibleRows {
		used -= commandsEntryLines(entries[CommandsDisplayIndex(n, s.ListScroll)])
		s.ListScroll++
	}
	s.ListScroll = min(s.ListScroll, commandsMaxListScroll(entries, visibleRows))
}
