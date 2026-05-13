package panel

import (
	"fmt"
	"strings"
)

// ListFormat selects which trailing columns appear after the name (and optional Meta) and size.
type ListFormat int

const (
	// ListFormatMtime shows Name, Meta (optional), Size, Modified.
	ListFormatMtime ListFormat = iota
	// ListFormatPerm shows Name, Meta (optional), Size, Permissions (Unix-style mode string).
	ListFormatPerm
	// ListFormatBrief shows Name, Meta (optional), Size only.
	ListFormatBrief
)

// String returns a short user-facing label for transient messages and menus.
func (f ListFormat) String() string {
	switch f {
	case ListFormatMtime:
		return "Modified column"
	case ListFormatPerm:
		return "Permissions column"
	case ListFormatBrief:
		return "Name and size only"
	default:
		return "Modified column"
	}
}

// ParseListFormat parses a listing format from config TOML.
func ParseListFormat(value string) (ListFormat, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "mtime", "modified":
		return ListFormatMtime, nil
	case "perm", "permissions", "mode":
		return ListFormatPerm, nil
	case "brief", "minimal":
		return ListFormatBrief, nil
	default:
		return ListFormatMtime, fmt.Errorf("unknown listing format %q", value)
	}
}

// IterateListFormats returns formats in cycle order.
func IterateListFormats() []ListFormat {
	return []ListFormat{ListFormatMtime, ListFormatPerm, ListFormatBrief}
}

// EffectiveListFormat returns f, or ListFormatMtime if f is unset/invalid (zero value is Mtime).
func EffectiveListFormat(f ListFormat) ListFormat {
	switch f {
	case ListFormatMtime, ListFormatPerm, ListFormatBrief:
		return f
	default:
		return ListFormatMtime
	}
}

// ListFormatDialogRadio describes one row in the listing format dialog (label must include shortcut rune).
type ListFormatDialogRadio struct {
	Format   ListFormat
	Label    string
	Shortcut rune
}

// ListFormatDialogRadios is the canonical radio list for the listing format dialog and its key handler.
func ListFormatDialogRadios() []ListFormatDialogRadio {
	return []ListFormatDialogRadio{
		{ListFormatMtime, "Modified time", 'm'},
		{ListFormatPerm, "Permissions", 'p'},
		{ListFormatBrief, "Brief", 'b'},
	}
}
