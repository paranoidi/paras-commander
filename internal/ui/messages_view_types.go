package ui

// MessageLogEntry is one timestamped status/toast line in the Messages view.
type MessageLogEntry struct {
	Time string // "15:04:05" (local wall clock when shown)
	Text string
	Urg  MessageUrgency
}

// MessagesViewState tracks selection and scroll for the Messages screen.
type MessagesViewState struct {
	Selected   int
	ListScroll int
}
