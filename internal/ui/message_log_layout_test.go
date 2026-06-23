package ui

import "testing"

func TestMessageLogWrapColsForLayoutMatchesMessagesView(t *testing.T) {
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 10},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 10},
	}
	// MergeTwinPanelRects width 80 → contentW 76 → msgW 67 (messagesListColTime = 9).
	want := 67
	if got := MessageLogWrapColsForLayout(layout, SplitHorizontal); got != want {
		t.Fatalf("MessageLogWrapColsForLayout() = %d, want %d", got, want)
	}
}

func TestMessageLogWrapColsForLayoutTooSmallUsesFallback(t *testing.T) {
	layout := Layout{TooSmall: true}
	if got := MessageLogWrapColsForLayout(layout, SplitHorizontal); got != MessageLogWrapRunes {
		t.Fatalf("got %d, want fallback %d", got, MessageLogWrapRunes)
	}
}
