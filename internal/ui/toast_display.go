package ui

import "strings"

// FormatToastDisplay adds one leading and trailing ASCII space around non-empty toast
// text for on-screen rendering. Stored message/log text must not include this padding.
func FormatToastDisplay(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return " " + text + " "
}
