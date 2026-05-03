package ui

// EnsureSelectionVisible clamps selection and list scroll for the commands list.
func (s *CommandsViewState) EnsureSelectionVisible(total int, visibleRows int) {
	if total == 0 {
		s.Selected = 0
		s.ListScroll = 0
		return
	}
	if s.Selected >= total {
		s.Selected = total - 1
	}
	if s.Selected < 0 {
		s.Selected = 0
	}
	if visibleRows <= 0 {
		return
	}
	if s.Selected < s.ListScroll {
		s.ListScroll = s.Selected
	}
	if s.Selected >= s.ListScroll+visibleRows {
		s.ListScroll = s.Selected - visibleRows + 1
	}
	maxScroll := max(0, total-visibleRows)
	if s.ListScroll > maxScroll {
		s.ListScroll = maxScroll
	}
	if s.ListScroll < 0 {
		s.ListScroll = 0
	}
}
