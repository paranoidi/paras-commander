package uiscrollbar

import (
	"fmt"
	"strings"
)

// Style selects how a vertical scroll indicator is painted on panel lists.
type Style string

const (
	StyleNone  Style = "none"
	StyleThumb Style = "thumb"
	StyleBar   Style = "bar"
)

// ParseStyle parses panel_scrollbar from config TOML.
func ParseStyle(value string) (Style, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return StyleThumb, nil
	case string(StyleNone):
		return StyleNone, nil
	case string(StyleThumb):
		return StyleThumb, nil
	case string(StyleBar):
		return StyleBar, nil
	default:
		return StyleThumb, fmt.Errorf("unknown panel scrollbar style %q", value)
	}
}

// EffectiveStyle returns s, or StyleThumb when unset/invalid.
func EffectiveStyle(s Style) Style {
	switch s {
	case StyleNone, StyleThumb, StyleBar:
		return s
	default:
		return StyleThumb
	}
}

// TOMLValue returns the canonical panel_scrollbar string for persistence.
func TOMLValue(s Style) string {
	switch EffectiveStyle(s) {
	case StyleNone:
		return string(StyleNone)
	case StyleBar:
		return string(StyleBar)
	default:
		return string(StyleThumb)
	}
}

// DialogRadio describes one row in the Configuration scrollbar-style radio group.
type DialogRadio struct {
	Style    Style
	Label    string
	Shortcut rune
}

// DialogRadios is the canonical radio list for the Configuration dialog and its key handler.
func DialogRadios() []DialogRadio {
	return []DialogRadio{
		{StyleNone, "None", 'n'},
		{StyleThumb, "Thumb", 'u'},
		{StyleBar, "Bar", 'r'},
	}
}
