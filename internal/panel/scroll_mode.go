package panel

import (
	"fmt"
	"strings"
)

// ScrollMode selects how file-list navigation adjusts ScrollOffset.
type ScrollMode string

const (
	ScrollModeMinimal ScrollMode = "minimal"
	ScrollModeCenter  ScrollMode = "center"
	ScrollModeEdge    ScrollMode = "edge"
)

// ParseScrollMode parses scroll_mode from config TOML.
func ParseScrollMode(value string) (ScrollMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ScrollModeEdge, nil
	case string(ScrollModeMinimal):
		return ScrollModeMinimal, nil
	case string(ScrollModeCenter):
		return ScrollModeCenter, nil
	case string(ScrollModeEdge):
		return ScrollModeEdge, nil
	default:
		return ScrollModeEdge, fmt.Errorf("unknown scroll mode %q", value)
	}
}

// EffectiveScrollMode returns m, or ScrollModeEdge when unset/invalid.
func EffectiveScrollMode(m ScrollMode) ScrollMode {
	switch m {
	case ScrollModeMinimal, ScrollModeCenter, ScrollModeEdge:
		return m
	default:
		return ScrollModeEdge
	}
}

// ScrollModeTOMLValue returns the canonical scroll_mode string for TOML persistence.
func ScrollModeTOMLValue(m ScrollMode) string {
	switch EffectiveScrollMode(m) {
	case ScrollModeMinimal:
		return string(ScrollModeMinimal)
	case ScrollModeCenter:
		return string(ScrollModeCenter)
	default:
		return string(ScrollModeEdge)
	}
}

// ScrollModeDialogRadio describes one row in the Configuration scroll-mode radio group.
type ScrollModeDialogRadio struct {
	Mode     ScrollMode
	Label    string
	Shortcut rune
}

// ScrollModeDialogRadios is the canonical radio list for the Configuration dialog and its key handler.
func ScrollModeDialogRadios() []ScrollModeDialogRadio {
	return []ScrollModeDialogRadio{
		{ScrollModeMinimal, "Minimal", 'i'},
		{ScrollModeEdge, "Edge", 'e'},
		{ScrollModeCenter, "Center", 'n'},
	}
}
