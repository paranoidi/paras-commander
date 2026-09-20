package ui

// EnsureSelectionVisible clamps selection and list scroll for the messages list.
func (s *MessagesViewState) EnsureSelectionVisible(total int, visibleRows int) {
	ensureSelectionVisible(&s.Selected, &s.ListScroll, total, visibleRows)
}
