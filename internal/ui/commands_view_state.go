package ui

// EnsureSelectionVisible clamps selection and list scroll for the commands list.
func (s *CommandsViewState) EnsureSelectionVisible(total int, visibleRows int) {
	ensureSelectionVisible(&s.Selected, &s.ListScroll, total, visibleRows)
}
